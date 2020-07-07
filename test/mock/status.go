// Copyright 2017-2019, Square, Inc.

package mock

import (
	"github.com/square/spincycle/v2/proto"
)

type JRStatus struct {
	RunningFunc func(proto.StatusFilter) ([]proto.JobStatus, error)
}

func (s *JRStatus) Running(f proto.StatusFilter) ([]proto.JobStatus, error) {
	if s.RunningFunc != nil {
		return s.RunningFunc(f)
	}
	return []proto.JobStatus{}, nil
}

// --------------------------------------------------------------------------

type RMStatus struct {
	RunningFunc        func(proto.StatusFilter) (proto.RunningStatus, error)
	UpdateProgressFunc func(proto.RequestProgress) error
}

func (s *RMStatus) Running(f proto.StatusFilter) (proto.RunningStatus, error) {
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
