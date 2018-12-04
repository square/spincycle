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
	"github.com/square/spincycle/retry"
)

var (
	// Returned when Stop is called but the chain has already been suspended.
	ErrShuttingDown = fmt.Errorf("chain not stopped because traverser is shutting down")
)

const (
	// Default timeout used by traverser factory for traverser's stopTimeout
	// and sendTimeout.
	defaultTimeout = 10 * time.Second

	// Number of times to attempt sending a job log to the RM.
	jobLogTries = 3
	// Time to wait between attempts to send a job log to RM.
	jobLogRetryWait = 500 * time.Millisecond

	// Number of times to attempt sending chain state / SJC to RM in Reaper.
	reaperTries = 5
	// Time to wait between tries to send chain state/SJC to RM.
	reaperRetryWait = 500 * time.Millisecond
)

// A Traverser provides the ability to run a job chain while respecting the
// dependencies between the jobs.
type Traverser interface {
	// Run traverses a job chain and runs all of the jobs in it. It starts by
	// running the first job in the chain, and then, if the job completed,
	// successfully, running its adjacent jobs. This process continues until there
	// are no more jobs to run, or until the Stop method is called on the traverser.
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

// Make makes a Traverser for the job chain. The chain is first validated
// and saved to the chain repo.
func (f *traverserFactory) Make(jobChain proto.JobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := NewChain(&jobChain, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	return f.make(chain)
}

// MakeFromSJC makes a Traverser from a suspended job chain.
func (f *traverserFactory) MakeFromSJC(sjc proto.SuspendedJobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := NewChain(sjc.JobChain, sjc.SequenceTries, sjc.TotalJobTries, sjc.LatestRunJobTries)
	return f.make(chain)
}

// Creates a new Traverser from a chain. Used for both new and resumed chains.
func (f *traverserFactory) make(chain *Chain) (Traverser, error) {
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
	cfg := TraverserConfig{
		Chain:         chain,
		ChainRepo:     f.chainRepo,
		RunnerFactory: f.rf,
		RMClient:      f.rmc,
		ShutdownChan:  f.shutdownChan,
		StopTimeout:   defaultTimeout,
		SendTimeout:   defaultTimeout,
	}
	tr := NewTraverser(cfg)
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

	stopMux   *sync.RWMutex // lock around checks to stopped
	stopped   bool          // has traverser been stopped
	suspended bool          // has traverser been suspended

	chain      *Chain
	chainRepo  Repo // stores all currently running chains
	rf         runner.Factory
	runnerRepo runner.Repo // stores actively running jobs
	rmc        rm.Client
	logger     *log.Entry

	stopTimeout time.Duration // Time to wait for jobs to stop
	sendTimeout time.Duration // Time to wait for a job to send on doneJobChan.
}

type TraverserConfig struct {
	Chain         *Chain
	ChainRepo     Repo
	RunnerFactory runner.Factory
	RMClient      rm.Client
	ShutdownChan  chan struct{}
	StopTimeout   time.Duration
	SendTimeout   time.Duration
}

func NewTraverser(cfg TraverserConfig) *traverser {
	// Include request id in all logging.
	logger := log.WithFields(log.Fields{"requestId": cfg.Chain.RequestId()})

	// Channels used to communicate between traverser + reaper(s)
	doneJobChan := make(chan proto.Job)
	runJobChan := make(chan proto.Job)

	runnerRepo := runner.NewRepo() // needed for traverser + reaper factory
	reaperFactory := &ChainReaperFactory{
		Chain:        cfg.Chain,
		ChainRepo:    cfg.ChainRepo,
		RMClient:     cfg.RMClient,
		RMCTries:     reaperTries,
		RMCRetryWait: reaperRetryWait,
		Logger:       logger,
		DoneJobChan:  doneJobChan,
		RunJobChan:   runJobChan,
		RunnerRepo:   runnerRepo,
	}

	return &traverser{
		reaperFactory: reaperFactory,
		logger:        logger,
		chain:         cfg.Chain,
		chainRepo:     cfg.ChainRepo,
		rf:            cfg.RunnerFactory,
		runnerRepo:    runnerRepo,
		shutdownChan:  cfg.ShutdownChan,
		runJobChan:    runJobChan,
		doneJobChan:   doneJobChan,
		doneChan:      make(chan struct{}),
		rmc:           cfg.RMClient,
		stopMux:       &sync.RWMutex{},
		stopTimeout:   cfg.StopTimeout,
		sendTimeout:   cfg.SendTimeout,
	}
}

// Run runs all jobs in the chain and blocks until the chain finishes running, is
// stopped, or is suspended.
func (t *traverser) Run() {
	t.logger.Infof("chain traverser started")
	defer t.logger.Infof("chain traverser done")

	// Start a goroutine to run jobs. This consumes from the runJobChan. When
	// jobs are done, they will be sent to the doneJobChan, which the job reapers
	// consume from.
	go t.runJobs()

	// Find all the jobs we can start running. For a new job chain (not suspended),
	// this'll end up being just the first job in the chain.
	jobsToRun := t.chain.RunnableJobs()

	// Add the first jobs to runJobChan.
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

	// Start a goroutine to reap done jobs. The running reaper consumes from
	// doneJobChan and sends the next jobs to be run to runJobChan.
	runningReaperChan := make(chan struct{})
	t.reaper = t.reaperFactory.MakeRunning()
	go func() {
		defer close(runningReaperChan) // indicate reaper is done (see select below)
		defer close(t.runJobChan)      // stop runJobs goroutine
		t.reaper.Run()
	}()

	// Wait for running reaper to be done or traverser to be shut down.
	select {
	case <-runningReaperChan:
		// If running reaper is done because traverser was stopped, we will
		// wait for Stop() to finish. Otherwise, the chain finished normally
		// (completed or failed) and we can return right away.
		//
		// We don't check if the chain was suspended, since that can only
		// happen via the other case in this select.
		t.stopMux.Lock()
		if !t.stopped {
			t.stopMux.Unlock()
			return
		}
		t.stopMux.Unlock()
	case <-t.shutdownChan:
		// The Job Runner is shutting down. Stop the running reaper and suspend
		// the job chain, to be resumed later by another Job Runner.
		t.shutdown()
	}

	// Traverser is being stopped or shut down - wait for that to finish before
	// returning.
	select {
	case <-t.doneChan:
		// Stopped/shutdown successfully - nothing left to do.
		return
	case <-time.After(20 * time.Second):
		// Failed to stop/shutdown in a reasonable amount of time.
		// Log the failure and return.
		t.logger.Warnf("stopping or suspending the job chain took too long. Exiting...")
		return
	}
}

// Stop stops the running job chain by switching the running chain reaper for a
// stopped chain reaper and stopping all currently running jobs. Stop blocks until
// all jobs have finished and the stopped reaper has send the chain's final state
// to the RM.
func (t *traverser) Stop() error {
	// Don't do anything if the traverser has already been stopped or suspended.
	t.stopMux.Lock()
	defer t.stopMux.Unlock()
	if t.stopped {
		return nil
	} else if t.suspended {
		return ErrShuttingDown
	}
	t.stopped = true

	t.logger.Infof("stopping traverser and all jobs")

	// Stop the current reaper and start running a reaper for stopped chains. This
	// reaper saves jobs' states (but doesn't enqueue any more jobs to run) and
	// sends the chain's final state to the RM when all jobs have stopped running.
	t.reaper.Stop() // blocks until running reaper is done stopping
	stoppedReaperChan := make(chan struct{})
	t.reaper = t.reaperFactory.MakeStopped()
	go func() {
		defer close(stoppedReaperChan)
		t.reaper.Run()
	}()

	// Stop all job runners in the runner repo. Do this after switching to the
	// stopped reaper so that when the jobs finish and are sent on doneJobChan,
	// they are reaped correctly.
	err := t.stopRunningJobs()
	if err != nil {
		// Don't return the error yet - we still want to wait for the stop
		// reaper to be done.
		err = fmt.Errorf("traverser was stopped, but encountered an error in the process: %s", err)
	}

	// Wait for the stopped reaper to finish. If it takes too long, some jobs
	// haven't respond quickly to being stopped. Stop waiting for these jobs by
	// stopping the stopped reaper.
	select {
	case <-stoppedReaperChan:
	case <-time.After(t.stopTimeout):
		t.logger.Warnf("timed out waiting for jobs to stop - stopping reaper")
		t.reaper.Stop()
	}
	close(t.doneChan)
	return err
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
func (t *traverser) runJobs() {
	// Run all jobs that come in on runJobChan. The loop exits when runJobChan
	// is closed after the running reaper finishes.
	for job := range t.runJobChan {
		// Explicitly pass the job into the func, or all goroutines would share
		// the same loop "job" variable.
		go func(job proto.Job) {
			jLogger := t.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": t.chain.SequenceTries(job.Id)})

			// Always send the finished job to doneJobChan to be reaped. If the
			// reaper isn't reaping any more jobs (if this job took too long to
			// finish after being stopped), sending to doneJobChan won't be
			// possible - timeout after a while so we don't leak this goroutine.
			defer func() {
				select {
				case t.doneJobChan <- job:
				case <-time.After(t.sendTimeout):
					jLogger.Warnf("timed out sending job to doneJobChan")
				}
				// Remove the job's runner from the repo (if it was ever added)
				// AFTER sending it to doneJobChan. This avoids a race condition
				// when the stopped + suspended reapers check if the runnerRepo
				// is empty.
				t.runnerRepo.Remove(job.Id)
			}()

			// Retrieve job and sequence try info from the chain for the Runner.
			sequenceTries := t.chain.SequenceTries(job.Id) // used in job logs
			totalJobTries := t.chain.TotalTries(job.Id)    // used in job logs
			// When resuming a stopped job, only try the job
			// [allowed tries - tries before being stopped] times, so the total
			// number of times the job is tried (during this sequence try) stays
			// correct. The job's last try (the try it was stopped on) doesn't
			// count, so subtract 1 if it was tried at least once before
			// being stopped.
			triesBeforeStopped := uint(0)
			if job.State == proto.STATE_STOPPED {
				triesBeforeStopped = t.chain.LatestRunTries(job.Id)
				if triesBeforeStopped > 0 {
					triesBeforeStopped--
				}
			}

			runner, err := t.rf.Make(job, t.chain.RequestId(), totalJobTries, triesBeforeStopped, sequenceTries)
			if err != nil {
				// Problem creating the job runner - treat job as failed.
				// Send a JobLog to the RM so that it knows this job failed.
				job.State = proto.STATE_FAIL
				err = fmt.Errorf("problem creating job runner: %s", err)
				t.sendJL(job, err)
				return
			}

			// Add the runner to the repo. Runners in the repo are used
			// by the Status, Stop, and shutdown methods on the traverser.
			t.runnerRepo.Set(job.Id, runner)

			// Bail out if Stop was called or traverser shut down. It is
			// important that this check happens AFTER the runner is added to
			// the repo. Otherwise if Stop gets called after this check but
			// before the runner is added to the repo, there will be nothing to
			// stop the job from running.
			//
			// We don't lock stopMux around this check and runner.Run. It's okay if
			// there's a small chance for the runner to be run after the traverser
			// gets stopped or shut down - it'll just return after trying the job
			// once.
			if t.stopped {
				job.State = proto.STATE_STOPPED

				// Send a JL to the RM so that it knows this job was stopped.
				// Add 1 to the total job tries, since this is used for keeping
				// job logs unique.
				t.chain.AddJobTries(job.Id, 1)
				err = fmt.Errorf("not starting job because traverser has already been stopped")
				t.sendJL(job, err)
				return
			} else if t.suspended {
				job.State = proto.STATE_STOPPED
				// Don't send a JL because this job will be resumed later,
				// and don't include this try in the total # of tries (only
				// set job tries for the latest run).
				t.chain.SetLatestRunJobTries(job.Id, 1)
				return
			}

			// Run the job. This is a blocking operation that could take a long time.
			t.chain.SetJobState(job.Id, proto.STATE_RUNNING)
			jLogger.Infof("running job")
			ret := runner.Run(job.Data)

			t.chain.AddJobTries(job.Id, ret.Tries)
			job.State = ret.FinalState
		}(job)
	}
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
	err = retry.Do(jobLogTries, jobLogRetryWait,
		func() error {
			return t.rmc.CreateJL(t.chain.RequestId(), jl)
		},
		nil,
	)
	if err != nil {
		jLogger.Errorf("problem sending job log (%#v) to the Request Manager: %s", jl, err)
		return
	}
}

// shutdown suspends the running chain by switching the running chain reaper for a
// suspended chain reaper and stopping all currently running jobs. Once all jobs
// have finished, the suspended reaper informs the RM about the suspended chain by
// sending a SuspendedJobChain.
//
// When a Job Runner is shutting down, all of its traversers are shut down and their
// running job chains suspended. The Request Manager can later resume these job
// chains by sending them to a running Job Runner instance.
func (t *traverser) shutdown() {
	// Don't do anything if the traverser has already been stopped or suspended.
	t.stopMux.Lock()
	defer t.stopMux.Unlock()
	if t.stopped || t.suspended {
		return
	}
	t.suspended = true

	t.logger.Info("suspending job chain - stopping all jobs")

	// Stop the current reaper and start running a reaper for suspended chains. This
	// reaper saves jobs' states and prepares the chain to be resumed later, but
	// doesn't enqueue any more jobs to run. When all jobs have stopped running,
	// it sends the SuspendedJobChain to the RM (or the final state if the
	// chain was completed or failed).
	t.reaper.Stop() // blocks until running reaper is done stopping
	suspendedReaperChan := make(chan struct{})
	t.reaper = t.reaperFactory.MakeSuspended()
	go func() {
		defer close(suspendedReaperChan)
		t.reaper.Run()
	}()

	// Stop all job runners in the runner repo. Do this after switching to the
	// suspended reaper so that when the jobs finish and are sent on doneJobChan,
	// they are reaped correctly.
	err := t.stopRunningJobs()
	if err != nil {
		t.logger.Errorf("problem suspending job chain: %s", err)
	}

	// Wait for suspended reaper to finish. If it takes too long, some jobs
	// haven't respond quickly to being stopped. Stop waiting for these jobs by
	// stopping the suspended reaper.
	select {
	case <-suspendedReaperChan:
	case <-time.After(t.stopTimeout):
		t.logger.Warnf("timed out waiting for jobs to stop - stopping reaper")
		t.reaper.Stop()
	}
	close(t.doneChan)
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

	// If there was an error when stopping at least one of the jobs, return it.
	runnerWG.Wait()
	if hadError {
		return fmt.Errorf("problem stopping one or more job runners - see logs for more info")
	}
	return nil
}
