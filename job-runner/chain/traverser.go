// Copyright 2018, Square, Inc.

package chain

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/retry"
)

const (
	// Time to wait for all active runners to return after stopping them.
	STOP_TIMEOUT = 10 * time.Second

	// Number of times to attempt sending a job log to the RM.
	JOB_LOG_TRIES = 3
	// Time to wait between attempts to send a job log to RM.
	JOB_LOG_RETRY_WAIT = 500 * time.Millisecond
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
	Run()

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
	MakeFromSJC(proto.SuspendedJobChain) (Traverser, error)
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
	return f.make(chain)
}

// MakeFromSJC makes a Traverser from a suspended job chain.
func (f *traverserFactory) MakeFromSJC(sjc proto.SuspendedJobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := ResumeChain(&sjc)
	return f.make(chain)
}

// Creates a new Traverser from a chain. Used for both new and resumed chains.
func (f *traverserFactory) make(chain *chain) (Traverser, error) {
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

// -------------------------------------------------------------------------- //

type traverser struct {
	reaperFactory ReaperFactory
	reaper        JobReaper

	shutdownChan chan struct{}  // indicates JR is shutting down
	runJobChan   chan proto.Job // jobs to be run
	doneJobChan  chan proto.Job // jobs that are done
	doneChan     chan struct{}  // closed when traverser finishes running
	stopChan     chan struct{}  // indicates JR has been stopped

	stopMux *sync.RWMutex

	chain      *chain
	chainRepo  Repo // stores all currently running chains
	rf         runner.Factory
	runnerRepo runner.Repo // stores actively running jobs
	rmc        rm.Client
	logger     *log.Entry

	StopTimeout time.Duration // Time to wait for runners to return after being stopped.
	jlTries     int           // Number of times to try sending a JL to the RM.
	jlRetryWait time.Duration // Time to wait between tries to send JL to RM.
}

func NewTraverser(chain *chain, cr Repo, rf runner.Factory, rmc rm.Client, shutdownChan chan struct{}) *traverser {
	return &traverser{
		reaperFactory: NewReaperFactory(chain, cr, rmc),
		chain:         chain,
		chainRepo:     cr,
		rf:            rf,
		runnerRepo:    runner.NewRepo(),
		stopChan:      make(chan struct{}),
		shutdownChan:  shutdownChan,
		runJobChan:    make(chan proto.Job),
		doneJobChan:   make(chan proto.Job),
		doneChan:      make(chan struct{}),
		stopMux:       &sync.RWMutex{},
		rmc:           rmc,
		// Include the request id in all logging.
		logger:      log.WithFields(log.Fields{"requestId": chain.RequestId()}),
		StopTimeout: STOP_TIMEOUT,
		jlTries:     JOB_LOG_TRIES,
		jlRetryWait: JOB_LOG_RETRY_WAIT,
	}
}

// Run runs all jobs in the chain and blocks until all jobs complete, a job fails,
// or the chain is stopped or suspended.
func (t *traverser) Run() {
	t.logger.Infof("chain traverser started")
	defer t.logger.Infof("chain traverser done")

	t.reaper = t.reaperFactory.MakeRunning(t.runJobChan)

	// Send final state or SJC to Request Manager.
	defer func() {
		t.reaper.Finalize() // stopMux lock unnecessary - traverser is done
	}()

	// Find all the jobs we can start running. For a new job chain (not suspended),
	// this'll end up being just the first job in the chain.
	jobsToRun := t.chain.RunnableJobs()
	if len(jobsToRun) == 0 {
		// No jobs ready to run means the chain must already be done.
		return
	}

	// Start a goroutine to run jobs. This consumes from the runJobChan. When
	// jobs are done, they will be sent to the doneJobChan, which gets consumed
	// from right below this.
	go t.runJobs()

	// Suspend running job chain if Job Runner shuts down.
	go func() {
		select {
		case <-t.shutdownChan:
			t.shutdown()
		case <-t.doneChan: // check if chain is done (or this func might never return)
			return
		}
	}()

	// Add the first jobs to the runJobChan.
	// Prevent Stop()/shutdown() from closing runJobChan and don't enqueue jobs
	// if the traverser has already been stopped or shutdown.
	t.stopMux.Lock()
	select {
	case <-t.stopChan:
		return
	case <-t.shutdownChan:
		return
	default:
		for _, job := range jobsToRun {
			t.logger.Infof("sending initial job (%s) to runJobChan", job.Id)
			if t.chain.IsSequenceStartJob(job.Id) {
				// Starting a sequence, so increment sequence try count.
				t.chain.IncrementSequenceTries(job.Id)
				seqLogger := t.logger.WithFields(log.Fields{"sequence_id": job.SequenceId, "sequence_try": t.chain.SequenceTries(job.Id)})
				seqLogger.Info("starting try of sequence")
			}
			t.runJobChan <- job
		}
	}
	t.stopMux.Unlock()

	// Reap jobs as they finish running. Stop reaping jobs when the doneJobChan
	// is closed by runJobs - when the chain is done, stopped, or suspended.
	for doneJob := range t.doneJobChan {
		// Don't allow closing runJobChan / switching the reaper while we are
		// reaping a job. Reap() should return quickly, so this is ok.
		t.stopMux.Lock()
		t.reaper.Reap(doneJob)
		t.stopMux.Unlock()
	}

	// Indicate that the chain is done running. Stops waitForShutdown() listener.
	select {
	case <-t.doneChan:
	default:
		close(t.doneChan)
	}
}

// Stop stops a running chain by stopping all its running jobs.
func (t *traverser) Stop() error {
	// LOCK - can't close these chans while a job is being Reaped / initial
	// jobs are being sent to runJobChan.
	t.stopMux.Lock()

	// Don't do anything if traverser has already been stopped.
	select {
	case <-t.stopChan:
		return nil
	default:
	}

	t.logger.Infof("stopping traverser and all jobs")

	t.reaper = t.reaperFactory.MakeStopped()
	close(t.runJobChan) // will stop runJobs loop (stop running job more jobs)
	close(t.stopChan)   // indicate traverser has been stopped
	t.stopMux.Unlock()  // UNLOCK

	err := t.stopRunningJobs()
	if err != nil {
		return fmt.Errorf("problem stopping traverser: %s", err)
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

// runJobs loops on the runJobChan, and runs each job that comes through the
// channel. When the job is done, it sends the job out through the doneJobChan.
// Once runJobChan is closed, it waits for any running jobs to finish and
// then closes doneJobChan.
func (t *traverser) runJobs() {
	var runningWG sync.WaitGroup // used to wait for all jobs to finish running
	stopTimeoutChan := make(chan struct{})
	timeoutMux := &sync.Mutex{}

	// Run all jobs that come in on runJobChan. This loop exits when
	// runJobChan is closed (when the chain is stopped, suspended, or done).
	for job := range t.runJobChan {
		runningWG.Add(1) // A new job is starting.

		// Explicitly pass the job in, or all goroutines
		// would share the same loop "job" variable.
		go func(job proto.Job) {
			defer runningWG.Done() // Job is done running.

			t.runJob(&job)

			// Send finished job to doneJobChan.
			timeoutMux.Lock()
			defer timeoutMux.Unlock()
			select {
			case <-stopTimeoutChan:
				// If this job took too long to finish after being stopped,
				// doneJobChan has already been closed - don't send job.
				return
			default:
				t.doneJobChan <- job
			}
		}(job)
	}

	// Wait for all running jobs to finish. Timeout if running jobs
	// haven't finished after 10 seconds.
	waitChan := make(chan struct{})
	go func() {
		runningWG.Wait()
		close(waitChan)
	}()

	go func() {
		time.Sleep(t.StopTimeout)
		close(stopTimeoutChan)
	}()

	select {
	case <-stopTimeoutChan:
		timeoutMux.Lock()
		defer timeoutMux.Unlock()
	case <-waitChan:
	}

	// Indicate there are no more jobs to reap. This kicks off finalizing the chain.
	close(t.doneJobChan)
}

// runJob creates a job Runner for the given job and runs it. If there
// are any errors creating the runner, it sends a JL that contains the error
// to the RM. It modifies the original job with its final State before returning.
func (t *traverser) runJob(job *proto.Job) {
	jLogger := t.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": t.chain.SequenceTries(job.Id)})

	// Make a job runner. If an error is encountered, set the
	// state of the job to FAIL and create a JL with the error.
	sequenceTries := t.chain.SequenceTries(job.Id) // used for job logs
	previousTries := t.chain.TotalTries(job.Id)    // used for job logs
	triesToSkip := uint(0)
	if job.State == proto.STATE_STOPPED {
		// When resuming a stopped job, take into account the number of times
		// it has already been tried. Its last try (the try it was stopped on)
		// doesn't count.
		triesToSkip = t.chain.LatestRunTries(job.Id) - 1
	}

	runner, err := t.rf.Make(*job, t.chain.RequestId(), previousTries, triesToSkip, sequenceTries)

	if err != nil {
		job.State = proto.STATE_FAIL
		t.sendJL(*job, err) // need to send a JL to the RM so that it knows this job failed
		return
	}

	// Add the runner to the repo. Runners in the repo are used
	// by the Status and Stop methods on the traverser.
	t.runnerRepo.Set(job.Id, runner)
	defer t.runnerRepo.Remove(job.Id)

	// Bail out if Stop was called or traverser suspended. It is important
	// that this check happens AFTER the runner is added to the repo,
	// because otherwise if Stop gets called between the time that a job
	// runner is created and it is added to the repo, there will be nothing
	// to stop that job from running.
	select {
	case <-t.stopChan:
		job.State = proto.STATE_STOPPED

		t.chain.AddJobTries(job.Id, 1)

		err = fmt.Errorf("not starting job because traverser has already been stopped")
		t.sendJL(*job, err) // need to send a JL to the RM so that it knows this job failed
		return
	case <-t.shutdownChan:
		job.State = proto.STATE_STOPPED
		// don't include this try in the total # of tries since we're not sending a JL
		t.chain.AddLatestJobTries(job.Id, 1)
		return
	default:
	}

	// Set the state of the job in the chain to "Running".
	t.chain.SetJobState(job.Id, proto.STATE_RUNNING)

	// Run the job. This is a blocking operation that could take a long time.
	jLogger.Infof("running job")
	ret := runner.Run(job.Data)

	t.chain.AddJobTries(job.Id, ret.Tries)

	job.State = ret.FinalState
}

// sendJL sends a job log to the Request Manager.
func (t *traverser) sendJL(job proto.Job, err error) {
	jLogger := t.logger.WithFields(log.Fields{"job_id": job.Id})
	jl := proto.JobLog{
		RequestId:   t.chain.RequestId(),
		JobId:       job.Id,
		Name:        job.Name,
		Type:        job.Type,
		Try:         t.chain.TotalTries(job.Id),
		SequenceId:  job.SequenceId,
		SequenceTry: t.chain.SequenceTries(job.Id),
		StartedAt:   0, // zero because the job never ran
		FinishedAt:  0,
		State:       job.State,
		Exit:        1,
		Error:       err.Error(),
	}

	if err != nil {
		jl.Error = err.Error()
	}

	// Send the JL to the RM.
	err = retry.Do(t.jlTries, t.jlRetryWait,
		func() error {
			return t.rmc.CreateJL(t.chain.RequestId(), jl)
		},
		func(err error) {
			jLogger.Errorf("problem sending job log (%#v) to the RM: %s. Retrying...", jl, err)
		},
	)
	if err != nil {
		jLogger.Errorf("failed to send job log (%#v) to Request Manager", jl)
		return
	}
	jLogger.Infof("successfully sent job log (%#v) to Request Manager", jl)
}

// shutdown suspends the running chain by stopping all its jobs and sending
// a suspended job chain to the Request Manager.
func (t *traverser) shutdown() {
	t.stopMux.Lock()

	// Don't do anything if traverser has already been stopped.
	select {
	case <-t.stopChan:
		return
	default:
	}

	t.logger.Info("suspending job chain - stopping all jobs")

	t.reaper = t.reaperFactory.MakeSuspended() // switch to suspended reaper
	close(t.runJobChan)                        // won't run any more jobs
	t.stopMux.Unlock()

	err := t.stopRunningJobs()
	if err != nil {
		t.logger.Errorf("problem suspending chain: %s", err)
	}
}

// stopRunningJobs stops all currently running jobs.
func (t *traverser) stopRunningJobs() error {
	// Get all of the active runners for this traverser from the repo. Only runners
	// that are in the repo will be stopped.
	activeRunners, err := t.runnerRepo.Items()
	if err != nil {
		return fmt.Errorf("problem retrieving job runners from repo: %s", err)
	}

	// Call Stop on each runner. Use goroutines in case some jobs don't return from
	// Stop() quickly.
	var runnerWG sync.WaitGroup
	hadError := false
	for jobId, activeRunner := range activeRunners {
		runnerWG.Add(1)
		go func(runner runner.Runner) {
			defer runnerWG.Done()
			err := runner.Stop()

			if err != nil {
				t.logger.Errorf("problem stopping job runner (job id = %s): %s", jobId, err)
				hadError = true
			}
		}(activeRunner)
	}

	// Check if there was an error when stopping at least one of the jobs.
	runnerWG.Wait()
	if hadError {
		return fmt.Errorf("problem stopping one or more job runners - see logs for more info")
	}
	return nil
}
