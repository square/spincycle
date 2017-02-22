// Copyright 2017, Square, Inc.

package mock

import (
	"errors"
	"sync"

	"github.com/square/spincycle/job-runner/runner"
)

var (
	ErrRunner = errors.New("forced error in runner")
)

type RunnerFactory struct {
	RunnersToReturn map[string]*Runner // Keyed on job name.
	MakeErr         error
}

func (f *RunnerFactory) Make(jobType, jobName string, jobBytes []byte, requestId uint) (runner.Runner, error) {
	return f.RunnersToReturn[jobName], f.MakeErr
}

type Runner struct {
	runCompleted bool
	statusResp   string
	runBlock     chan struct{}     // Channel that Runner.Run() will block on, if defined.
	stopChan     chan struct{}     // Channel used to stop a blocked Runner.Run().
	jobData      map[string]string // The jobData that this runner will set.
	// --
	running     bool // true when Run is running
	*sync.Mutex      // guards running
}

func NewRunner(runCompleted bool, statusResp string, runBlock, stopChan chan struct{}, jobData map[string]string) *Runner {
	return &Runner{
		runCompleted: runCompleted,
		statusResp:   statusResp,
		runBlock:     runBlock,
		stopChan:     stopChan,
		jobData:      jobData,
		running:      false,
		Mutex:        &sync.Mutex{},
	}
}

func (r *Runner) Run(jobData map[string]string) bool {
	r.Lock() // -- lock
	r.running = true
	r.Unlock() // -- unlock

	// Set the jobData
	for k, v := range r.jobData {
		jobData[k] = v
	}

	if r.runBlock != nil && r.stopChan != nil {
	LOOP:
		for {
			select {
			case <-r.runBlock:
				// stop running when the runblock channel is closed
				break LOOP
			case <-r.stopChan:
				return false
			}
		}
	} else if r.runBlock != nil {
		<-r.runBlock
	}
	return r.runCompleted
}

func (r *Runner) Stop() error {
	return nil
}

func (r *Runner) Status() string {
	return r.statusResp
}

func (r *Runner) Running() bool {
	r.Lock()         // -- lock
	defer r.Unlock() // -- unlock
	return r.running
}
