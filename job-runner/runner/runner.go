// Copyright 2017-2019, Square, Inc.

// Package runner implements running a job.
package runner

import (
	"fmt"
	"sync"
	"time"

	"github.com/square/spincycle/v2/job"
	"github.com/square/spincycle/v2/proto"
	rm "github.com/square/spincycle/v2/request-manager"
	"github.com/square/spincycle/v2/retry"

	log "github.com/sirupsen/logrus"
)

const (
	// Number of times to attempt sending a job log to the RM.
	JOB_LOG_TRIES = 5
	// Time to wait between attempts to send a job log to RM.
	JOB_LOG_RETRY_WAIT = 500 * time.Millisecond
)

type Return struct {
	FinalState byte // Final proto.STATE_*. Determines if/how chain continues running.
	Tries      uint // Number of tries this run, not including any previous tries
}

type Status struct {
	Job       proto.Job
	StartedAt time.Time // set once when Runner created
	Try       uint      // total tries, not current sequence try (proto.JobLog.Try)
	Status    string    // real-time job status (job.Job.Status())
	Sleeping  bool      // if sleeping between tries
}

// A Runner runs and manages one job in a job chain. The job must implement the
// job.Job interface.
type Runner interface {
	// Run runs the job, blocking until it has completed or when Stop is called.
	// If the job fails, Run will retry it as many times as the job is configured
	// to be retried. After each run attempt, a Job Log is created and sent to
	// the RM. When the job successfully completes, or reaches the maximum number
	// of retry attempts, Run returns the final state of the job.
	Run(jobData map[string]interface{}) Return

	// Stop stops the job if it's running. The job is responsible for stopping
	// quickly because Stop blocks while waiting for the job to stop.
	Stop() error

	// Status returns the job try count and real-time status. The runner handles
	// the try count. The underlying job.Job must handle async, real-time status
	// requests while running.
	Status() Status
}

// A runner represents all information needed to run a job.
type runner struct {
	pJob    proto.Job
	realJob job.Job   // the actual job interface to run
	reqId   string    // the request id the job belongs to
	rmc     rm.Client // client used to send JLs to the RM
	// --
	jobId      string
	jobName    string
	jobType    string
	prevTries  uint // tries previous run (on resume/retry)
	totalTries uint // try count all seq tries
	maxTries   uint // max tries per seq try, not global maxTry in request spec (once implemented)
	retryWait  time.Duration
	stopChan   chan struct{}
	*sync.Mutex
	logger    *log.Entry
	startTime time.Time
	sleeping  bool
}

// NewRunner takes a proto.Job struct and its corresponding job.Job interface, and
// returns a Runner.
func NewRunner(pJob proto.Job, realJob job.Job, reqId string, prevTries, totalTries uint, rmc rm.Client) Runner {
	var retryWait time.Duration
	if pJob.RetryWait != "" {
		retryWait, _ = time.ParseDuration(pJob.RetryWait) // validated by grapher
	} else {
		retryWait = 0
	}
	return &runner{
		pJob:       pJob,
		realJob:    realJob,
		reqId:      reqId,
		prevTries:  prevTries,
		totalTries: 1 + totalTries, // this run + past totalTries (on resume/retry)
		rmc:        rmc,
		// --
		maxTries:  1 + pJob.Retry, // + 1 because we always run once
		retryWait: retryWait,
		stopChan:  make(chan struct{}),
		Mutex:     &sync.Mutex{},
		logger:    log.WithFields(log.Fields{"request_id": reqId, "job_id": pJob.Id}),
		startTime: time.Now().UTC(),
	}
}

func (r *runner) Run(jobData map[string]interface{}) Return {
	// The chain.traverser that's calling us only cares about the final state
	// of the job. If maxTries > 1, the intermediate states are only logged if
	// the run fails.
	finalState := proto.STATE_PENDING
	tries := uint(1)         // number of tries this run
	tryNo := 1 + r.prevTries // this run + past tries (on resume/retry)
TRY_LOOP:
	for tryNo <= r.maxTries {
		tryLogger := r.logger.WithFields(log.Fields{
			"try":       r.totalTries,
			"tries":     tryNo,
			"max_tries": r.maxTries,
		})

		// Can be stopped before we've started. Although we never started, we
		// must set final state = stopped so that this try is re-run on resume.
		if r.stopped() {
			tryLogger.Infof("job stopped before start")
			finalState = proto.STATE_STOPPED
			break TRY_LOOP
		}

		// Run the job. Use a separate method so we can easily recover from a panic
		// in job.Run.
		tryLogger.Infof("job start")
		startedAt, finishedAt, jobRet, runErr := r.runJob(jobData)
		runtime := time.Duration(finishedAt-startedAt) * time.Nanosecond
		tryLogger.Infof("job return: runtime=%s, state=%s (%d), exit=%d, err=%v", runtime, proto.StateName[jobRet.State], jobRet.State, jobRet.Exit, runErr)

		// Figure out what the error message in the JL should be. An
		// error returned by Run takes precedence (because it implies
		// a high-level error with the job), followed by the error
		// returned in the job.Return struct from the job itself (which
		// probably won't even be meaningful if runErr != nil).
		var errMsg string
		if runErr != nil {
			errMsg = runErr.Error()
		} else if jobRet.Error != nil {
			errMsg = jobRet.Error.Error()
		}

		// Can be stopped while running, in which case STATE_FAIL is not really
		// because it failed but because we stopped it, so log then overwrite
		// the state = stopped. This also sets finalState below.
		if r.stopped() {
			if jobRet.State != proto.STATE_STOPPED && jobRet.State != proto.STATE_COMPLETE {
				tryLogger.Errorf("job stopped: changing state %s (%d) to STATE_STOPPED", proto.StateName[jobRet.State], jobRet.State)
				jobRet.State = proto.STATE_STOPPED
			}
		}

		// Create a JL and send it to the RM.
		jl := proto.JobLog{
			RequestId:  r.reqId,
			JobId:      r.pJob.Id,
			Name:       r.pJob.Name,
			Type:       r.pJob.Type,
			Try:        r.totalTries,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			State:      jobRet.State,
			Exit:       jobRet.Exit,
			Error:      errMsg,
			Stdout:     jobRet.Stdout,
			Stderr:     jobRet.Stderr,
		}
		err := retry.Do(JOB_LOG_TRIES, JOB_LOG_RETRY_WAIT,
			func() error { return r.rmc.CreateJL(r.reqId, jl) },
			func(err error) { tryLogger.Warnf("error sending job log entry: %s (retrying)", err) },
		)
		if err != nil {
			tryLogger.Errorf("failed to send job log entry: %s (%+v)", err, jl)
		}

		// Set final job state to this job state
		finalState = jobRet.State

		// Break try loop on success or stop
		if jobRet.State == proto.STATE_COMPLETE || jobRet.State == proto.STATE_STOPPED {
			break TRY_LOOP
		}

		// //////////////////////////////////////////////////////////////////
		// Job failed, wait and retry?
		// //////////////////////////////////////////////////////////////////
		tryLogger.Warnf("job failed: state %s (%d), %d try left", proto.StateName[jl.State], jl.State, r.maxTries-tryNo)

		// If last try, break retry loop, don't wait
		if tryNo == r.maxTries {
			break TRY_LOOP
		}

		// Wait between retries. Can be stopped while waiting which is why we
		// need to increment tryNo first. At this point, we're effectively on
		// the next try. E.g. try 1 fails, we're waiting for try 2, then we're
		// stopped: we want try 2 state=stopped so on resume try 2 is re-run.
		r.Lock()
		tries++
		tryNo++
		r.totalTries++
		r.sleeping = true
		r.Unlock()
		select {
		case <-time.After(r.retryWait):
			r.Lock()
			r.sleeping = false
			r.Unlock()
		case <-r.stopChan:
			tryLogger.Infof("job stopped while waiting to run try %d", r.totalTries)
			finalState = proto.STATE_STOPPED
			break TRY_LOOP
		}
	}

	return Return{
		FinalState: finalState,
		Tries:      tries,
	}
}

// Actually run the job.
func (r *runner) runJob(jobData map[string]interface{}) (startedAt, finishedAt int64, ret job.Return, err error) {
	defer func() {
		// Recover from a panic inside Job.Run()
		if panicErr := recover(); panicErr != nil {
			// Set named return values. startedAt will already be set before
			// the panic.
			finishedAt = time.Now().UnixNano()
			ret = job.Return{
				State: proto.STATE_FAIL,
				Exit:  1,
			}
			// The returned error will be used in the job log entry.
			err = fmt.Errorf("panic from job.Run: %s", panicErr)
		}
	}()

	// Run the job. Run is a blocking operation that could take a long
	// time. Run will return when a job finishes running (either by
	// its own accord or by being forced to finish when Stop is called).
	startedAt = time.Now().UnixNano()
	jobRet, runErr := r.realJob.Run(jobData)
	finishedAt = time.Now().UnixNano()

	return startedAt, finishedAt, jobRet, runErr
}

func (r *runner) Stop() error {
	r.Lock() // LOCK

	// Return if stop was already called.
	select {
	case <-r.stopChan:
		r.Unlock() // UNLOCK
		return nil
	default:
	}

	close(r.stopChan)

	r.Unlock() // UNLOCK

	r.logger.Infof("stopping the job")
	return r.realJob.Stop() // this is a blocking operation that should return quickly
}

func (r *runner) stopped() bool {
	select {
	case <-r.stopChan:
		return true
	default:
		return false
	}
}

func (r *runner) Runtime() float64 {
	return time.Now().Sub(r.startTime).Seconds()
}

func (r *runner) Status() Status {
	// Get real-time status before locking in case it's slow
	status := r.realJob.Status()

	r.Lock()
	defer r.Unlock()

	// Indicate in real-time status if job is sleep, i.e. not truly running
	if r.sleeping {
		status = "(retry sleep) " + status
	}

	return Status{
		Job:       r.pJob,
		StartedAt: r.startTime,
		Try:       r.totalTries,
		Status:    status,
		Sleeping:  r.sleeping,
	}
}
