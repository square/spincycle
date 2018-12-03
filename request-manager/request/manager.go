// Copyright 2017-2018, Square, Inc.

// Package request provides an interface for managing requests.
package request

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	myconn "github.com/go-mysql/conn"
	"github.com/go-sql-driver/mysql"

	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/util"
)

// A Manager is used to create and manage requests.
type Manager interface {
	// Create creates a proto.Request and saves it to the db.
	Create(proto.CreateRequestParams) (proto.Request, error)

	// Get retrieves the request corresponding to the provided id,
	// without its job chain.
	Get(requestId string) (proto.Request, error)

	// Start starts a request (sends it to the JR).
	Start(requestId string) error

	// Stop stops a request (sends a stop signal to the JR).
	Stop(requestId string) error

	// Status returns the status of a request and all of the jobs in it.
	// The live status output of any jobs that are currently running will be
	// included as well.
	Status(requestId string) (proto.RequestStatus, error)

	// Finish marks a request as being finished. It gets the request's final
	// state from the proto.FinishRequestParams argument.
	Finish(requestId string, finishParams proto.FinishRequestParams) error

	// IncrementFinishedJobs increments the count of the FinishedJobs field
	// on the request and saves it to the db.
	IncrementFinishedJobs(requestId string) error

	// Specs returns a list of all the request specs the the RM knows about.
	Specs() []proto.RequestSpec

	// JobChain returns the job chain for the given request id.
	JobChain(requestId string) (proto.JobChain, error)
}

// manager implements the Manager interface.
type manager struct {
	grf          grapher.GrapherFactory
	dbc          myconn.Connector
	jrc          jr.Client
	shutdownChan chan struct{}
	*sync.Mutex
}

func NewManager(grf grapher.GrapherFactory, dbc myconn.Connector, jrClient jr.Client, shutdownChan chan struct{}) Manager {
	return &manager{
		grf:          grf,
		dbc:          dbc,
		jrc:          jrClient,
		shutdownChan: shutdownChan,
		Mutex:        &sync.Mutex{},
	}
}

func (m *manager) Create(reqParams proto.CreateRequestParams) (proto.Request, error) {
	var req proto.Request
	if reqParams.Type == "" {
		return req, ErrInvalidParams
	}

	reqIdBytes := util.XID()
	reqId := reqIdBytes.String()
	req = proto.Request{
		Id:        reqId,
		Type:      reqParams.Type,
		CreatedAt: time.Now(),
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
	}

	// Make a copy of args so that the request resolver doesn't modify reqParams.
	args := map[string]interface{}{}
	for k, v := range reqParams.Args {
		args[k] = v
	}

	// Resolve the request into a graph, and convert to a proto.JobChain.
	gr := m.grf.Make()
	g, err := gr.CreateGraph(reqParams.Type, args)
	if err != nil {
		return req, err
	}

	jc := &proto.JobChain{
		Jobs:          map[string]proto.Job{},
		AdjacencyList: g.Edges,
	}
	for jobId, node := range g.Vertices {
		bytes, err := node.Datum.Serialize()
		if err != nil {
			return req, err
		}
		job := proto.Job{
			Type:          node.Datum.Id().Type,
			Id:            node.Datum.Id().Id,
			Name:          node.Datum.Id().Name,
			Bytes:         bytes,
			Args:          node.Args,
			Retry:         node.Retry,
			RetryWait:     node.RetryWait,
			SequenceId:    node.SequenceId,
			SequenceRetry: node.SequenceRetry,
			State:         proto.STATE_PENDING,
		}
		jc.Jobs[jobId] = job
	}
	jc.State = proto.STATE_PENDING
	jc.RequestId = reqId
	req.JobChain = jc
	req.TotalJobs = len(jc.Jobs)

	// Marshal the job chain and request params.
	rawJc, err := json.Marshal(req.JobChain)
	if err != nil {
		return req, fmt.Errorf("cannot marshal job chain: %s", err)
	}
	rawParams, err := json.Marshal(reqParams)
	if err != nil {
		return req, fmt.Errorf("cannot marshal request params: %s", err)
	}

	// Connect to database
	ctx := context.TODO()
	conn, err := m.dbc.Open(ctx)
	if err != nil {
		return req, err
	}
	defer m.dbc.Close(conn) // don't leak conn

	// Begin a transaction to insert the request into the requests table, as
	// well as the jc and raw request params into the raw_requests table.
	txn, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return req, err
	}
	defer txn.Rollback()

	q := "INSERT INTO requests (request_id, type, state, user, created_at, total_jobs) VALUES (?, ?, ?, ?, ?, ?)"
	_, err = txn.ExecContext(ctx, q,
		reqIdBytes,
		req.Type,
		req.State,
		req.User,
		req.CreatedAt,
		req.TotalJobs,
	)
	if err != nil {
		return req, err
	}

	q = "INSERT INTO raw_requests (request_id, request, job_chain) VALUES (?, ?, ?)"
	if _, err = txn.ExecContext(ctx, q,
		reqIdBytes,
		rawParams,
		rawJc); err != nil {
		return req, err
	}

	return req, txn.Commit()
}

// Retrieve the request without its corresponding Job Chain.
func (m *manager) Get(requestId string) (proto.Request, error) {
	var req proto.Request

	ctx := context.TODO()
	conn, err := m.dbc.Open(ctx)
	if err != nil {
		return req, err
	}
	defer m.dbc.Close(conn) // don't leak conn

	// Nullable columns.
	var user sql.NullString
	var jrHost sql.NullString
	startedAt := mysql.NullTime{}
	finishedAt := mysql.NullTime{}

	q := "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, " +
		"finished_jobs, jr_host FROM requests WHERE request_id = ?"
	err = conn.QueryRowContext(ctx, q, requestId).Scan(
		&req.Id,
		&req.Type,
		&req.State,
		&user,
		&req.CreatedAt,
		&startedAt,
		&finishedAt,
		&req.TotalJobs,
		&req.FinishedJobs,
		&jrHost)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			return req, db.NewErrNotFound("request")
		default:
			return req, err
		}
	}

	if user.Valid {
		req.User = user.String
	}
	if jrHost.Valid {
		req.JobRunnerHost = jrHost.String
	}
	if startedAt.Valid {
		req.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		req.FinishedAt = &finishedAt.Time
	}

	return req, nil
}

func (m *manager) Start(requestId string) error {
	req, err := m.getWithJc(requestId)
	if err != nil {
		return err
	}

	now := time.Now()

	// Only start the request if it's currently Pending.
	if req.State != proto.STATE_PENDING {
		return NewErrInvalidState(proto.StateName[proto.STATE_PENDING], proto.StateName[req.State])
	}

	// TODO(felixp): add retries to this call to the JR to start the job chain
	// Send the request's job chain to the job runner, which will start running it.
	host, err := m.jrc.NewJobChain(*req.JobChain)
	if err != nil {
		return err
	}

	req.StartedAt = &now
	req.State = proto.STATE_RUNNING
	req.JobRunnerHost = host

	// This will only update the request if the current state is PENDING. The
	// state should be PENDING since we checked this earlier, but it's possible
	// something else has changed the state since then.
	err = m.updateRequest(req, proto.STATE_PENDING)
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) Stop(requestId string) error {
	req, err := m.Get(requestId)
	if err != nil {
		return err
	}

	if req.State == proto.STATE_COMPLETE {
		return nil
	}

	// Return an error unless the request is in the running state, which prevents
	// us from stopping a request which should not be able to be stopped.
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

func (m *manager) Status(requestId string) (proto.RequestStatus, error) {
	var reqStatus proto.RequestStatus

	req, err := m.getWithJc(requestId)
	if err != nil {
		return reqStatus, err
	}
	reqStatus.Request = req

	// //////////////////////////////////////////////////////////////////////
	// Live jobs
	// //////////////////////////////////////////////////////////////////////

	// If the request is running, get the chain's live status from the job runner.
	var liveS proto.JobStatuses
	if req.State == proto.STATE_RUNNING {
		// TODO update to query the specific JR host that is running this request
		s, err := m.jrc.RequestStatus(req.Id)
		if err != nil {
			return reqStatus, err
		}
		liveS = s.JobStatuses
	}

	// //////////////////////////////////////////////////////////////////////
	// Finished jobs
	// //////////////////////////////////////////////////////////////////////

	// Get the status of all finished jobs from the db.
	ctx := context.TODO()
	conn, err := m.dbc.Open(ctx)
	if err != nil {
		return reqStatus, err
	}
	defer m.dbc.Close(conn) // don't leak conn

	// TODO(alyssa): change query when we add support for nested sequence retries
	q := "SELECT j1.job_id, j1.name, j1.state FROM job_log j1 LEFT JOIN job_log j2 ON (j1.request_id = " +
		"j2.request_id AND j1.job_id = j2.job_id AND j1.try < j2.try) WHERE j1.request_id = ? AND j2.try IS NULL"
	rows, err := conn.QueryContext(ctx, q, requestId)
	if err != nil {
		return reqStatus, err
	}
	defer rows.Close()

	var finishedS proto.JobStatuses
	for rows.Next() {
		var s proto.JobStatus
		if err := rows.Scan(&s.JobId, &s.Name, &s.State); err != nil {
			return reqStatus, err
		}
		s.RequestId = requestId

		finishedS = append(finishedS, s)
	}

	// //////////////////////////////////////////////////////////////////////
	// Combine live + finished
	// //////////////////////////////////////////////////////////////////////

	// Convert liveS and finishedS into maps of jobId => status so that it
	// is easy to lookup a job's status by its id (used below).
	liveJ := map[string]proto.JobStatus{}
	for _, s := range liveS {
		liveJ[s.JobId] = s
	}
	finishedJ := map[string]proto.JobStatus{}
	for _, s := range finishedS {
		finishedJ[s.JobId] = s
	}

	// For each job in the job chain, get the job's status from either
	// liveJobs or finishedJobs. Since the way we collect these maps is not
	// transactional (we get liveJobs before finishedJobs), there can
	// potentially be outdated info in liveJobs. Therefore, statuses in
	// finishedJobs take priority over statuses in liveJobs. If a job does
	// not exist in either map, it must be pending.
	allS := proto.JobStatuses{}
	for _, j := range req.JobChain.Jobs {
		if s, ok := finishedJ[j.Id]; ok {
			allS = append(allS, s)
		} else if s, ok := liveJ[j.Id]; ok {
			allS = append(allS, s)
		} else {
			s := proto.JobStatus{
				JobId: j.Id,
				Name:  j.Name,
				State: proto.STATE_PENDING,
			}
			allS = append(allS, s)
		}
	}

	reqStatus.JobChainStatus = proto.JobChainStatus{
		RequestId:   req.Id,
		JobStatuses: allS,
	}

	return reqStatus, nil
}

func (m *manager) Finish(requestId string, finishParams proto.FinishRequestParams) error {
	req, err := m.Get(requestId)
	if err != nil {
		return err
	}

	req.FinishedAt = &finishParams.FinishedAt
	prevState := req.State
	req.State = finishParams.State
	req.JobRunnerHost = ""

	// This will only update the request if the current state is RUNNING.
	err = m.updateRequest(req, proto.STATE_RUNNING)
	if err != nil {
		if prevState != proto.STATE_RUNNING {
			// This should never happen - we never finish a request that isn't running.
			return NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[prevState])
		}
		return err
	}

	return nil
}

func (m *manager) IncrementFinishedJobs(requestId string) error {
	ctx := context.TODO()
	conn, err := m.dbc.Open(ctx)
	if err != nil {
		return err
	}
	defer m.dbc.Close(conn) // don't leak conn

	q := "UPDATE requests SET finished_jobs = finished_jobs + 1 WHERE request_id = ?"
	res, err := conn.ExecContext(ctx, q, &requestId)
	if err != nil {
		return err
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	switch cnt {
	case 0:
		return db.ErrNotUpdated
	case 1:
		return nil
	default:
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}
}

var requestList []proto.RequestSpec

func (m *manager) Specs() []proto.RequestSpec {
	m.Lock()
	defer m.Unlock()

	if requestList != nil {
		return requestList
	}

	gr := m.grf.Make()
	req := gr.Sequences()
	sortedReqNames := make([]string, 0, len(req))
	for name := range req {
		if req[name].Request {
			sortedReqNames = append(sortedReqNames, name)
		}
	}
	sort.Strings(sortedReqNames)

	requestList = make([]proto.RequestSpec, 0, len(sortedReqNames))
	for _, name := range sortedReqNames {
		s := proto.RequestSpec{
			Name: name,
			Args: []proto.RequestArg{},
		}
		for _, arg := range req[name].Args.Required {
			a := proto.RequestArg{
				Name:     arg.Name,
				Desc:     arg.Desc,
				Required: true,
			}
			s.Args = append(s.Args, a)
		}
		for _, arg := range req[name].Args.Optional {
			a := proto.RequestArg{
				Name:     arg.Name,
				Desc:     arg.Desc,
				Required: false,
				Default:  arg.Default,
			}
			s.Args = append(s.Args, a)
		}
		requestList = append(requestList, s)
	}

	return requestList
}

func (s *manager) JobChain(requestId string) (proto.JobChain, error) {
	var jc proto.JobChain
	var rawJc []byte // raw job chains are stored as blobs in the db.

	ctx := context.TODO()
	conn, err := s.dbc.Open(ctx)
	if err != nil {
		return jc, err
	}
	defer s.dbc.Close(conn) // don't leak conn

	// Get the job chain from the raw_requests table.
	q := "SELECT job_chain FROM raw_requests WHERE request_id = ?"
	if err := conn.QueryRowContext(ctx, q, requestId).Scan(&rawJc); err != nil {
		switch err {
		case sql.ErrNoRows:
			return jc, db.NewErrNotFound("job chain")
		default:
			return jc, err
		}
	}

	// Unmarshal the job chain into a proto.JobChain.
	if err := json.Unmarshal(rawJc, &jc); err != nil {
		return jc, fmt.Errorf("cannot unmarshal job chain: %s", err)
	}

	return jc, nil
}

// ------------------------------------------------------------------------- //

// get a request with proto.Request.JobChain set
func (m *manager) getWithJc(requestId string) (proto.Request, error) {
	req, err := m.Get(requestId)
	if err != nil {
		return req, err
	}

	ctx := context.TODO()
	conn, err := m.dbc.Open(ctx)
	if err != nil {
		return req, err
	}
	defer m.dbc.Close(conn) // don't leak conn

	var jc proto.JobChain
	var params proto.CreateRequestParams
	var rawJc []byte     // raw job chains are stored as blobs in the db.
	var rawParams []byte // raw params are stored as blobs in the db.
	q := "SELECT job_chain, request FROM raw_requests WHERE request_id = ?"
	if err := conn.QueryRowContext(ctx, q, requestId).Scan(&rawJc, &rawParams); err != nil {
		switch err {
		case sql.ErrNoRows:
			return req, db.NewErrNotFound("raw_request")
		default:
			return req, err
		}
	}

	if err := json.Unmarshal(rawJc, &jc); err != nil {
		return req, fmt.Errorf("cannot unmarshal job chain: %s", err)
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return req, fmt.Errorf("cannot unmarshal params: %s", err)
	}

	req.JobChain = &jc
	req.Params = params.Args
	return req, nil
}

// Updates the state, started/finished timestamps, and JR host of the provided
// request. The request is updated only if its current state (in the db) matches
// the state provided.
func (m *manager) updateRequest(req proto.Request, curState byte) error {
	ctx := context.TODO()
	conn, err := m.dbc.Open(ctx)
	if err != nil {
		return err
	}
	defer m.dbc.Close(conn) // don't leak conn

	// If JobRunnerHost is empty, we want to set the db field to NULL (not an empty string).
	var jrHost interface{}
	if req.JobRunnerHost != "" {
		jrHost = req.JobRunnerHost
	}

	// Fields that should never be updated by this package are not listed in this query.
	q := "UPDATE requests SET state = ?, started_at = ?, finished_at = ?, jr_host = ? WHERE request_id = ? AND state = ?"
	res, err := conn.ExecContext(ctx, q,
		req.State,
		req.StartedAt,
		req.FinishedAt,
		jrHost,
		req.Id,
		curState)
	if err != nil {
		return err
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	switch cnt {
	case 0:
		return db.ErrNotUpdated
	case 1:
		break
	default:
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}

	return nil
}
