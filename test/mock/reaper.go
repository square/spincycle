// Copyright 2018, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/v2/job-runner/chain"
)

var (
	ErrJobReaper = errors.New("forced error in job reaper")
)

type JobReaper struct {
	RunFunc      func()
	StopFunc     func()
	FinalizeFunc func()
}

func (r *JobReaper) Run() {
	if r.RunFunc != nil {
		r.RunFunc()
	}
}

func (r *JobReaper) Stop() {
	if r.StopFunc != nil {
		r.StopFunc()
	}
}

func (r *JobReaper) Finalize() {
	if r.FinalizeFunc != nil {
		r.FinalizeFunc()
	}
}

type ReaperFactory struct {
	RunFunc      func()
	StopFunc     func()
	FinalizeFunc func()
}

func (rf *ReaperFactory) Make() chain.JobReaper {
	return &JobReaper{
		RunFunc:      rf.RunFunc,
		StopFunc:     rf.StopFunc,
		FinalizeFunc: rf.FinalizeFunc,
	}
}

func (rf *ReaperFactory) MakeRunning() chain.JobReaper {
	return rf.Make()
}

func (rf *ReaperFactory) MakeSuspended() chain.JobReaper {
	return rf.Make()
}

func (rf *ReaperFactory) MakeStopped() chain.JobReaper {
	return rf.Make()
}
