// Copyright 2017, Square, Inc.

// Package request provides an interface for managing requests, which are the
// core compontent of the Request Manager.
package request

import (
	"time"

	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/util"
)

// A Manager is used to create and manage requests.
type Manager interface {
	// CreateRequest creates a proto.Request from a proto.CreateRequestParams. It
	// uses the Grapher package to create a job chain which it attaches to the
	// request proto.
	CreateRequest(proto.CreateRequestParams) (proto.Request, error)

	// GetRequest retrieves the request corresponding to the provided request id.
	GetRequest(requestId string) (proto.Request, error)

	// StartRequest starts a request (sends it to the Job Runner).
	StartRequest(requestId string) error

	// FinishRequest marks a request as being finished. It gets the request's final
	// state from the proto.FinishRequestParams argument.
	FinishRequest(requestId string, finishParams proto.FinishRequestParams) error

	// StopRequest tells the Job Runner to stop running the request's job chain.
	StopRequest(requestId string) error

	// RequestStatus returns the status of a request and all of the jobs in it.
	// The live status output of any jobs that are currently running will be
	// included as well.
	RequestStatus(requestId string) (proto.RequestStatus, error)

	// GetJobChain retrieves the job chain corresponding to the provided request id.
	GetJobChain(requestId string) (proto.JobChain, error)

	// GetJL retrieves the job log corresponding to the provided request and job ids.
	GetJL(requestId string, jobId string) (proto.JobLog, error)

	// CreateJL takes a request id and a proto.JobLog and creates a
	// proto.JobLog. This could be a no-op, or, depending on implementation, it
	// could update some of the fields on the JL, save it to a database, etc.
	CreateJL(requestId string, jl proto.JobLog) (proto.JobLog, error)
}

// Create an interface that we can mock out during testing.
type Grapher interface {
	CreateGraph(string, map[string]interface{}) (*grapher.Graph, error)
}

type manager struct {
	// Resolves requests into graphs.
	rr Grapher

	// Persists requests in a database.
	db DBAccessor

	// A client for communicating with the Job Runner (JR).
	jrc jr.Client
}

func NewManager(requestResolver Grapher, dbAccessor DBAccessor, jrClient jr.Client) Manager {
	return &manager{
		rr:  requestResolver,
		db:  dbAccessor,
		jrc: jrClient,
	}
}

// CreateRequest creates a proto.Request and persists it (along with it's job chain
// and the request params) in the database.
func (m *manager) CreateRequest(reqParams proto.CreateRequestParams) (proto.Request, error) {
	var req proto.Request
	if reqParams.Type == "" {
		return req, ErrInvalidParams
	}

	// Generate a UUID for the request.
	reqUuid := util.UUID()

	// Make a copy of args so that the request resolver doesn't modify our version.
	args := map[string]interface{}{}
	for k, v := range reqParams.Args {
		args[k] = v
	}

	// Resolve the request into a graph.
	g, err := m.rr.CreateGraph(reqParams.Type, args)
	if err != nil {
		return req, err
	}

	// Convert the graph into a proto.JobChain.
	jc := &proto.JobChain{
		Jobs:          map[string]proto.Job{},
		AdjacencyList: g.Edges,
	}
	for name, node := range g.Vertices {
		bytes, err := node.Datum.Serialize()
		if err != nil {
			return req, err
		}
		job := proto.Job{
			Type:      node.Datum.Type(),
			Id:        node.Datum.Name(),
			Bytes:     bytes,
			Retry:     node.Retry,
			RetryWait: node.RetryWait,
		}
		jc.Jobs[name] = job
	}
	jc.State = proto.STATE_PENDING
	jc.RequestId = reqUuid

	// Create a proto.Request.
	req = proto.Request{
		Id:        reqUuid,
		Type:      reqParams.Type,
		CreatedAt: time.Now(),
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
		JobChain:  jc,
		TotalJobs: len(jc.Jobs),
	}

	return req, m.db.SaveRequest(req, reqParams)
}

func (m *manager) GetRequest(requestId string) (proto.Request, error) {
	return m.db.GetRequest(requestId)
}

func (m *manager) StartRequest(requestId string) error {
	// Get the request from the db.
	req, err := m.db.GetRequest(requestId)
	if err != nil {
		return err
	}

	// Return an error unless the request is in the pending state, which prevents
	// us from starting a request which should not be able to be started.
	if req.State != proto.STATE_PENDING {
		return NewErrInvalidState(proto.StateName[proto.STATE_PENDING], proto.StateName[req.State])
	}

	// Get the request's job chain from the db.
	jc, err := m.db.GetJobChain(requestId)
	if err != nil {
		return err
	}

	// Update the request.
	now := time.Now()
	req.StartedAt = &now
	req.State = proto.STATE_RUNNING
	if err = m.db.UpdateRequest(req); err != nil {
		return err
	}

	// Send the request's job chain to the job runner, which will start running it.
	err = m.jrc.NewJobChain(jc)
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) FinishRequest(requestId string, finishParams proto.FinishRequestParams) error {
	// Get the request from the db.
	req, err := m.db.GetRequest(requestId)
	if err != nil {
		return err
	}

	// Return an error unless the request is in the running state, which prevents
	// us from finishing a request which should not be able to be finished.
	if req.State != proto.STATE_RUNNING {
		return NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[req.State])
	}

	// Update the request.
	now := time.Now()
	req.FinishedAt = &now
	req.State = finishParams.State
	if err = m.db.UpdateRequest(req); err != nil {
		return err
	}

	return nil
}

func (m *manager) StopRequest(requestId string) error {
	// Get the request from the db.
	req, err := m.db.GetRequest(requestId)
	if err != nil {
		return err
	}

	// Return an error unless the request is in the running state, which prevents
	// us from finishing a request which should not be able to be finished.
	if req.State != proto.STATE_RUNNING {
		return NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[req.State])
	}

	// Tell the JR to stop running the job chain for the request.
	err = m.jrc.StopRequest(requestId)
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) RequestStatus(requestId string) (proto.RequestStatus, error) {
	var reqStatus proto.RequestStatus

	// Get the request from the db.
	req, err := m.db.GetRequest(requestId)
	if err != nil {
		return reqStatus, err
	}
	reqStatus.Request = req

	// Get the job chain from the db.
	jc, err := m.db.GetJobChain(requestId)
	if err != nil {
		return reqStatus, err
	}

	// If the request is running, get the chain's live status from the job runner.
	var liveStatuses proto.JobStatuses
	if req.State == proto.STATE_RUNNING {
		liveStatus, err := m.jrc.RequestStatus(req.Id)
		if err != nil {
			return reqStatus, err
		}
		liveStatuses = liveStatus.JobStatuses
	}

	// Get the status of all finished jobs in the request.
	finishedStatuses, err := m.db.GetRequestJobStatuses(req.Id)
	if err != nil {
		return reqStatus, err
	}

	// Convert liveStatuses and finishedStatuses into maps of jobId => status
	// so that it is easy to lookup a job's status by it's id (used below).
	liveJobs := map[string]proto.JobStatus{}
	for _, status := range liveStatuses {
		liveJobs[status.Id] = status
	}
	finishedJobs := map[string]proto.JobStatus{}
	for _, status := range finishedStatuses {
		finishedJobs[status.Id] = status
	}

	// For each job in the job chain, get the job's status from either
	// liveJobs or finishedJobs. Since the way we collect these maps is not
	// transactional (we get liveJobs before finishedJobs), there can
	// potentially be outdated info in liveJobs. Therefore, statuses in
	// finishedJobs take priority over statuses in liveJobs. If a job does
	// not exist in either map, it must be pending.
	allStatuses := proto.JobStatuses{}
	for _, job := range jc.Jobs {
		if status, ok := finishedJobs[job.Id]; ok {
			allStatuses = append(allStatuses, status)
		} else if status, ok := liveJobs[job.Id]; ok {
			allStatuses = append(allStatuses, status)
		} else {
			status := proto.JobStatus{
				Id:    job.Id,
				State: proto.STATE_PENDING,
			}
			allStatuses = append(allStatuses, status)
		}
	}

	reqStatus.JobChainStatus = proto.JobChainStatus{
		RequestId:   req.Id,
		JobStatuses: allStatuses,
	}

	return reqStatus, nil
}

func (m *manager) GetJobChain(requestId string) (proto.JobChain, error) {
	return m.db.GetJobChain(requestId)
}

func (m *manager) GetJL(requestId, jobId string) (proto.JobLog, error) {
	return m.db.GetLatestJL(requestId, jobId)
}

func (m *manager) CreateJL(requestId string, jl proto.JobLog) (proto.JobLog, error) {
	// Set the request id on the JL to be the request id argument.
	jl.RequestId = requestId

	err := m.db.SaveJL(jl)
	if err != nil {
		return jl, err
	}

	// Update the finished job count on the request if the job completed successfully.
	// This is so that, when you just get the high-lvel status of a request, you can
	// see what % of jobs are finished (as opposed to having to look at job logs,
	// querying the job runner, etc.).
	if jl.State == proto.STATE_COMPLETE {
		if err = m.db.IncrementRequestFinishedJobs(requestId); err != nil {
			return jl, err
		}
	}

	return jl, nil
}
