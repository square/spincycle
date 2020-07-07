// Copyright 2017-2019, Square, Inc.

package mock

import (
	"errors"
	"sync"

	"github.com/square/spincycle/v2/job-runner/runner"
	"github.com/square/spincycle/v2/proto"
)

var (
	ErrRunner = errors.New("forced error in runner")
)

type RunnerFactory struct {
	RunnersToReturn map[string]*Runner // Keyed on job name.
	MakeErr         error
	MakeFunc        func(job proto.Job, requestId string, prevTries uint, totalTries uint) (runner.Runner, error)
}

func (f *RunnerFactory) Make(job proto.Job, requestId string, prevTries uint, totalTries uint) (runner.Runner, error) {
	if f.MakeFunc != nil {
		return f.MakeFunc(job, requestId, prevTries, totalTries)
	}
	return f.RunnersToReturn[job.Id], f.MakeErr
}

type Runner struct {
	RunReturn    runner.Return
	RunErr       error
	RunFunc      func(jobData map[string]interface{}) byte // can use this instead of RunErr and RunFunc for more involved mocks
	AddedJobData map[string]interface{}                    // Data to add to jobData.
	RunWg        *sync.WaitGroup                           // WaitGroup that gets released from when a runner starts running.
	RunBlock     chan struct{}                             // Channel that runner.Run() will block on, if defined.
	IgnoreStop   bool                                      // false: return immediately after Stop, true: keep running after Stop
	StatusResp   runner.Status

	stopped bool // if Stop was called
}

func (r *Runner) Run(jobData map[string]interface{}) runner.Return {
	// If RunFunc is defined, use that.
	if r.RunFunc != nil {
		state := r.RunFunc(jobData)
		return runner.Return{
			FinalState: state,
			Tries:      r.RunReturn.Tries,
		}
	}

	if r.RunWg != nil {
		r.RunWg.Done()
	}
	if r.RunBlock != nil {
		<-r.RunBlock
		if r.stopped && !r.IgnoreStop {
			return r.RunReturn
		}
	}
	// Add job data.
	for k, v := range r.AddedJobData {
		jobData[k] = v
	}

	return r.RunReturn
}

func (r *Runner) Stop() error {
	r.stopped = true
	if r.RunBlock != nil {
		close(r.RunBlock)
	}
	return nil
}

func (r *Runner) Status() runner.Status {
	if r.RunBlock != nil {
		close(r.RunBlock)
	}
	return r.StatusResp
}

// --------------------------------------------------------------------------

type RunnerRepo struct {
	SetFunc    func(jobId string, runner runner.Runner)
	GetFunc    func(jobId string) runner.Runner
	RemoveFunc func(jobId string)
	ItemsFunc  func() (map[string]runner.Runner, error)
	CountFunc  func() int
}

func (r RunnerRepo) Set(jobId string, runner runner.Runner) {
	if r.SetFunc != nil {
		r.Set(jobId, runner)
	}
	return
}

func (r RunnerRepo) Get(jobId string) runner.Runner {
	if r.GetFunc != nil {
		return r.GetFunc(jobId)
	}
	return nil
}

func (r RunnerRepo) Remove(jobId string) {
	if r.RemoveFunc != nil {
		r.RemoveFunc(jobId)
	}
	return
}

func (r RunnerRepo) Items() (map[string]runner.Runner, error) {
	if r.ItemsFunc != nil {
		return r.ItemsFunc()
	}
	return map[string]runner.Runner{}, nil
}

func (r RunnerRepo) Count() int {
	if r.CountFunc != nil {
		return r.CountFunc()
	}
	return 0
}
