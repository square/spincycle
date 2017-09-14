// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrRequestDBAccessor = errors.New("forced error in request dbaccessor")
)

type RequestDBAccessor struct {
	SaveRequestFunc                  func(proto.Request, proto.CreateRequestParams) error
	GetRequestFunc                   func(string) (proto.Request, error)
	UpdateRequestFunc                func(proto.Request) error
	IncrementRequestFinishedJobsFunc func(string) error
	GetRequestJobStatusesFunc        func(string) (proto.JobStatuses, error)
	GetJobChainFunc                  func(string) (proto.JobChain, error)
	GetLatestJLFunc                  func(string, string) (proto.JobLog, error)
	GetFullJLFunc                    func(string) ([]proto.JobLog, error)
	SaveJLFunc                       func(proto.JobLog) error
}

func (a *RequestDBAccessor) SaveRequest(req proto.Request, reqParams proto.CreateRequestParams) error {
	if a.SaveRequestFunc != nil {
		return a.SaveRequestFunc(req, reqParams)
	}
	return nil
}

func (a *RequestDBAccessor) GetRequest(requestId string) (proto.Request, error) {
	if a.GetRequestFunc != nil {
		return a.GetRequestFunc(requestId)
	}
	return proto.Request{}, nil
}

func (a *RequestDBAccessor) GetJobChain(requestId string) (proto.JobChain, error) {
	if a.GetJobChainFunc != nil {
		return a.GetJobChainFunc(requestId)
	}
	return proto.JobChain{}, nil
}

func (a *RequestDBAccessor) UpdateRequest(req proto.Request) error {
	if a.UpdateRequestFunc != nil {
		return a.UpdateRequestFunc(req)
	}
	return nil
}

func (a *RequestDBAccessor) IncrementRequestFinishedJobs(requestId string) error {
	if a.IncrementRequestFinishedJobsFunc != nil {
		return a.IncrementRequestFinishedJobsFunc(requestId)
	}
	return nil
}

func (a *RequestDBAccessor) GetRequestJobStatuses(requestId string) (proto.JobStatuses, error) {
	if a.GetRequestJobStatusesFunc != nil {
		return a.GetRequestJobStatusesFunc(requestId)
	}
	return proto.JobStatuses{}, nil
}

func (a *RequestDBAccessor) GetLatestJL(requestId, jobId string) (proto.JobLog, error) {
	if a.GetLatestJLFunc != nil {
		return a.GetLatestJLFunc(requestId, jobId)
	}
	return proto.JobLog{}, nil
}

func (a *RequestDBAccessor) GetFullJL(requestId string) ([]proto.JobLog, error) {
	if a.GetFullJLFunc != nil {
		return a.GetFullJLFunc(requestId)
	}
	return []proto.JobLog{}, nil
}

func (a *RequestDBAccessor) SaveJL(jl proto.JobLog) error {
	if a.SaveJLFunc != nil {
		return a.SaveJLFunc(jl)
	}
	return nil
}
