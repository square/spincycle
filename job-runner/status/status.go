// Copyright 2017-2019, Square, Inc.

// Package status provides system-wide status.
package status

import (
	"github.com/orcaman/concurrent-map"
	log "github.com/sirupsen/logrus"

	serr "github.com/square/spincycle/errors"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager"
)

type Manager interface {
	Running(proto.StatusFilter) ([]proto.JobStatus, error)
}

type manager struct {
	traverserRepo cmap.ConcurrentMap
}

func NewManager(traverserRepo cmap.ConcurrentMap) *manager {
	m := &manager{
		traverserRepo: traverserRepo,
	}
	return m
}

func (m *manager) Running(f proto.StatusFilter) ([]proto.JobStatus, error) {
	var traversers []chain.Traverser

	// If filter by request ID, get only that request; else, get all
	if f.RequestId == "" {
		items := m.traverserRepo.Items() // returns map[reqId]interface{}
		traversers = make([]chain.Traverser, 0, len(items))
		for _, v := range items {
			traversers = append(traversers, v.(chain.Traverser))
		}
	} else {
		v, ok := m.traverserRepo.Get(f.RequestId) // returns interface{}
		if !ok {
			return nil, serr.RequestNotFound{f.RequestId}
		}
		traversers = []chain.Traverser{v.(chain.Traverser)}
	}

	// Get currently running jobs in each traverser/chain
	running := []proto.JobStatus{}
	for _, tr := range traversers {
		status := tr.Running()
		running = append(running, status...)
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
