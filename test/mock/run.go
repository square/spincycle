// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

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
	RunCompleted bool
	StatusResp   string
	RunBlock     chan struct{} // Channel that Runner.Run() will block on, if defined.
	StopChan     chan struct{} // Channel used to stop a blocked Runner.Run().
}

func (r *Runner) Run(jobData map[string]string) bool {
	if r.RunBlock != nil && r.StopChan != nil {
	LOOP:
		for {
			select {
			case <-r.RunBlock:
				break LOOP
			case <-r.StopChan:
				return false
			}
		}
	} else if r.RunBlock != nil {
		<-r.RunBlock
	}
	return r.RunCompleted
}

func (r *Runner) Stop() error {
	return nil
}

func (r *Runner) Status() string {
	return r.StatusResp
}
