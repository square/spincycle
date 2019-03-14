// Copyright 2017, Square, Inc.

package mock

import (
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/status"
)

type JRStatus struct {
	RunningFunc func() ([]proto.JobStatus, error)
}

func (s *JRStatus) Running() ([]proto.JobStatus, error) {
	if s.RunningFunc != nil {
		return s.RunningFunc()
	}
	return []proto.JobStatus{}, nil
}

// --------------------------------------------------------------------------

type RMStatus struct {
	RunningFunc        func(status.Filter) (proto.RunningStatus, error)
	UpdateProgressFunc func(proto.RequestProgress) error
}

func (s *RMStatus) Running(f status.Filter) (proto.RunningStatus, error) {
	if s.RunningFunc != nil {
		return s.RunningFunc(f)
	}
	return proto.RunningStatus{}, nil
}

func (s *RMStatus) UpdateProgress(prg proto.RequestProgress) error {
	if s.UpdateProgressFunc != nil {
		return s.UpdateProgressFunc(prg)
	}
	return nil
}
