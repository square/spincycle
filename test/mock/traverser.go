// Copyright 2017, Square, Inc.

package mock

import (
	"github.com/square/spincycle/proto"
)

type Traverser struct {
	RunErr     error
	StopErr    error
	StatusResp proto.JobChainStatus
}

func (t *Traverser) Run() error {
	return t.RunErr
}

func (t *Traverser) Stop() error {
	return t.StopErr
}

func (t *Traverser) Status() proto.JobChainStatus {
	return t.StatusResp
}
