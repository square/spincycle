// Copyright 2017, Square, Inr.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrRequestManager = errors.New("forced error in request manager")
)

type RequestManager struct {
	CreateRequestFunc func(proto.CreateRequestParams) (proto.Request, error)
	GetRequestFunc    func(string) (proto.Request, error)
	StartRequestFunc  func(string) error
	FinishRequestFunc func(string, proto.FinishRequestParams) error
	StopRequestFunc   func(string) error
	RequestStatusFunc func(string) (proto.RequestStatus, error)
	GetJobChainFunc   func(string) (proto.JobChain, error)
	GetJLFunc         func(string, string) (proto.JobLog, error)
	GetFullJLFunc     func(string) ([]proto.JobLog, error)
	CreateJLFunc      func(string, proto.JobLog) (proto.JobLog, error)
	RequestListFunc   func() []proto.RequestSpec
}

func (r *RequestManager) CreateRequest(reqParams proto.CreateRequestParams) (proto.Request, error) {
	if r.CreateRequestFunc != nil {
		return r.CreateRequestFunc(reqParams)
	}
	return proto.Request{}, nil
}

func (r *RequestManager) GetRequest(reqId string) (proto.Request, error) {
	if r.GetRequestFunc != nil {
		return r.GetRequestFunc(reqId)
	}
	return proto.Request{}, nil
}

func (r *RequestManager) StartRequest(reqId string) error {
	if r.StartRequestFunc != nil {
		return r.StartRequestFunc(reqId)
	}
	return nil
}

func (r *RequestManager) FinishRequest(reqId string, finishParams proto.FinishRequestParams) error {
	if r.FinishRequestFunc != nil {
		return r.FinishRequestFunc(reqId, finishParams)
	}
	return nil
}

func (r *RequestManager) StopRequest(reqId string) error {
	if r.StopRequestFunc != nil {
		return r.StopRequestFunc(reqId)
	}
	return nil
}

func (r *RequestManager) RequestStatus(reqId string) (proto.RequestStatus, error) {
	if r.RequestStatusFunc != nil {
		return r.RequestStatusFunc(reqId)
	}
	return proto.RequestStatus{}, nil
}

func (r *RequestManager) GetJobChain(reqId string) (proto.JobChain, error) {
	if r.GetJobChainFunc != nil {
		return r.GetJobChainFunc(reqId)
	}
	return proto.JobChain{}, nil
}

func (r *RequestManager) GetJL(reqId, jobId string) (proto.JobLog, error) {
	if r.GetJLFunc != nil {
		return r.GetJLFunc(reqId, jobId)
	}
	return proto.JobLog{}, nil
}

func (r *RequestManager) GetFullJL(reqId string) ([]proto.JobLog, error) {
	if r.GetFullJLFunc != nil {
		return r.GetFullJLFunc(reqId)
	}
	return []proto.JobLog{}, nil
}

func (r *RequestManager) CreateJL(reqId string, jl proto.JobLog) (proto.JobLog, error) {
	if r.CreateJLFunc != nil {
		return r.CreateJLFunc(reqId, jl)
	}
	return proto.JobLog{}, nil
}

func (r *RequestManager) RequestList() []proto.RequestSpec {
	if r.RequestListFunc != nil {
		return r.RequestListFunc()
	}
	return []proto.RequestSpec{}
}
