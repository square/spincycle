// Copyright 2017, Square, Inc.

// Package status provides system-wide status.
package status

import (
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
)

type Manager interface {
	Running() ([]proto.JobStatus, error)
}

type manager struct {
	cr chain.Repo
}

func NewManager(cr chain.Repo) *manager {
	m := &manager{
		cr: cr,
	}
	return m
}

func (m *manager) Running() ([]proto.JobStatus, error) {
	chains, err := m.cr.GetAll()
	if err != nil {
		return nil, err
	}

	running := []proto.JobStatus{}
	for _, c := range chains {
		for _, jobStatus := range c.Running() {
			running = append(running, jobStatus)
		}
	}

	return running, nil
}
