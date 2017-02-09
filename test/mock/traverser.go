// Copyright 2017, Square, Inc.

package mock

import (
	"github.com/square/spincycle/proto"
)

type Traverser struct {
	RunErr     error
	StatusResp proto.JobChainStatus
}

func (t *Traverser) Run() error {
	return t.RunErr
}

func (t *Traverser) Stop() {
}

func (t *Traverser) Status() proto.JobChainStatus {
	return t.StatusResp
}
