// Copyright 2017-2018, Square, Inc.

package chain

import (
	"fmt"
	"sync"
	"sync/atomic"
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
	Make(*proto.JobChain) (Traverser, error)
	MakeFromSJC(*proto.SuspendedJobChain) (Traverser, error)
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
func (f *traverserFactory) Make(jobChain *proto.JobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := NewChain(jobChain, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	return f.make(chain)
}

// MakeFromSJC makes a Traverser from a suspended job chain.
func (f *traverserFactory) MakeFromSJC(sjc *proto.SuspendedJobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := NewChain(sjc.JobChain, sjc.SequenceTries, sjc.TotalJobTries, sjc.LatestRunJobTries)
	return f.make(chain)
}

// Creates a new Traverser from a chain. Used for both new and resumed chains.
func (f *traverserFactory) make(chain *Chain) (Traverser, error) {
	// Add chain to repo. This used to save the chain in Redis, if configured,
	// but now it's only an in-memory map. The only functionality it serves is
	// preventing this JR instance from running the same job chain.
	if err := f.chainRepo.Add(chain); err != nil {
		return nil, fmt.Errorf("error adding job chain: %s", err)
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
	return NewTraverser(cfg), nil
}

// -------------------------------------------------------------------------- //

type traverser struct {
	reaperFactory ReaperFactory
	reaper        JobReaper

	shutdownChan chan struct{}  // indicates JR is shutting down
	runJobChan   chan proto.Job // jobs to be run
	doneJobChan  chan proto.Job // jobs that are done
	doneChan     chan struct{}  // closed when traverser finishes running

	stopMux     *sync.RWMutex // lock around checks to stopped
	stopped     bool          // has traverser been stopped
	suspended   bool          // has traverser been suspended
	stopChan    chan struct{} // don't run jobs in runJobs
	pendingChan chan struct{} // runJobs closes on return
	pending     int64         // N runJob goroutines are pending runnerRepo.Set

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
		stopChan:      make(chan struct{}),
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

	defer t.chainRepo.Remove(t.chain.RequestId())

	// Update finished_jobs count which determines request progress. For
	if err := t.rmc.FinishedJobs(t.chain.FinishedJobs()); err != nil {
	}

	// Start a goroutine to run jobs. This consumes runJobChan. When jobs are done,
	// they're sent to doneJobChan, which a reaper consumes. This goroutine returns
	// when runJobChan is closed below.
	go t.runJobs()

	// Enqueue all the first runnable jobs. For a new job chain (not suspended),
	// this'll be the first job in the chain. For a resumed job chain, it'll be
	// stopped jobs (see Chain.isRunnable).
	for _, job := range t.chain.RunnableJobs() {
		t.logger.Infof("sending initial job (%s) to runJobChan", job.Id)
		t.runJobChan <- job
	}

	// Start a goroutine to reap done jobs. The runningReaper consumes from
	// doneJobChan and sends the next jobs to be run to runJobChan. Stop()
	// calls t.reaper.Stop(), which is this reaper. The close(t.runJobChan)
	// causes runJobs() (started above ^) to return.
	runningReaperChan := make(chan struct{})
	t.reaper = t.reaperFactory.MakeRunning() // t.reaper = runningReaper
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
	}
	if t.suspended {
		return ErrShuttingDown
	}
	close(t.stopChan)
	t.stopped = true
	t.logger.Infof("stopping traverser and all jobs")

	// Stop the runningReaper and start the stoppedReaper which saves jobs' states
	// but doesn't enqueue any more jobs to run. It sends the chain's final state
	// to teh RM when all jobs have stopped running.
	t.reaper.Stop() // blocks until runningReaper stops
	stoppedReaperChan := make(chan struct{})
	t.reaper = t.reaperFactory.MakeStopped() // t.reaper = stoppedReaper
	go func() {
		defer close(stoppedReaperChan)
		t.reaper.Run()
	}()

	// Stop all job runners in the runner repo. Do this after switching to the
	// stopped reaper so that when the jobs finish and are sent on doneJobChan,
	// they are reaped correctly.
	timeout := time.After(t.stopTimeout)
	err := t.stopRunningJobs(timeout)
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
	case <-timeout:
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
// channel. When the job is done, it sends the job out through the doneJobChan
// which is being consumed by a reaper.
func (t *traverser) runJobs() {
	t.logger.Infof("runJobs call")
	defer t.logger.Infof("runJobs return")
	defer close(t.pendingChan)

	// Run all jobs that come in on runJobChan. The loop exits when runJobChan
	// is closed in the runningReaper goroutine in Run().
	for job := range t.runJobChan {

		// Explicitly pass the job into the func, or all goroutines would share
		// the same loop "job" variable.
		go func(job proto.Job) {
			jLogger := t.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": t.chain.SequenceTries(job.Id)})

			// Don't run the job if traverser stopped or shutting down. We can't
			// do this check before running this goroutine because runJobChan is
			// unbuffered, so runningReaper would block trying to enqueue next jobs.
			// As long as we do not add job to runner repo (or do anything else
			// to job), we can simply return from the goroutine and it's the same
			// as never having run the job. If/when chain resumes, the job will
			// still be PENDING and tries=0.
			select {
			case <-t.stopChan:
				jLogger.Info("traverser stopped or shutting down, not running job")
				return
			default:
			}

			// Signal to stopRunningJobs that there's +1 goroutine that's going
			// to add itself to runnerRepo
			atomic.AddInt64(&t.pending, 1)

			// Always send the finished job to doneJobChan to be reaped. If the
			// reaper isn't reaping any more jobs (if this job took too long to
			// finish after being stopped), sending to doneJobChan won't be
			// possible - timeout after a while so we don't leak this goroutine.
			defer func() {
				select {
				case t.doneJobChan <- job: // reap the done job
				case <-time.After(t.sendTimeout):
					jLogger.Warnf("timed out sending job to doneJobChan")
				}
				// Remove the job's runner from the repo (if it was ever added)
				// AFTER sending it to doneJobChan. This avoids a race condition
				// when the stopped + suspended reapers check if the runnerRepo
				// is empty.
				t.runnerRepo.Remove(job.Id)
			}()

			// Increment sequence try count if this is sequence start job, which
			// currently means sequenceId == job.Id.
			if t.chain.IsSequenceStartJob(job.Id) {
				jLogger.Infof("sequence start job")
				t.chain.IncrementSequenceTries(job.Id, 1)
			}

			// If job state is STOPPED, it's most likely because we're resuming
			// this chain. Stopped jobs are re-runnable. Its current try count
			// is the job try on which it was stopped, but stopping isn't a failure
			// (retry only applies to failure), so roll back the try count by 1.
			if job.State == proto.STATE_STOPPED {
				t.chain.IncrementJobTries(job.Id, -1) // negative only decr current tries
			}

			// Job tries for current sequence try and total tries for all seq tries.
			// For new chains, these are zero. For suspended/resumed chains they can
			// be > 0 which is why we pass them to the job runner: to resume for the
			// last counts.
			curTries, totalTries := t.chain.JobTries(job.Id)

			runner, err := t.rf.Make(job, t.chain.RequestId(), curTries, totalTries)
			if err != nil {
				// Problem creating the job runner - treat job as failed.
				// Send a JobLog to the RM so that it knows this job failed.
				atomic.AddInt64(&t.pending, -1)
				job.State = proto.STATE_FAIL
				err = fmt.Errorf("problem creating job runner: %s", err)
				t.sendJL(job, err)
				return
			}

			// --------------------------------------------------------------

			// Add the runner to the repo. Runners in the repo are used
			// by the Status, Stop, and shutdown methods on the traverser.
			// Then decrement pending to signal to stopRunningJobs that
			// there's one less goroutine it nees to wait for.
			t.runnerRepo.Set(job.Id, runner)
			atomic.AddInt64(&t.pending, -1)

			// Run the job. This is a blocking operation that could take a long time.
			jLogger.Infof("running job")
			t.chain.SetJobState(job.Id, proto.STATE_RUNNING)
			ret := runner.Run(job.Data)
			jLogger.Infof("job done: state=%s (%d)", proto.StateName[ret.FinalState], ret.FinalState)

			// We don't pass the Chain to the job runner, so it can't call this
			// itself. Instead, it returns how many tries it did, and we set it.
			t.chain.IncrementJobTries(job.Id, int(ret.Tries))

			// Set job final state because this job is about to be reaped on
			// the doneJobChan, sent in this goroutine's defer func at top ^.
			job.State = ret.FinalState
		}(job)
	}
}

// sendJL sends a job log to the Request Manager.
func (t *traverser) sendJL(job proto.Job, err error) {
	_, totalTries := t.chain.JobTries(job.Id)
	jLogger := t.logger.WithFields(log.Fields{"job_id": job.Id})
	jl := proto.JobLog{
		RequestId:  t.chain.RequestId(),
		JobId:      job.Id,
		Name:       job.Name,
		Type:       job.Type,
		Try:        totalTries,
		StartedAt:  0, // zero because the job never ran
		FinishedAt: 0,
		State:      job.State,
		Exit:       1,
	}
	if err != nil {
		jl.Error = err.Error()
	}
	err = retry.Do(jobLogTries, jobLogRetryWait,
		func() error {
			return t.rmc.CreateJL(t.chain.RequestId(), jl)
		},
		nil,
	)
	if err != nil {
		jLogger.Errorf("problem sending job log (%#v) to the Request Manager: %s", jl, err)
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
	close(t.stopChan)
	t.suspended = true
	t.logger.Info("suspending job chain - stopping all jobs")

	// Stop the runningReaper and start the suspendedReaper which saves jobs'
	// states and prepares the chain to be resumed later but doesn't enqueue
	// any more jobs to run. When all jobs have stopped running, it sends a
	// SuspendedJobChain to the RM, or the final state if the chain completed
	// or failed.
	t.reaper.Stop() // blocks until runningReaper stops
	suspendedReaperChan := make(chan struct{})
	t.reaper = t.reaperFactory.MakeSuspended() // t.reaper = suspendedReaper
	go func() {
		defer close(suspendedReaperChan)
		t.reaper.Run()
	}()

	// Stop all job runners in the runner repo. Do this after switching to the
	// suspended reaper so that when the jobs finish and are sent on doneJobChan,
	// they are reaped correctly.
	timeout := time.After(t.stopTimeout)
	err := t.stopRunningJobs(timeout)
	if err != nil {
		t.logger.Errorf("problem suspending job chain: %s", err)
	}

	// Wait for suspended reaper to finish. If it takes too long, some jobs
	// haven't respond quickly to being stopped. Stop waiting for these jobs by
	// stopping the suspended reaper.
	select {
	case <-suspendedReaperChan:
	case <-timeout:
		t.logger.Warnf("timed out waiting for jobs to stop - stopping reaper")
		t.reaper.Stop()
	}
	close(t.doneChan)
}

// stopRunningJobs stops all currently running jobs.
func (t *traverser) stopRunningJobs(timeout <-chan time.Time) error {
	// To stop all running jobs without race coditions, we need to know:
	//   1. runJobs is done, won't start any more goroutines
	//   2. All in-flight runJob goroutines have added themselves to runner repo
	// First is easy: wait for it to close pendingChan. Second is like a wait
	// group wait: runJobs add +1 to pending when goroutine starts, and -1 after
	// it adds itself to runner repo. So all runJob goroutines have added
	// themselves to the runner repo when pending == 0.
	//
	// The shutdown sequence is:
	//   1. close(stopChan): runJob goroutines (RGs) check this very first, so
	//      new RGs return immediately, as if the job never ran. This allows
	//      runJobChan to drain and prevents runnerRepo from blocking because
	//      the chan is unbuffered. 2nd thing RGs do: pending+1. Therefore, all
	//      in-flight RGs (i.e. RGs already started) are guaranteed to be added
	//      to the pending count.
	//   2. Stop runningReaper: This stops new/next jobs into runJobChan, which
	//      is being drained because of step 1.
	//   3. close(runJobChan): When runningReaper.Run returns, the goroutine in
	//      in traverser.Run closes runJobChan. Since runningReaper is only thing
	//      that sends to runJobChan, it must be closed like this so runningReaper
	//      doesn't panic on "send on closed channel".
	//   4. close(pendingChan): Given step 3 and step 1, eventually runJobChan
	//      will drain and runJobs() will return, closing pendingChan when it does.
	//   5. Call stopRunningJobs: This func waits for step 4, which ensures no
	//      more RGs. And given step 1, we're assured that any in-flight RGs _must_
	//      have added themsevs to pending count. Therefore, this func waits for
	//      pending count == 0 which means all RGs have added themselves to the
	//      runner repo.
	//   6. Stop all active runners in runner repo.
	//
	// Strictly speaking, there's one insane race condition. If an RG checks stopChan
	// and it's _not_ stopped, then we do steps 1 through 5, and then the RG does
	// pending+1, it could be missed in step 6. That race is practically impossible.

	// Wait for runJobs to return
	select {
	case <-t.pendingChan:
	case <-timeout:
		return fmt.Errorf("stopRunningJobs: timeout waiting for pendingChan")
	}

	// Wait for in-flight runJob goroutines to add themselves to runner repo
	if n := atomic.LoadInt64(&t.pending); n > 0 {
		for atomic.LoadInt64(&t.pending) > 0 {
			select {
			case <-timeout:
				return fmt.Errorf("stopRunningJobs: timeout waiting for pending count")
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	// Get all of the active runners for this traverser from the repo. Only runners
	// that are in the repo will be stopped.
	activeRunners, err := t.runnerRepo.Items()
	if err != nil {
		return fmt.Errorf("problem retrieving job runners from repo: %s", err)
	}
	t.logger.Printf("stopping %d active job runners", len(activeRunners))

	// Stop all runners in parallel in case some jobs don't stop quickly
	var wg sync.WaitGroup
	hadError := false
	for jobId, activeRunner := range activeRunners {
		wg.Add(1)
		go func(runner runner.Runner) {
			defer wg.Done()
			if err := runner.Stop(); err != nil {
				t.logger.Errorf("problem stopping job runner (job id = %s): %s", jobId, err)
				hadError = true
			}
		}(activeRunner)
	}
	wg.Wait()

	// If there was an error when stopping at least one of the jobs, return it.
	if hadError {
		return fmt.Errorf("problem stopping one or more job runners - see logs for more info")
	}
	return nil
}
