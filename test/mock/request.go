// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrRequestManager = errors.New("forced error in request manager")
)

type RequestManager struct {
	CreateFunc                func(proto.CreateRequestParams) (proto.Request, error)
	GetFunc                   func(string) (proto.Request, error)
	StartFunc                 func(string) error
	StopFunc                  func(string) error
	FinishFunc                func(string, proto.FinishRequestParams) error
	StatusFunc                func(string) (proto.RequestStatus, error)
	IncrementFinishedJobsFunc func(string) error
	SpecsFunc                 func() []proto.RequestSpec
	JobChainFunc              func(string) (proto.JobChain, error)
}

func (r *RequestManager) Create(reqParams proto.CreateRequestParams) (proto.Request, error) {
	if r.CreateFunc != nil {
		return r.CreateFunc(reqParams)
	}
	return proto.Request{}, nil
}

func (r *RequestManager) Get(reqId string) (proto.Request, error) {
	if r.GetFunc != nil {
		return r.GetFunc(reqId)
	}
	return proto.Request{}, nil
}

func (r *RequestManager) Start(reqId string) error {
	if r.StartFunc != nil {
		return r.StartFunc(reqId)
	}
	return nil
}

func (r *RequestManager) Finish(reqId string, finishParams proto.FinishRequestParams) error {
	if r.FinishFunc != nil {
		return r.FinishFunc(reqId, finishParams)
	}
	return nil
}

func (r *RequestManager) Stop(reqId string) error {
	if r.StopFunc != nil {
		return r.StopFunc(reqId)
	}
	return nil
}

func (r *RequestManager) Status(reqId string) (proto.RequestStatus, error) {
	if r.StatusFunc != nil {
		return r.StatusFunc(reqId)
	}
	return proto.RequestStatus{}, nil
}

func (r *RequestManager) IncrementFinishedJobs(reqId string) error {
	if r.IncrementFinishedJobsFunc != nil {
		return r.IncrementFinishedJobsFunc(reqId)
	}
	return nil
}

func (r *RequestManager) Specs() []proto.RequestSpec {
	if r.SpecsFunc != nil {
		return r.SpecsFunc()
	}
	return []proto.RequestSpec{}
}

func (r *RequestManager) JobChain(reqId string) (proto.JobChain, error) {
	if r.JobChainFunc != nil {
		return r.JobChainFunc(reqId)
	}
	return proto.JobChain{}, nil
}
