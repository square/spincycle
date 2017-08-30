// Copyright 2017, Square, Inc.

// Package status provides system-wide status.
package status

import (
	"time"

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
		for jobId, j := range c.Running {
			startTime := time.Unix(0, j.StartTs)
			s := proto.JobStatus{
				RequestId: c.RequestId(),
				JobId:     jobId,
				State:     proto.STATE_RUNNING, // must be since it's in chain.Running
				Runtime:   time.Now().Sub(startTime).Seconds(),
				N:         j.N,
			}
			running = append(running, s)
		}
	}

	return running, nil
}
