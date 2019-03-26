// Copyright 2017-2019, Square, Inc.

// Package status provides system-wide status.
package status

import (
	log "github.com/Sirupsen/logrus"

	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager"
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

// --------------------------------------------------------------------------

// FinishedJobs sends updated finished jobs counts to the Request Manager.
// This is a singleton service that's ran in Server.Run(). Updates are best-effort.
// The final finished jobs count for a chain is sent with FinishRequest in
// a reaper when the chain is done.
type FinishedJobs struct {
	ChainRepo chain.Repo
	RMC       rm.Client
}

func (f FinishedJobs) Update() {
	chains, err := f.ChainRepo.GetAll()
	if err != nil {
		log.Warnf("FinishedJobs.Update: ChainRepo.GetAll: %s", err)
		return
	}
	for _, chain := range chains {
		prg := proto.RequestProgress{
			RequestId:    chain.RequestId(),
			FinishedJobs: chain.FinishedJobs(),
		}
		if err := f.RMC.UpdateProgress(prg); err != nil {
			log.Warnf("FinishedJobs.Update: UpdateProgress: %s", err)
		}
	}
}
