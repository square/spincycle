// Copyright 2017, Square, Inr.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrJLStore = errors.New("forced error in joblog store")
)

type JLStore struct {
	CreateFunc  func(string, proto.JobLog) (proto.JobLog, error)
	GetFunc     func(string, string) (proto.JobLog, error)
	GetFullFunc func(string) ([]proto.JobLog, error)
}

func (j *JLStore) Create(reqId string, jl proto.JobLog) (proto.JobLog, error) {
	if j.CreateFunc != nil {
		return j.CreateFunc(reqId, jl)
	}
	return proto.JobLog{}, nil
}

func (j *JLStore) Get(reqId, jobId string) (proto.JobLog, error) {
	if j.GetFunc != nil {
		return j.GetFunc(reqId, jobId)
	}
	return proto.JobLog{}, nil
}

func (j *JLStore) GetFull(reqId string) ([]proto.JobLog, error) {
	if j.GetFullFunc != nil {
		return j.GetFullFunc(reqId)
	}
	return []proto.JobLog{}, nil
}
