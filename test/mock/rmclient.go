// Copyright 2017-2019, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrRMClient = errors.New("forced error in rm client")
)

type RMClient struct {
	CreateRequestFunc  func(string, map[string]interface{}) (string, error)
	GetRequestFunc     func(string) (proto.Request, error)
	StartRequestFunc   func(string) error
	FinishRequestFunc  func(proto.FinishRequest) error
	StopRequestFunc    func(string) error
	SuspendRequestFunc func(string, proto.SuspendedJobChain) error
	GetJobChainFunc    func(string) (proto.JobChain, error)
	GetJLFunc          func(string) ([]proto.JobLog, error)
	CreateJLFunc       func(string, proto.JobLog) error
	RunningFunc        func(proto.StatusFilter) (proto.RunningStatus, error)
	RequestListFunc    func() ([]proto.RequestSpec, error)
	UpdateProgressFunc func(proto.RequestProgress) error
}

func (c *RMClient) CreateRequest(requestId string, args map[string]interface{}) (string, error) {
	if c.CreateRequestFunc != nil {
		return c.CreateRequestFunc(requestId, args)
	}
	return "", nil
}

func (c *RMClient) GetRequest(requestId string) (proto.Request, error) {
	if c.GetRequestFunc != nil {
		return c.GetRequestFunc(requestId)
	}
	return proto.Request{}, nil
}

func (c *RMClient) StartRequest(requestId string) error {
	if c.StartRequestFunc != nil {
		return c.StartRequestFunc(requestId)
	}
	return nil
}

func (c *RMClient) FinishRequest(fr proto.FinishRequest) error {
	if c.FinishRequestFunc != nil {
		return c.FinishRequestFunc(fr)
	}
	return nil
}

func (c *RMClient) StopRequest(requestId string) error {
	if c.StopRequestFunc != nil {
		return c.StopRequestFunc(requestId)
	}
	return nil
}

func (c *RMClient) SuspendRequest(requestId string, sjc proto.SuspendedJobChain) error {
	if c.SuspendRequestFunc != nil {
		return c.SuspendRequestFunc(requestId, sjc)
	}
	return nil
}

func (c *RMClient) GetJobChain(requestId string) (proto.JobChain, error) {
	if c.GetJobChainFunc != nil {
		return c.GetJobChainFunc(requestId)
	}
	return proto.JobChain{}, nil
}

func (c *RMClient) GetJL(requestId string) ([]proto.JobLog, error) {
	if c.GetJLFunc != nil {
		return c.GetJLFunc(requestId)
	}
	return []proto.JobLog{}, nil
}

func (c *RMClient) CreateJL(requestId string, jl proto.JobLog) error {
	if c.CreateJLFunc != nil {
		return c.CreateJLFunc(requestId, jl)
	}
	return nil
}

func (c *RMClient) RequestList() ([]proto.RequestSpec, error) {
	if c.RequestListFunc != nil {
		return c.RequestListFunc()
	}
	return []proto.RequestSpec{}, nil
}

func (c *RMClient) Running(f proto.StatusFilter) (proto.RunningStatus, error) {
	if c.RunningFunc != nil {
		return c.RunningFunc(f)
	}
	return proto.RunningStatus{}, nil
}

func (c *RMClient) UpdateProgress(prg proto.RequestProgress) error {
	if c.RequestListFunc != nil {
		return c.UpdateProgressFunc(prg)
	}
	return nil
}
