// Copyright 2017, Square, Inc.

package chain

import (
	"fmt"

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
	// or no more jobs to run, or until the Stop method is called on the traverser.
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

// A TraverserFactory makes new Traverser.
type TraverserFactory interface {
	Make(proto.JobChain) (Traverser, error)
}

type traverserFactory struct {
	chainRepo Repo
	rf        runner.Factory
	rmc       rm.Client
}

func NewTraverserFactory(cr Repo, rf runner.Factory, rmc rm.Client) TraverserFactory {
	return &traverserFactory{
		chainRepo: cr,
		rf:        rf,
		rmc:       rmc,
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
	tr := NewTraverser(chain, f.chainRepo, f.rf, f.rmc)
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

	// Queue for processing jobs that need to run.
	runJobChan chan proto.Job

	// Queue for processing jobs that are done running.
	doneJobChan chan proto.Job

	// Client for communicating with the Request Manager.
	rmc rm.Client

	// Used for logging.
	logger *log.Entry
}

// NewTraverser creates a new traverser for a job chain.
func NewTraverser(chain *chain, cr Repo, rf runner.Factory, rmc rm.Client) *traverser {
	return &traverser{
		chain:       chain,
		chainRepo:   cr,
		rf:          rf,
		runnerRepo:  runner.NewRepo(),
		stopChan:    make(chan struct{}),
		runJobChan:  make(chan proto.Job),
		doneJobChan: make(chan proto.Job),
		rmc:         rmc,
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
		// Set final state of chain in repo. This will be very short-lived because
		// next we'll finalize the request in the RM. Although short-lived, we set
		// it case there's problems finalizing with RM.
		t.chain.SetState(finalState)
		t.chainRepo.Set(t.chain)

		// Mark the request as finished in the Request Manager.
		if err := t.rmc.FinishRequest(t.chain.RequestId(), finalState); err != nil {
			t.logger.Errorf("problem reporting status of the finished chain: %s", err)
		} else {
			t.chainRepo.Remove(t.chain.RequestId())
		}
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

	// Set the state of the first job in the chain to RUNNING.
	t.chain.SetJobState(firstJob.Id, proto.STATE_RUNNING)
	t.chainRepo.Set(t.chain)

	// Add the first job in the chain to the runJobChan.
	t.logger.Infof("sending the first job (%s) to runJobChan", firstJob.Id)
	t.runJobChan <- firstJob

	// When a job finishes, update the state of the chain and figure out what
	// to do next (check to see if the entire chain is done running, and
	// enqueue the next jobs if there are any).
JOB_REAPER:
	for doneJ := range t.doneJobChan {
		jLogger := t.logger.WithFields(log.Fields{"job_id": doneJ.Id})

		// Set the final state of the job in the chain.
		t.chain.SetJobState(doneJ.Id, doneJ.State)
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

		// If the job did not complete successfully, then ignore subsequent jobs.
		// For example, if job B of A -> B -> C  fails, then C is not ran.
		if doneJ.State != proto.STATE_COMPLETE {
			jLogger.Warn("job did not complete successfully")
			continue JOB_REAPER
		}

		// Job completed successfully, so enqueue/run the next jobs in the chain.
		// This will yield multiple next jobs when the current job is the start of
		// a sequence (a fanout node).
		jLogger.Infof("job completed successfully")

	NEXT_JOB:
		for _, nextJ := range t.chain.NextJobs(doneJ.Id) {
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
			for k, v := range doneJ.Data {
				nextJ.Data[k] = v
			}

			// Set the state of the job in the chain to "Running".
			// @todo: this should be in the goroutine in runJobs, because it's not
			//        truly running until that point, but then this causes race conditions
			//        which indicates we need to more closely examine concurrent
			//        access to internal chain data.
			t.chain.SetJobState(nextJ.Id, proto.STATE_RUNNING)

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
	var jcStatus proto.JobChainStatus
	var jobStatuses []proto.JobStatus

	// Get all of the active runners for this traverser from the repo. Only runners
	// that are in the repo will have their statuses queried.
	activeRunners, err := t.runnerRepo.Items()
	if err != nil {
		return jcStatus, err
	}

	// Get the status and state of each job runner.
	for jobId, runner := range activeRunners {
		jobStatus := proto.JobStatus{
			JobId:  jobId,
			State:  t.chain.JobState(jobId), // get the state of the job
			Status: runner.Status(),         // get the job status. this should return quickly
		}
		jobStatuses = append(jobStatuses, jobStatus)
	}

	jcStatus = proto.JobChainStatus{
		RequestId:   t.chain.RequestId(),
		JobStatuses: jobStatuses,
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
			runner, err := t.rf.Make(pJob, t.chain.RequestId())
			if err != nil {
				pJob.State = proto.STATE_FAIL
				t.sendJL(pJob, err) // need to send a JL to the RM so that it knows this job failed
				return
			}

			// Add the runner to the repo. Runners in the repo are used
			// by the Status and Stop methods on the traverser.
			t.runnerRepo.Set(pJob.Id, runner)
			defer t.runnerRepo.Remove(pJob.Id)

			// Bail out if Stop was called. It is important that this check happens AFTER
			// the runner is added to the repo, because if Stop gets called between the
			// time that a job runner is created and it is added to the repo, there will
			// be nothing to stop that job from running.
			select {
			case <-t.stopChan:
				pJob.State = proto.STATE_STOPPED
				err = fmt.Errorf("not starting job because traverser has already been stopped")
				t.sendJL(pJob, err) // need to send a JL to the RM so that it knows this job failed
				return
			default:
			}

			// Run the job. This is a blocking operation that could take a long time.
			finalState := runner.Run(pJob.Data)

			// The traverser only cares about if a job completes or fails. Therefore,
			// we set the state of every job that isn't COMPLETE to be FAIL.
			if finalState != proto.STATE_COMPLETE {
				finalState = proto.STATE_FAIL
			}
			pJob.State = finalState
		}(runnableJob)
	}
}

func (t *traverser) sendJL(pJob proto.Job, err error) {
	jLogger := t.logger.WithFields(log.Fields{"job_id": pJob.Id})
	jl := proto.JobLog{
		RequestId:  t.chain.RequestId(),
		JobId:      pJob.Id,
		Type:       pJob.Type,
		StartedAt:  0, // zero because the job never ran
		FinishedAt: 0,
		State:      pJob.State,
		Exit:       1,
		Error:      err.Error(),
	}

	// Send the JL to the RM.
	err = t.rmc.CreateJL(t.chain.RequestId(), jl)
	if err != nil {
		jLogger.Errorf("problem sending job log (%#v) to the RM: %s", jl, err)
	}
}
