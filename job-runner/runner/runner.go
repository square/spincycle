// Copyright 2017, Square, Inc.

// Package runner implements running a job.
package runner

import (
	"sync"

	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"

	log "github.com/Sirupsen/logrus"
)

// A Runner runs and manages one job in a job chain. The job must implement
// the Job interface (spincycle/job.Job).
type Runner interface {
	// Run runs the job, blocking until it has completed or when Stop is called.
	// It returns true if the job completes, else false. Jobs are all or nothing
	// so "completes" means the returns on its own (isn't stopped) with no error
	// and a zero exit. jobData from the previous job is passed to the job, and
	// the job is free to write to it.
	Run(jobData map[string]interface{}) bool

	// Stop stops the job if it's running. The job is responsible for stopping
	// quickly because Stop blocks while waiting for the job to stop. Stop
	// returns the error from calling Stop interface method of the job.
	Stop() error

	// Status returns the status of the job as reported by the job. The job
	// is responsible for handling status requests asynchronously while running.
	Status() string
}

// A JobRunner represents all information needed to run a job.
type JobRunner struct {
	job       job.Job // job to run
	requestId string  // for logging
	// --
	stopChan    chan struct{} // used on Stop
	running     bool          // true when Run is running
	*sync.Mutex               // guards running
}

// NewJobRunner returns a JobRunner for a job.
func NewJobRunner(job job.Job, requestId string) *JobRunner {
	return &JobRunner{
		job:       job,
		requestId: requestId,
		// --
		stopChan: make(chan struct{}),
		running:  false,
		Mutex:    &sync.Mutex{},
	}
}

func (r *JobRunner) Run(jobData map[string]interface{}) bool {
	r.Lock()
	log.Infof("[chain=%s,job=%s]: Starting the job.", r.requestId, r.job.Name())
	stateChan := make(chan byte, 1) // must be buffered!
	go r.runJob(jobData, stateChan)
	r.running = true
	r.Unlock()

	defer func() {
		r.Lock()
		r.running = false
		r.Unlock()
	}()

	// Wait for job to finish or a call to Stop
	select {
	case state := <-stateChan: // job finished
		switch state {
		case proto.STATE_COMPLETE:
			log.Infof("[chain=%s,job=%s]: Job completed successfully.", r.requestId, r.job.Name())
			return true
		default:
			log.Errorf("[chain=%s,job=%s]: Job did not complete successfully (state: %s).", r.requestId, r.job.Name(), proto.StateName[state])
			return false
		}
	case <-r.stopChan: // Stop called
		return false
	}
}

func (r *JobRunner) Stop() error {
	r.Lock()
	defer r.Unlock()

	if !r.running {
		return nil
	}

	// Stop Run() which is waiting on either runJob() or this:
	close(r.stopChan)

	// Stop is a blocking call that should return quickly.
	err := r.job.Stop()
	if err != nil {
		log.Errorf("[chain=%s,job=%s]: Error stopping job (error: %s).", r.requestId, r.job.Name(), err)
	} else {
		log.Infof("[chain=%s,job=%s]: Job stopped successfully.", r.requestId, r.job.Name())
	}
	return err
}

func (r *JobRunner) Status() string {
	log.Infof("[chain=%s,job=%s]: Getting job status.", r.requestId, r.job.Name())
	// job.Status is a blocking operation that is expected to return quickly.
	return r.job.Status()
}

// -------------------------------------------------------------------------- //

// runJob runs a job and creates a job log entry when it's done.
func (r *JobRunner) runJob(jobData map[string]interface{}, stateChan chan byte) {
	// job.Run is a blocking operation that could take a long time.
	jobReturn, err := r.job.Run(jobData)
	if err != nil {
		log.Errorf("[chain=%s,job=%s]: Error running job (error: %s).", r.requestId, r.job.Name(), err)
	}

	log.Infof("[chain=%s,job=%s]: Job Return - state: %s, exit code: %d, error message: %s, stdout: %s, "+
		"stderr: %s.", r.requestId, r.job.Name(), proto.StateName[jobReturn.State], jobReturn.Exit,
		jobReturn.Error, jobReturn.Stdout, jobReturn.Stderr)

	// create a JobLogEntry and ship it off
	// jle := NewJLE(jobData, jobReturn, err)
	// jle.Send()

	stateChan <- jobReturn.State
}
