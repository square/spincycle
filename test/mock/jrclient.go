// Copyright 2017, Square, Inc.

package mock

import (
	"errors"
	"net/url"

	"github.com/square/spincycle/proto"
)

var (
	ErrJRClient = errors.New("forced error in jr client")
)

type JRClient struct {
	NewJobChainFunc    func(string, proto.JobChain) (*url.URL, error)
	ResumeJobChainFunc func(string, proto.SuspendedJobChain) (*url.URL, error)
	StartRequestFunc   func(string, string) error
	StopRequestFunc    func(string, string) error
	RequestStatusFunc  func(string, string) (proto.JobChainStatus, error)
	SysStatRunningFunc func(string) ([]proto.JobStatus, error)
}

func (c *JRClient) NewJobChain(baseURL string, jc proto.JobChain) (*url.URL, error) {
	if c.NewJobChainFunc != nil {
		return c.NewJobChainFunc(baseURL, jc)
	}
	return nil, nil
}

func (c *JRClient) ResumeJobChain(baseURL string, sjc proto.SuspendedJobChain) (*url.URL, error) {
	if c.ResumeJobChainFunc != nil {
		return c.ResumeJobChainFunc(baseURL, sjc)
	}
	return nil, nil
}

func (c *JRClient) StartRequest(baseURL string, requestId string) error {
	if c.StartRequestFunc != nil {
		return c.StartRequestFunc(baseURL, requestId)
	}
	return nil
}

func (c *JRClient) StopRequest(baseURL string, requestId string) error {
	if c.StopRequestFunc != nil {
		return c.StopRequestFunc(baseURL, requestId)
	}
	return nil
}

func (c *JRClient) RequestStatus(baseURL string, requestId string) (proto.JobChainStatus, error) {
	if c.RequestStatusFunc != nil {
		return c.RequestStatusFunc(baseURL, requestId)
	}
	return proto.JobChainStatus{}, nil
}

func (c *JRClient) SysStatRunning(baseURL string) ([]proto.JobStatus, error) {
	if c.SysStatRunningFunc != nil {
		return c.SysStatRunningFunc(baseURL)
	}
	return []proto.JobStatus{}, nil
}
