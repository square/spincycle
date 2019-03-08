// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
)

var (
	ErrTraverser = errors.New("forced error in traverser")
)

type Traverser struct {
	RunErr     error
	StopErr    error
	StatusResp proto.JobChainStatus
	StatusErr  error
}

func (t *Traverser) Run() {
	return
}

func (t *Traverser) Stop() error {
	return t.StopErr
}

func (t *Traverser) Status() (proto.JobChainStatus, error) {
	return t.StatusResp, t.StatusErr
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
