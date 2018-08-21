// Copyright 2017-2018, Square, Inc.

package chain

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
)

// A Traverser provides the ability to run a job chain while respecting the
// dependencies between the jobs.
type Traverser interface {
	// Run traverses a job chain and runs all of the jobs in it. It starts by
	// running the first job in the chain, and then, if the job completed,
	// successfully, running its adjacent jobs. This process continues until there
	// are no more jobs to run, or until the Stop method is called on the traverser.
	//
	// It returns an error if it fails to start.
	Run() error

	// Stop makes a traverser stop traversing its job chain. It also sends a stop
	// signal to all of the jobs that a traverser is running.
	//
	// It returns an error if it fails to stop all running jobs.
	Stop() error

	// Status gets the status of all running and failed jobs. Since a job can only
	// run when all of its ancestors have completed, the state of the entire chain
	// can be inferred from this information - every job in the chain before a
	// running or failed job must be complete, and every job in the chain after a
	// running or failed job must be pending.
	//
	// It returns an error if it fails to get the status of all running jobs.
	Status() (proto.JobChainStatus, error)
}

// A TraverserFactory makes a new Traverser.
type TraverserFactory interface {
	Make(proto.JobChain) (Traverser, error)
}

type traverserFactory struct {
	chainRepo    Repo
	rf           runner.Factory
	rmc          rm.Client
	shutdownChan chan struct{}
}

func NewTraverserFactory(cr Repo, rf runner.Factory, rmc rm.Client, shutdownChan chan struct{}) TraverserFactory {
	return &traverserFactory{
		chainRepo:    cr,
		rf:           rf,
		rmc:          rmc,
		shutdownChan: shutdownChan,
	}
}

// Make makes a Traverser for the given job chain. The chain is first validated
// and saved to the chain repo.
func (f *traverserFactory) Make(jobChain proto.JobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := NewChain(&jobChain)

	// Validate the chain
	err := chain.Validate()
	if err != nil {
		return nil, err
	}

	// Save the chain. If this JR instance dies, another can recover the chain
	// from the repo.
	err = f.chainRepo.Set(chain)
	if err != nil {
		return nil, fmt.Errorf("cannot save chain to repo: %s", err)
	}

	// Create and return a traverser for the chain. The traverser is responsible
	// for the chain: running, cleaning up, removing from repo when done, etc.
	// And traverser and chain have the same lifespan: traverser is done when
	// chain is done.
	tr := NewTraverser(chain, f.chainRepo, f.rf, f.rmc, f.shutdownChan)
	return tr, nil
}

// A traverser represents a job chain and everything needed to traverse it.
type traverser struct {
	// The chain that will be traversed.
	chain *chain

	// Repo for acessing/updating chains.
	chainRepo Repo

	// Factory for creating job runners.
	rf runner.Factory

	// Repo for keeping track of active jobs.
	runnerRepo runner.Repo

	// Used to stop a running traverser.
	stopChan chan struct{}

	// Used to notify traverser to suspend the running job chain.
	shutdownChan chan struct{}

	// Queue for processing jobs that need to run.
	runJobChan chan proto.Job

	// Queue for processing jobs that are done running.
	doneJobChan chan proto.Job

	// Indicator for whether the job chain is done running.
	doneChan chan struct{}

	// Client for communicating with the Request Manager.
	rmc rm.Client

	// job.Id -> number of tries
	jobTries map[string]uint

	// mutex to access jobTries list
	jtMux *sync.RWMutex

	// job.Id -> number of tries a job had before it was stopped
	stoppedJobTries map[string]uint

	// mutex to access stoppedJobTries list
	sjtMux *sync.RWMutex

	// Used for logging.
	logger *log.Entry
}

// NewTraverser creates a new traverser for a job chain.
func NewTraverser(chain *chain, cr Repo, rf runner.Factory, rmc rm.Client, shutdownChan chan struct{}) *traverser {
	return &traverser{
		chain:           chain,
		chainRepo:       cr,
		rf:              rf,
		runnerRepo:      runner.NewRepo(),
		stopChan:        make(chan struct{}),
		shutdownChan:    shutdownChan,
		runJobChan:      make(chan proto.Job),
		doneJobChan:     make(chan proto.Job),
		doneChan:        make(chan struct{}),
		rmc:             rmc,
		jobTries:        make(map[string]uint),
		stoppedJobTries: make(map[string]uint),
		jtMux:           &sync.RWMutex{},
		sjtMux:          &sync.RWMutex{},
		// Include the request id in all logging.
		logger: log.WithFields(log.Fields{"requestId": chain.RequestId()}),
	}
}

// Run runs all jobs in the chain and blocks until all jobs complete or a job fails.
func (t *traverser) Run() error {
	t.logger.Infof("chain traverser start")
	defer t.logger.Infof("chain traverser done")

	var finalState byte
	defer func() {
		// Only tell RM the request was finished if job chain wasn't suspended or
		// if it was suspended but completed successfully anyways.
		if !t.suspended() || finalState == proto.STATE_COMPLETE {
			// Set final state of chain in repo. This will be very short-lived
			// because next we'll finalize the request in the RM. Although
			// short-lived, we set it in case there's problems finalizing with RM.
			t.chain.SetState(finalState)
			t.chainRepo.Set(t.chain)

			// Mark the request as finished in the Request Manager.
			if err := t.rmc.FinishRequest(t.chain.RequestId(), finalState); err != nil {
				t.logger.Errorf("problem reporting status of the finished chain: %s", err)
			} else {
				t.chainRepo.Remove(t.chain.RequestId())
			}
		}

		// Job chain is done running - doneChan is watched by suspend().
		close(t.doneChan)
	}()

	// Watch for shutdown signal and suspend job chain when received.
	go func() {
		<-t.shutdownChan
		t.suspend()
	}()

	firstJob, err := t.chain.FirstJob()
	if err != nil {
		finalState = proto.STATE_FAIL
		return err
	}

	// Set the starting state of the chain.
	t.chain.SetState(proto.STATE_RUNNING)
	t.chainRepo.Set(t.chain)

	// Start a goroutine to run jobs. This consumes from the runJobChan. When
	// jobs are done, they will be sent to the doneJobChan, which gets consumed
	// from right below this.
	go t.runJobs()

	// Add the first job in the chain to the runJobChan.
	t.logger.Infof("sending the first job (%s) to runJobChan", firstJob.Id)
	t.runJobChan <- firstJob

	// When a job finishes, update the state of the chain and figure out what
	// to do next (check to see if the entire chain is done running, and
	// enqueue the next jobs if there are any).
JOB_REAPER:
	for doneJob := range t.doneJobChan {
		jLogger := t.logger.WithFields(log.Fields{"job_id": doneJob.Id, "sequence_id": doneJob.SequenceId, "sequence_retry_count": t.chain.SequenceRetryCount(doneJob)})

		// Set the final state of the job in the chain.
		t.chain.SetJobState(doneJob.Id, doneJob.State)
		t.chainRepo.Set(t.chain)

		// Check to see if the entire chain is done. If it is, break out of
		// the loop on doneJobChan because there is no more work for us to do.
		//
		// A chain is done if no more jobs in it can run. A chain is
		// complete if every job in it completed successfully.
		done, complete := t.chain.IsDone()
		if done {
			close(t.runJobChan)
			if complete {
				t.logger.Infof("chain is done, all jobs finished successfully")
				finalState = proto.STATE_COMPLETE
			} else {
				t.logger.Warn("chain is done, some jobs failed")
				finalState = proto.STATE_FAIL
			}
			break
		}

		// Don't start next jobs if this job failed in some way.
		if doneJob.State != proto.STATE_COMPLETE {
			// Retry sequence as long as job wasn't stopped.
			if doneJob.State != proto.STATE_STOPPED && t.chain.CanRetrySequence(doneJob) {
				jLogger.Info("job did not complete successfully. retrying sequence.")
				t.retrySequence(doneJob) // re-enqueue first job of failed sequence
				continue JOB_REAPER
			}

			// If the failed job was stopped or is not part of a retryable
			// sequence, then ignore subsequent jobs. For example, if job B of
			// A -> B -> C  fails, then C is not run.
			jLogger.Warn("job did not complete successfully")
			continue JOB_REAPER
		}

		// Job completed successfully, so enqueue/run the next jobs in the chain.
		// This will yield multiple next jobs when the current job is the start of
		// a sequence (a fanout node).
		jLogger.Infof("job completed successfully")

	NEXT_JOB:
		for _, nextJ := range t.chain.NextJobs(doneJob.Id) {
			nextJLogger := jLogger.WithFields(log.Fields{"next_job_id": nextJ.Id})

			// Check to make sure the job is ready to run. It might not be if it has
			// upstream dependencies that are still running.
			if !t.chain.JobIsReady(nextJ.Id) {
				nextJLogger.Infof("next job is not ready to run - not enqueuing it")
				continue NEXT_JOB
			}

			nextJLogger.Infof("next job is ready to run - enqueuing it")

			// Copy the jobData from the job that just finished to the next job.
			// It is important to note that when a job has multiple parent
			// jobs, it will get its jobData from whichever parent finishes
			// last. Therefore, a job should never rely on jobData that was
			// created during an unrelated sequence at any time earlier in
			// the chain.
			for k, v := range doneJob.Data {
				nextJ.Data[k] = v
			}

			t.runJobChan <- nextJ // add the job to the run queue
		}

		// Update chain repo for jobs next jobs just started ^
		t.chainRepo.Set(t.chain)
	}

	return nil
}

// Stop stops the traverser if it's running.
func (t *traverser) Stop() error {
	t.logger.Infof("stopping traverser and all jobs")

	// Stop the traverser (i.e., stop running new jobs).
	close(t.stopChan)

	// Get all of the active runners for this traverser from the repo. Only runners
	// that are in the repo will be stopped.
	activeRunners, err := t.runnerRepo.Items()
	if err != nil {
		return err
	}

	// Call Stop on each runner, and then remove it from the repo.
	for jobId, runner := range activeRunners {
		err := runner.Stop() // this should return quickly
		if err != nil {
			return err
		}
		t.runnerRepo.Remove(jobId)
	}

	return nil
}

// Status returns the status of currently running jobs in the chain.
func (t *traverser) Status() (proto.JobChainStatus, error) {
	t.logger.Infof("getting the status of all running jobs")

	activeRunners, err := t.runnerRepo.Items()
	if err != nil {
		return proto.JobChainStatus{}, err
	}

	runningJobs := t.chain.Running()
	status := make([]proto.JobStatus, len(runningJobs))
	i := 0
	for jobId, jobStatus := range runningJobs {
		runner := activeRunners[jobId]
		if runner == nil {
			// The job finished between the call to chain.Running() and now,
			// so it's runner no longer exists in the runner.Repo.
			jobStatus.Status = "(finished)"
		} else {
			jobStatus.Status = runner.Status()
		}
		status[i] = jobStatus
		i++
	}
	jcStatus := proto.JobChainStatus{
		RequestId:   t.chain.RequestId(),
		JobStatuses: status,
	}
	return jcStatus, nil
}

// -------------------------------------------------------------------------- //

// runJobs loops on the runJobChannel, and for each job that comes through the
// channel, it creates a job.Job interface and runs it in a goroutine. If there
// are any errors creating the job.Job it creates a JL that contains the error
// and attaches it to the job. When it's done, it sends the job out through the
// doneJobChan.
func (t *traverser) runJobs() {
	for runnableJob := range t.runJobChan {
		go func(pJob proto.Job) {
			// Always return the job when done, else the traverser will block.
			defer func() { t.doneJobChan <- pJob }()

			// Make a job runner. If an error is encountered, set the
			// state of the job to FAIL and create a JL with the error.
			sequenceRetry := t.chain.SequenceRetryCount(pJob)
			t.jtMux.RLock()
			runner, err := t.rf.Make(pJob, t.chain.RequestId(), t.jobTries[pJob.Id], sequenceRetry)
			t.jtMux.RUnlock()
			if err != nil {
				pJob.State = proto.STATE_FAIL
				t.sendJL(pJob, err) // need to send a JL to the RM so that it knows this job failed
				return
			}

			// Add the runner to the repo. Runners in the repo are used
			// by the Status and Stop methods on the traverser.
			t.runnerRepo.Set(pJob.Id, runner)
			defer t.runnerRepo.Remove(pJob.Id)

			// Bail out if Stop was called or traverser suspended. It is important
			// that this check happens AFTER the runner is added to the repo,
			// because if Stop gets called between the time that a job runner is
			// created and it is added to the repo, there will be nothing to stop
			// that job from running.
			select {
			case <-t.stopChan:
				pJob.State = proto.STATE_STOPPED

				t.jtMux.Lock() // -- lock
				t.jobTries[pJob.Id] = t.jobTries[pJob.Id] + 1
				t.jtMux.Unlock() // -- unlock

				err = fmt.Errorf("not starting job because traverser has already been stopped")
				t.sendJL(pJob, err) // need to send a JL to the RM so that it knows this job failed
				return
			case <-t.shutdownChan:
				// don't send job log / update job tries - will be retried on resume
				pJob.State = proto.STATE_STOPPED
				return
			default:
			}

			// Set the state of the job in the chain to "Running".
			t.chain.SetJobState(pJob.Id, proto.STATE_RUNNING)

			// Run the job. This is a blocking operation that could take a long time.
			ret := runner.Run(pJob.Data)

			t.jtMux.Lock() // -- lock
			t.jobTries[pJob.Id] = t.jobTries[pJob.Id] + ret.Tries
			t.jtMux.Unlock() // -- unlock

			if ret.FinalState == proto.STATE_STOPPED {
				t.sjtMux.Lock() // -- lock
				t.stoppedJobTries[pJob.Id] = ret.Tries
				t.sjtMux.Unlock() // -- unlock
			}

			pJob.State = ret.FinalState
		}(runnableJob)
	}
}

func (t *traverser) sendJL(pJob proto.Job, err error) {
	jLogger := t.logger.WithFields(log.Fields{"job_id": pJob.Id})
	jl := proto.JobLog{
		RequestId:  t.chain.RequestId(),
		JobId:      pJob.Id,
		Name:       pJob.Name,
		Type:       pJob.Type,
		Try:        t.jobTries[pJob.Id],
		SequenceId: pJob.SequenceId,
		StartedAt:  0, // zero because the job never ran
		FinishedAt: 0,
		State:      pJob.State,
		Exit:       1,
		Error:      err.Error(),
	}

	if err != nil {
		jl.Error = err.Error()
	}

	// Send the JL to the RM.
	err = t.rmc.CreateJL(t.chain.RequestId(), jl)
	if err != nil {
		jLogger.Errorf("problem sending job log (%#v) to the RM: %s", jl, err)
	}
}

// retrySequence retries a sequence by doing the following
//   1. Mark the failed job and all previously completed jobs in sequence to PENDING.
//   2. Decrement the number of jobs that were previously ran (failed job +
//   previously completed jobs in sequence) from the chain job count.
//   3. Increment retry count for the sequence
//   4. Enqueue the first job in the sequence to be ran again. It'll
//   subsequently enqueue the remaining jobs in the sequence.
func (t *traverser) retrySequence(failedJob proto.Job) error {
	sequenceStartJob := t.chain.SequenceStartJob(failedJob)

	// sequenceJobsToRetry is a list containing the failed job and all previously
	// completed jobs in the sequence. For example, if job C of A -> B -> C -> D
	// fails, then A and B are the previously completed jobs and C is the failed
	// job. So, jobs A, B, and C will be added to sequenceJobsToRetry. D will not be
	// added because it was never ran.
	var sequenceJobsToRetry []proto.Job
	sequenceJobsToRetry = append(sequenceJobsToRetry, failedJob)
	sequenceJobsCompleted := t.sequenceJobsCompleted(sequenceStartJob)
	sequenceJobsToRetry = append(sequenceJobsCompleted, sequenceJobsCompleted...)

	// Mark all jobs to retry to PENDING state
	for _, job := range sequenceJobsToRetry {
		t.chain.SetJobState(job.Id, proto.STATE_PENDING)
	}

	// Decrement number of jobs we will attempt to retry
	t.chain.n -= uint(len(sequenceJobsToRetry))

	// Increment retry count for this sequence
	t.chain.IncrementSequenceRetryCount(failedJob)

	// Enqueue first job in sequence
	t.runJobChan <- sequenceStartJob

	return nil
}

// sequenceJobsCompleted does a BFS to find all jobs in the sequence that have
// completed. You can read how BFS works here:
// https://en.wikipedia.org/wiki/Breadth-first_search.
func (t *traverser) sequenceJobsCompleted(sequenceStartJob proto.Job) []proto.Job {
	// toVisit is a map of job id->job to visit
	toVisit := map[string]proto.Job{}
	// visited is a map of job id->job visited
	visited := map[string]proto.Job{}

	// Process sequenceStartJob
	for _, pJob := range t.chain.NextJobs(sequenceStartJob.Id) {
		toVisit[pJob.Id] = pJob
	}
	visited[sequenceStartJob.Id] = sequenceStartJob

PROCESS_TO_VISIT_LIST:
	for len(toVisit) > 0 {

	PROCESS_CURRENT_JOB:
		for currentJobId, currentJob := range toVisit {

		PROCESS_NEXT_JOBS:
			for _, nextJob := range t.chain.NextJobs(currentJobId) {
				// Don't add failed or pending jobs to toVisit list
				// For example, if job C of A -> B -> C -> D fails, then do not add C
				// or D to toVisit list. Because we have single sequence retries,
				// stopping at the failed job ensures we do not add jobs not in the
				// sequence to the toVisit list.
				if nextJob.State != proto.STATE_COMPLETE {
					continue PROCESS_NEXT_JOBS
				}

				// Make sure we don't visit a job multiple times. We can see a job
				// mulitple times if it is a "fan in" node.
				if _, seen := visited[nextJob.Id]; !seen {
					toVisit[nextJob.Id] = nextJob
				}
			}

			// Since we have processed all of the next jobs for this current job, we
			// are done visiting the current job and can delete it from the toVisit
			// list and add it to the visited list.
			delete(toVisit, currentJobId)
			visited[currentJobId] = currentJob

			continue PROCESS_CURRENT_JOB
		}

		continue PROCESS_TO_VISIT_LIST
	}

	completedJobs := make([]proto.Job, 0, len(visited))
	for _, j := range visited {
		completedJobs = append(completedJobs, j)
	}

	return completedJobs
}

// Stops the traverser and sends a Suspended Job Chain (SJC) to the Request Manager.
// The SJC contains all the necessary info to recreate the traverser and resume
// running the job chain.
// suspend is called when the Job Runner is shutting down.
func (t *traverser) suspend() {
	t.logger.Info("suspending job chain")
	defer t.logger.Info("job chain suspended")

	// Stop all active runners.
	activeRunners, err := t.runnerRepo.Items()
	if err != nil {
		// Log error but continue trying to suspend the job chain - JR is being
		// shut down. (even on error, activeRunners won't be set to nil)
		t.logger.Errorf("problem retrieving job runners from repo: %s", err)
	}
	for jobId, runner := range activeRunners {
		err := runner.Stop() // this should return quickly
		if err != nil {
			t.logger.Errorf("problem stopping job runner (job id = %s): %s", jobId, err) // log error but continue
		}
	}

	// Wait for chain to finish running - timeout after 10 seconds.
	waitChan := time.After(10 * time.Second)
	select {
	case <-waitChan:
	case <-t.doneChan:
	}

	if t.chain.State() != proto.STATE_COMPLETE {
		// Only send SJC if chain didn't complete successfully.
		// If chain completed successfully, Run() will have told the RM.

		// Save chain state (in case there's an issue sending the SJC to the RM).
		t.chain.SetState(proto.STATE_SUSPENDED)
		t.chainRepo.Set(t.chain)

		// Create a SuspendedJobChain and send it to the RM.
		jobChain := t.chain.JobChain()
		requestId := jobChain.RequestId
		sequenceRetries := t.chain.SequenceRetryCounts()
		sjc := proto.SuspendedJobChain{
			RequestId:       requestId,
			JobChain:        jobChain,
			JobTries:        t.jobTries,
			StoppedJobTries: t.stoppedJobTries,
			SequenceRetries: sequenceRetries,
		}
		if err := t.rmc.SuspendRequest(requestId, sjc); err != nil {
			t.logger.Errorf("problem sending suspended job chain to RM: %s", err)
		} else {
			t.chainRepo.Remove(t.chain.RequestId())
		}
	}
}

func (t *traverser) suspended() bool {
	select {
	case <-t.shutdownChan:
		return true
	default:
		return false
	}
}
