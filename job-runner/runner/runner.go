// Copyright 2017, Square, Inc.

// Package runner implements running a job.
package runner

import (
	"sync"
	"time"

	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"

	log "github.com/Sirupsen/logrus"
)

// A Runner runs and manages one job in a job chain. The job must implement the
// job.Job interface.
type Runner interface {
	// Run runs the job, blocking until it has completed or when Stop is called.
	// If the job fails, Run will retry it as many times as the job is configured
	// to be retried. After each run attempt, a Job Log is created and sent to
	// the RM. When the job successfully completes, or reaches the maximum number
	// of retry attempts, Run returns the final state of the job.
	Run(jobData map[string]interface{}) byte

	// Stop stops the job if it's running. The job is responsible for stopping
	// quickly because Stop blocks while waiting for the job to stop.
	Stop() error

	// Status returns the status of the job as reported by the job. The job
	// is responsible for handling status requests asynchronously while running.
	Status() string
}

// A runner represents all information needed to run a job.
type runner struct {
	realJob job.Job   // the actual job interface to run
	reqId   string    // the request id the job belongs to
	rmc     rm.Client // client used to send JLs to the RM
	// --
	jobId     string
	jobType   string
	maxTries  uint
	retryWait time.Duration
	stopChan  chan struct{}
	*sync.Mutex
	logger    *log.Entry
	startTime time.Time
}

// NewRunner takes a proto.Job struct and its corresponding job.Job interface, and
// returns a Runner.
func NewRunner(pJob proto.Job, realJob job.Job, reqId string, rmc rm.Client) Runner {
	return &runner{
		realJob: realJob,
		reqId:   reqId,
		rmc:     rmc,
		// --
		jobId:     pJob.Id,
		jobType:   pJob.Type,
		maxTries:  1 + pJob.Retry, // + 1 because we always run once
		retryWait: time.Duration(pJob.RetryWait) * time.Millisecond,
		stopChan:  make(chan struct{}),
		Mutex:     &sync.Mutex{},
		logger:    log.WithFields(log.Fields{"requestId": reqId, "jobId": pJob.Id}),
	}
}

func (r *runner) Run(jobData map[string]interface{}) byte {
	// The chain.traverser that's calling us only cares about the final state
	// of the job. If maxTries > 1, the intermediate states are only logged if
	// the run fails.
	var finalState byte = proto.STATE_PENDING

	r.startTime = time.Now()

TRY_LOOP:
	for tryNo := uint(1); tryNo <= r.maxTries; tryNo++ {
		tryLogger := r.logger.WithFields(log.Fields{
			"try":       tryNo,
			"max_tries": r.maxTries,
		})
		tryLogger.Infof("starting the job")

		// Run the job. Run is a blocking operation that could take a long
		// time. Run will return when a job finishes running (either by
		// its own accord or by being forced to finish when Stop is called).
		startedAt := time.Now().UnixNano()
		jobRet, runErr := r.realJob.Run(jobData)
		finishedAt := time.Now().UnixNano()

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

		// Create a JL and send it to the RM.
		jl := proto.JobLog{
			RequestId:  r.reqId,
			JobId:      r.jobId,
			Type:       r.jobType,
			Try:        tryNo,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			State:      jobRet.State,
			Exit:       jobRet.Exit,
			Error:      errMsg,
			Stdout:     jobRet.Stdout,
			Stderr:     jobRet.Stderr,
		}
		if err := r.rmc.CreateJL(r.reqId, jl); err != nil {
			tryLogger.Errorf("problem sending job log (%#v) to the RM: %s", jl, err)
		}

		// Set final job state to this job state
		finalState = jobRet.State

		// If job completed successfully, break retry loop
		if jobRet.State == proto.STATE_COMPLETE {
			tryLogger.Info("job completed successfully")
			break TRY_LOOP
		}

		// //////////////////////////////////////////////////////////////////
		// Job failed, wait and retry?
		// //////////////////////////////////////////////////////////////////

		// Log the failure
		tryLogger.Errorf("job failed because state != %s: state = %s",
			proto.StateName[proto.STATE_COMPLETE], proto.StateName[jl.State])

		// If last try, break retry loop, don't wait
		if tryNo == r.maxTries {
			break TRY_LOOP
		}

		// Wait between retries
		select {
		case <-time.After(r.retryWait):
		case <-r.stopChan:
			break TRY_LOOP // job stopped
		}
	}

	return finalState
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

func (r *runner) Runtime() float64 {
	return time.Now().Sub(r.startTime).Seconds()
}

func (r *runner) Status() string {
	r.logger.Infof("getting job status")
	return r.realJob.Status() // this is a blocking operation that should return quickly
}
