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

func (t *Traverser) Run() error {
	return t.RunErr
}

func (t *Traverser) Stop() error {
	return t.StopErr
}

func (t *Traverser) Status() (proto.JobChainStatus, error) {
	return t.StatusResp, t.StatusErr
}

type TraverserFactory struct {
	MakeFunc func(proto.JobChain) (chain.Traverser, error)
}

func (tf *TraverserFactory) Make(jc proto.JobChain) (chain.Traverser, error) {
	if tf.MakeFunc != nil {
		return tf.MakeFunc(jc)
	}
	return &Traverser{}, nil
}
