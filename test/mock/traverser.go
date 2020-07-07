// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/v2/job-runner/chain"
	"github.com/square/spincycle/v2/proto"
)

var (
	ErrTraverser = errors.New("forced error in traverser")
)

type Traverser struct {
	RunErr    error
	StopErr   error
	StatusErr error
	JobStatus []proto.JobStatus
}

func (t *Traverser) Run() {
	return
}

func (t *Traverser) Stop() error {
	return t.StopErr
}

func (t *Traverser) Running() []proto.JobStatus {
	if t.JobStatus != nil {
		return t.JobStatus
	}
	return []proto.JobStatus{}
}

type TraverserFactory struct {
	MakeFunc        func(*proto.JobChain) (chain.Traverser, error)
	MakeFromSJCFunc func(*proto.SuspendedJobChain) (chain.Traverser, error)
}

func (tf *TraverserFactory) Make(jc *proto.JobChain) (chain.Traverser, error) {
	if tf.MakeFunc != nil {
		return tf.MakeFunc(jc)
	}
	return &Traverser{}, nil
}

func (tf *TraverserFactory) MakeFromSJC(sjc *proto.SuspendedJobChain) (chain.Traverser, error) {
	if tf.MakeFromSJCFunc != nil {
		return tf.MakeFromSJCFunc(sjc)
	}
	return &Traverser{}, nil
}
