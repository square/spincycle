// Copyright 2017-2018, Square, Inc.

package mock

import (
	"errors"
	"net/http"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/auth"
)

var (
	ErrRequestManager = errors.New("forced error in request manager")
	ErrRequestResumer = errors.New("forced error in request resumer")
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

// --------------------------------------------------------------------------

type RequestResumer struct {
	ResumeAllFunc func()
	CleanupFunc   func()
	ResumeFunc    func(string) error
	SuspendFunc   func(proto.SuspendedJobChain) error
}

func (r *RequestResumer) ResumeAll() {
	if r.ResumeAllFunc != nil {
		r.ResumeAllFunc()
	}
	return
}

func (r *RequestResumer) Cleanup() {
	if r.CleanupFunc != nil {
		r.CleanupFunc()
	}
	return
}

func (r *RequestResumer) Resume(id string) error {
	if r.ResumeFunc != nil {
		return r.ResumeFunc(id)
	}
	return nil
}

func (r *RequestResumer) Suspend(sjc proto.SuspendedJobChain) error {
	if r.SuspendFunc != nil {
		return r.SuspendFunc(sjc)
	}
	return nil
}

// --------------------------------------------------------------------------

type AuthPlugin struct {
	AuthenticateFunc func(*http.Request) (auth.Caller, error)
	AuthorizeFunc    func(c auth.Caller, op string, req proto.Request) error
}

func (a AuthPlugin) Authenticate(req *http.Request) (auth.Caller, error) {
	if a.AuthenticateFunc != nil {
		return a.AuthenticateFunc(req)
	}
	return auth.Caller{}, nil
}

func (a AuthPlugin) Authorize(c auth.Caller, op string, req proto.Request) error {
	if a.AuthorizeFunc != nil {
		return a.AuthorizeFunc(c, op, req)
	}
	return nil
}
