// Copyright 2017, Square, Inr.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrJLManager = errors.New("forced error in jl manager")
)

type JLManager struct {
	CreateFunc  func(string, proto.JobLog) (proto.JobLog, error)
	GetFunc     func(string, string) (proto.JobLog, error)
	GetFullFunc func(string) ([]proto.JobLog, error)
}

func (j *JLManager) Create(reqId string, jl proto.JobLog) (proto.JobLog, error) {
	if j.CreateFunc != nil {
		return j.CreateFunc(reqId, jl)
	}
	return proto.JobLog{}, nil
}

func (j *JLManager) Get(reqId, jobId string) (proto.JobLog, error) {
	if j.GetFunc != nil {
		return j.GetFunc(reqId, jobId)
	}
	return proto.JobLog{}, nil
}

func (j *JLManager) GetFull(reqId string) ([]proto.JobLog, error) {
	if j.GetFullFunc != nil {
		return j.GetFullFunc(reqId)
	}
	return []proto.JobLog{}, nil
}
