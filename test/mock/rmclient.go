// Copyright 2017, Square, Inc.

package mock

import (
	"errors"
	"time"

	"github.com/square/spincycle/proto"
)

var (
	ErrRMClient = errors.New("forced error in rm client")
)

type RMClient struct {
	CreateRequestFunc  func(string, map[string]interface{}) (string, error)
	GetRequestFunc     func(string) (proto.Request, error)
	StartRequestFunc   func(string) error
	FinishRequestFunc  func(string, byte, time.Time) error
	StopRequestFunc    func(string) error
	SuspendRequestFunc func(string, proto.SuspendedJobChain) error
	RequestStatusFunc  func(string) (proto.RequestStatus, error)
	GetJobChainFunc    func(string) (proto.JobChain, error)
	GetJLFunc          func(string) ([]proto.JobLog, error)
	CreateJLFunc       func(string, proto.JobLog) error
	SysStatRunningFunc func() (proto.RunningStatus, error)
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

func (c *RMClient) FinishRequest(requestId string, state byte, finishedAt time.Time) error {
	if c.FinishRequestFunc != nil {
		return c.FinishRequestFunc(requestId, state, finishedAt)
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

func (c *RMClient) RequestStatus(requestId string) (proto.RequestStatus, error) {
	if c.RequestStatusFunc != nil {
		return c.RequestStatusFunc(requestId)
	}
	return proto.RequestStatus{}, nil
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
	return []proto.RequestSpec{}, nil
}

func (c *RMClient) SysStatRunning() (proto.RunningStatus, error) {
	if c.SysStatRunningFunc != nil {
		return c.SysStatRunningFunc()
	}
	return proto.RunningStatus{}, nil
}
