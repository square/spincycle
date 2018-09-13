// Copyright 2018, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
)

var (
	ErrJobReaper = errors.New("forced error in job reaper")
)

type JobReaper struct {
	ReapFunc     func(proto.Job)
	FinalizeFunc func()
}

func (r *JobReaper) Reap(job proto.Job) {
	if r.ReapFunc != nil {
		r.ReapFunc(job)
	}
}

func (r *JobReaper) Finalize() {
	if r.FinalizeFunc != nil {
		r.FinalizeFunc()
	}
}

type ReaperFactory struct {
	ReapFunc     func(proto.Job)
	FinalizeFunc func()
}

func (rf *ReaperFactory) Make() chain.JobReaper {
	return &JobReaper{
		ReapFunc:     rf.ReapFunc,
		FinalizeFunc: rf.FinalizeFunc,
	}
}

func (rf *ReaperFactory) MakeRunning(runJobChan chan proto.Job) chain.JobReaper {
	return rf.Make()
}

func (rf *ReaperFactory) MakeSuspended() chain.JobReaper {
	return rf.Make()
}

func (rf *ReaperFactory) MakeStopped() chain.JobReaper {
	return rf.Make()
}
