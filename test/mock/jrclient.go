// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrJRClient = errors.New("forced error in jr client")
)

type JRClient struct {
	NewJobChainFunc    func(proto.JobChain) (string, error)
	ResumeJobChainFunc func(proto.SuspendedJobChain) (string, error)
	StartRequestFunc   func(string) error
	StopRequestFunc    func(string) error
	RequestStatusFunc  func(string) (proto.JobChainStatus, error)
	SysStatRunningFunc func() ([]proto.JobStatus, error)
}

func (c *JRClient) NewJobChain(jc proto.JobChain) (string, error) {
	if c.NewJobChainFunc != nil {
		return c.NewJobChainFunc(jc)
	}
	return "", nil
}

func (c *JRClient) ResumeJobChain(sjc proto.SuspendedJobChain) (string, error) {
	if c.ResumeJobChainFunc != nil {
		return c.ResumeJobChainFunc(sjc)
	}
	return "", nil
}

func (c *JRClient) StartRequest(requestId string) error {
	if c.StartRequestFunc != nil {
		return c.StartRequestFunc(requestId)
	}
	return nil
}

func (c *JRClient) StopRequest(requestId string) error {
	if c.StopRequestFunc != nil {
		return c.StopRequestFunc(requestId)
	}
	return nil
}

func (c *JRClient) RequestStatus(requestId string) (proto.JobChainStatus, error) {
	if c.RequestStatusFunc != nil {
		return c.RequestStatusFunc(requestId)
	}
	return proto.JobChainStatus{}, nil
}

func (c *JRClient) SysStatRunning() ([]proto.JobStatus, error) {
	if c.SysStatRunningFunc != nil {
		return c.SysStatRunningFunc()
	}
	return []proto.JobStatus{}, nil
}
