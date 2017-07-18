// Copyright 2017, Square, Inc.

// Package request provides an interface for managing requests, which are the
// core compontent of the Request Manager.
package request

import (
	"encoding/json"
	"fmt"
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

	// RequestStatus returns the status of a request. If the request is running, or
	// if it has already finished and some jobs failed, the proto.RequestStatus
	// return struct will include the status of all "burning-edge" jobs. From the
	// burning-edge jobs, one can infer the status of all other jobs (all ancestors
	// of a burning-edge job must be complete, and all children of a burning-edge
	// job must be pending). If the request hasn't started, or if it is finished
	// and all jobs in it completed, the return struct won't include the status of
	// any jobs (since they can be inferred to be pending or completed).
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
	jr jr.Client
}

func NewManager(requestResolver Grapher, dbAccessor DBAccessor, jrClient jr.Client) Manager {
	return &manager{
		rr: requestResolver,
		db: dbAccessor,
		jr: jrClient,
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
			Type:  node.Datum.Type(),
			Id:    node.Datum.Name(),
			Bytes: bytes,
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

	// Marshal the the job chain and request params.
	rawJc, err := json.Marshal(req.JobChain)
	if err != nil {
		return req, fmt.Errorf("cannot marshal job chain: %s", err)
	}
	rawParams, err := json.Marshal(reqParams)
	if err != nil {
		return req, fmt.Errorf("cannot marshal request params: %s", err)
	}

	return req, m.db.SaveRequest(req, rawParams, rawJc)
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
	err = m.jr.NewJobChain(jc)
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
	err = m.jr.StopRequest(requestId)
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

	// Here we get the status of jobs in the request. Since there may be a large
	// number of jobs in the request, the goal is to return the status of the
	// fewest number of jobs as possible (while still providing enough info
	// for the caller to be able to infer the status of all other jobs).
	// Therefore, we only return the status of jobs that are running or have
	// failed. We say that these jobs are at the "burning edges" of the chain.
	//
	// Based on the state of the request, there are four different scenarios
	// that we can encounter:
	//   1) If the request hasn't started, we don't need to return the status of
	//      any jobs because they must all be pending.
	//   2) If the request is complete, we don't need to return the status of
	//      any jobs because they must all be complete.
	//   3) If the request is running, we return the status of all burning-edge
	//      jobs (these jobs could be running, completed, or failed). Any
	//      ancestors of these jobs must be complete, and any children of them
	//      must be pending.
	//   4) If the request is incomplete (i.e., it finished, but some jobs
	//      failed), we return the status of all burning-edge jobs (these jobs
	//      will all be in the failed state). Any ancestors of these jobs must
	//      be complete, and any children of them must be pending.
	//
	// Scenarios 3 and 4 are the only two that require us to get job statuses,
	// and scenario 3 is the only one that requires us to reach out to the JR
	// to get the status of live jobs.
	if req.State == proto.STATE_RUNNING || req.State == proto.STATE_INCOMPLETE {
		// If the request is running, get the chain's live status from
		// the job runner.
		var liveStatuses proto.JobStatuses
		if req.State == proto.STATE_RUNNING {
			liveStatus, err := m.jr.RequestStatus(req.Id)
			if err != nil {
				return reqStatus, err
			}
			liveStatuses = liveStatus.JobStatuses
		}

		// Create a map of job id => nothing for all jobs in liveStatuses.
		// We will use this below so that we can easily see if a given job
		// id exists in liveStatuses (i.e., if a given job is running).
		runningJobs := map[string]struct{}{}
		for _, status := range liveStatuses {
			runningJobs[status.Id] = struct{}{}
		}

		// Get a list of all the finished job ids in this request, and then
		// convert the finishedJobIds slice to a map so that it's easier
		// to see if a given job id is in the list of finished ids.
		finishedJobIds, err := m.db.GetRequestFinishedJobIds(req.Id)
		if err != nil {
			return reqStatus, err
		}
		finishedJobs := map[string]struct{}{}
		for _, id := range finishedJobIds {
			finishedJobs[id] = struct{}{}
		}

		// Get the job chain from the db.
		jc, err := m.db.GetJobChain(requestId)
		if err != nil {
			return reqStatus, err
		}

		// Figure out which finished jobs are at the burning edges of the
		// chain. A finished job is only a burning edge if none of its
		// adjacent jobs are in the finishedJobs or runningJobs maps.
		var finishedBurningEdgeIds []string
		for id := range finishedJobs {
			isBurningEdge := true
			for _, adjId := range jc.AdjacencyList[id] {
				if _, ok := finishedJobs[adjId]; ok {
					isBurningEdge = false
					break
				}
				if _, ok := runningJobs[adjId]; ok {
					isBurningEdge = false
					break
				}
			}

			if isBurningEdge {
				finishedBurningEdgeIds = append(finishedBurningEdgeIds, id)
			}
		}

		// Get the status of all of the finished burning edges.
		finishedStatuses, err := m.db.GetRequestJobStatuses(req.Id, finishedBurningEdgeIds)
		if err != nil {
			return reqStatus, err
		}

		// Lastly, we need to combine finishedStatuses (statuses from
		// burning-edge jobs that have already finished) with liveStatuses
		// (statuses from live running jobs). Since the way we get these
		// slices is not transactional (first we get liveStatuses, and then
		// we get statuses), there could potentially be outdated info in
		// liveStatuses. Therefore, when constructing our combined status
		// slice, we only include liveStatuses for job ids that do not
		// exist in finishedJobs.
		allStatuses := finishedStatuses
		for _, status := range liveStatuses {
			if _, ok := finishedJobs[status.Id]; !ok {
				allStatuses = append(allStatuses, status)
			}
		}

		reqStatus.JobChainStatus = proto.JobChainStatus{
			RequestId:   req.Id,
			JobStatuses: allStatuses,
		}
	}

	return reqStatus, nil
}

func (m *manager) GetJobChain(requestId string) (proto.JobChain, error) {
	return m.db.GetJobChain(requestId)
}

func (m *manager) GetJL(requestId, jobId string) (proto.JobLog, error) {
	return m.db.GetJL(requestId, jobId)
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
