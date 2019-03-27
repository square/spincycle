// Copyright 2017-2019, Square, Inc.

// Package status provides system-wide status.
package status

import (
	log "github.com/Sirupsen/logrus"

	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager"
)

type Manager interface {
	Running(proto.StatusFilter) ([]proto.JobStatus, error)
}

type manager struct {
	cr chain.Repo
	rr runner.Repo
}

func NewManager(cr chain.Repo, rr runner.Repo) *manager {
	m := &manager{
		cr: cr,
		rr: rr,
	}
	return m
}

func (m *manager) Running(f proto.StatusFilter) ([]proto.JobStatus, error) {
	var chains []*chain.Chain
	var err error

	// If filter by request ID, get only that request's chain; else, get all chains
	if f.RequestId == "" {
		chains, err = m.cr.GetAll()
		if err != nil {
			return nil, err
		}
	} else {
		c, err := m.cr.Get(f.RequestId)
		if err != nil {
			return nil, err
		}
		chains = []*chain.Chain{c}
	}

	// Get currently running jobs in each chain
	running := []proto.JobStatus{}
	for _, c := range chains {
		for jobId, jobStatus := range c.Running() {
			r := m.rr.Get(jobId)
			if r == nil {
				continue // job completed, ignore it
			}
			// Get real-time job status and current try
			jobStatus.Try, jobStatus.Status = r.Status()
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
