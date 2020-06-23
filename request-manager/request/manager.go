// Copyright 2017-2019, Square, Inc.

// Package request provides an interface for managing requests.
package request

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"

	serr "github.com/square/spincycle/errors"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/retry"
)

const (
	DB_TRIES      = 3
	DB_RETRY_WAIT = time.Duration(500 * time.Millisecond)
	JR_TRIES      = 5
	JR_RETRY_WAIT = time.Duration(5 * time.Second)
)

// A Manager creates and manages the life cycle of requests.
type Manager interface {
	// Create creates a request and saves it to the db. The request is not
	// started; its state is pending until Start is called.
	Create(proto.CreateRequest) (proto.Request, error)

	// Get retrieves the request corresponding to the provided id,
	// without its job chain or parameters set.
	Get(requestId string) (proto.Request, error)

	// Get retrieves the request corresponding to the provided id,
	// with its job chain and parameters.
	GetWithJC(requestId string) (proto.Request, error)

	// Start starts a request (sends it to the JR).
	Start(requestId string) error

	// Stop stops a request (sends a stop signal to the JR).
	Stop(requestId string) error

	// Finish marks a request as being finished. It gets the request's final
	// state from the proto.FinishRequest argument.
	Finish(requestId string, finishParams proto.FinishRequest) error

	// Specs returns a list of all the request specs the the RM knows about.
	Specs() []proto.RequestSpec

	// JobChain returns the job chain for the given request id.
	JobChain(requestId string) (proto.JobChain, error)

	// Find returns a list of requests that match the given filter criteria,
	// in descending order by create time (i.e. most recent first) and ascending
	// by request id where create time is not unique. Returned requests do
	// not have job chain or args set.
	Find(filter proto.RequestFilter) ([]proto.Request, error)
}

// manager implements the Manager interface.
type manager struct {
	grf          grapher.GrapherFactory
	dbc          *sql.DB
	jrc          jr.Client
	defaultJRURL string
	shutdownChan chan struct{}
	*sync.Mutex
}

type ManagerConfig struct {
	GrapherFactory grapher.GrapherFactory
	DBConnector    *sql.DB
	JRClient       jr.Client
	DefaultJRURL   string
	ShutdownChan   chan struct{}
}

func NewManager(config ManagerConfig) Manager {
	return &manager{
		grf:          config.GrapherFactory,
		dbc:          config.DBConnector,
		jrc:          config.JRClient,
		defaultJRURL: config.DefaultJRURL,
		shutdownChan: config.ShutdownChan,
		Mutex:        &sync.Mutex{},
	}
}

func (m *manager) Create(newReq proto.CreateRequest) (proto.Request, error) {
	var req proto.Request
	if newReq.Type == "" {
		return req, serr.ErrInvalidCreateRequest{Message: "Type is empty, must be a request name"}
	}

	reqIdBytes := xid.New()
	reqId := reqIdBytes.String()
	req = proto.Request{
		Id:        reqId,
		Type:      newReq.Type,
		CreatedAt: time.Now().UTC(),
		State:     proto.STATE_PENDING,
		User:      newReq.User, // Caller.Name if not set by SetUsername
	}

	// ----------------------------------------------------------------------
	// Verify and finalize request args. The final request args are given
	// (from caller) + optional + static.
	gr := m.grf.Make(req)
	reqArgs, err := gr.RequestArgs(req.Type, newReq.Args)
	if err != nil {
		return req, err
	}
	req.Args = reqArgs

	// Copy requests args -> initial job args. We save the former as a record
	// (request_archives.args) of every request arg that the request was started
	// with. CreateGraph modifies and greatly expands the latter (job args).
	// Final job args are saved with each job because the same job arg can have
	// different values for different jobs (especially true for each: expansions).
	jobArgs := map[string]interface{}{}
	for k, v := range newReq.Args {
		jobArgs[k] = v
	}

	// ----------------------------------------------------------------------
	// Create graph from request specs and jobs args. Then translate the
	// generic graph into a job chain and save it with the request.
	newGraph, err := gr.CreateGraph(req.Type, jobArgs)
	if err != nil {
		return req, err
	}
	jc := &proto.JobChain{
		AdjacencyList: newGraph.Edges,
		RequestId:     reqId,
		State:         proto.STATE_PENDING,
		Jobs:          map[string]proto.Job{},
	}
	for jobId, node := range newGraph.Vertices {
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
	req.JobChain = jc
	req.TotalJobs = uint(len(jc.Jobs))

	// ----------------------------------------------------------------------
	// Serial data for request_archives
	jcBytes, err := json.Marshal(req.JobChain)
	if err != nil {
		return req, fmt.Errorf("cannot marshal job chain: %s", err)
	}
	newReqBytes, err := json.Marshal(newReq)
	if err != nil {
		return req, fmt.Errorf("cannot marshal create request: %s", err)
	}
	reqArgsBytes, err := json.Marshal(reqArgs)
	if err != nil {
		return req, fmt.Errorf("cannot marshal request args: %s", err)
	}

	// ----------------------------------------------------------------------
	// Save everything in a transaction. request_archive is immutable data,
	// i.e. these never change now that request is fully created. requests is
	// highly mutable, especially requests.state and requests.finished_jobs.
	ctx := context.TODO()
	err = retry.Do(DB_TRIES, DB_RETRY_WAIT, func() error {
		txn, err := m.dbc.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer txn.Rollback()

		q := "INSERT INTO request_archives (request_id, create_request, args, job_chain) VALUES (?, ?, ?, ?)"
		_, err = txn.ExecContext(ctx, q,
			reqIdBytes,
			string(newReqBytes),
			string(reqArgsBytes),
			jcBytes,
		)
		if err != nil {
			return serr.NewDbError(err, "INSERT request_archives")
		}

		q = "INSERT INTO requests (request_id, type, state, user, created_at, total_jobs) VALUES (?, ?, ?, ?, ?, ?)"
		_, err = txn.ExecContext(ctx, q,
			reqIdBytes,
			req.Type,
			req.State,
			req.User,
			req.CreatedAt,
			req.TotalJobs,
		)
		if err != nil {
			return serr.NewDbError(err, "INSERT requests")
		}
		return txn.Commit()
	}, nil)
	return req, err
}

// Retrieve the request without its corresponding Job Chain.
func (m *manager) Get(requestId string) (proto.Request, error) {
	var req proto.Request

	ctx := context.TODO()

	// Nullable columns.
	var user sql.NullString
	var jrURL sql.NullString
	startedAt := mysql.NullTime{}
	finishedAt := mysql.NullTime{}

	var reqArgsBytes []byte

	// Technically, a LEFT JOIN shouldn't be necessary, but we have tests that
	// create a request but no corresponding request_archive which makes a plain
	// JOIN not match any row.
	q := "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, finished_jobs, jr_url, args" +
		" FROM requests r LEFT JOIN request_archives a USING (request_id)" +
		" WHERE request_id = ?"
	notFound := false
	err := retry.Do(DB_TRIES, DB_RETRY_WAIT, func() error {
		err := m.dbc.QueryRowContext(ctx, q, requestId).Scan(
			&req.Id,
			&req.Type,
			&req.State,
			&user,
			&req.CreatedAt,
			&startedAt,
			&finishedAt,
			&req.TotalJobs,
			&req.FinishedJobs,
			&jrURL,
			&reqArgsBytes,
		)
		if err != nil {
			switch err {
			case sql.ErrNoRows:
				notFound = true
				return nil // don't try again
			default:
				return err
			}
		}
		return nil
	}, nil)
	if err != nil {
		return req, serr.NewDbError(err, "SELECT requests")
	}
	if notFound {
		return req, serr.RequestNotFound{requestId}
	}

	if user.Valid {
		req.User = user.String
	}
	if jrURL.Valid {
		req.JobRunnerURL = jrURL.String
	}
	if startedAt.Valid {
		req.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		req.FinishedAt = &finishedAt.Time
	}

	if len(reqArgsBytes) > 0 {
		var reqArgs []proto.RequestArg
		if err := json.Unmarshal(reqArgsBytes, &reqArgs); err != nil {
			return req, err
		}
		req.Args = reqArgs
	}
	return req, nil
}

func (m *manager) Start(requestId string) error {
	req, err := m.GetWithJC(requestId)
	if err != nil {
		return err
	}

	// Only start the request if it's currently Pending.
	if req.State != proto.STATE_PENDING {
		return serr.NewErrInvalidState(proto.StateName[proto.STATE_PENDING], proto.StateName[req.State])
	}

	// Send the request's job chain to the job runner, which will start running it.
	var chainURL *url.URL
	for i := 0; i < JR_TRIES; i++ {
		if i != 0 {
			time.Sleep(JR_RETRY_WAIT)
		}
		chainURL, err = m.jrc.NewJobChain(m.defaultJRURL, *req.JobChain)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	req.StartedAt = &now
	req.State = proto.STATE_RUNNING

	req.JobRunnerURL = strings.TrimSuffix(chainURL.String(), chainURL.RequestURI())

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
		return serr.NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[req.State])
	}

	// Tell the JR to stop running the job chain for the request.
	err = m.jrc.StopRequest(req.JobRunnerURL, requestId)
	if err != nil {
		return fmt.Errorf("error stopping request in Job Runner: %s", err)
	}

	return nil
}

func (m *manager) Finish(requestId string, finishParams proto.FinishRequest) error {
	req, err := m.Get(requestId)
	if err != nil {
		return err
	}
	log.Infof("finish request: %+v", finishParams)

	prevState := req.State

	req.State = finishParams.State
	req.FinishedAt = &finishParams.FinishedAt
	req.FinishedJobs = finishParams.FinishedJobs
	req.JobRunnerURL = ""

	// This will only update the request if the current state is RUNNING.
	err = m.updateRequest(req, proto.STATE_RUNNING)
	if err != nil {
		if prevState != proto.STATE_RUNNING {
			// This should never happen - we never finish a request that isn't running.
			return serr.NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[prevState])
		}
		return err
	}

	return nil
}

var requestList []proto.RequestSpec

func (m *manager) Specs() []proto.RequestSpec {
	m.Lock()
	defer m.Unlock()

	if requestList != nil {
		return requestList
	}

	gr := m.grf.Make(proto.Request{})
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
				Name: arg.Name,
				Desc: arg.Desc,
				Type: proto.ARG_TYPE_REQUIRED,
			}
			s.Args = append(s.Args, a)
		}
		for _, arg := range req[name].Args.Optional {
			a := proto.RequestArg{
				Name:    arg.Name,
				Desc:    arg.Desc,
				Type:    proto.ARG_TYPE_OPTIONAL,
				Default: arg.Default,
			}
			s.Args = append(s.Args, a)
		}
		requestList = append(requestList, s)
	}

	return requestList
}

func (m *manager) JobChain(requestId string) (proto.JobChain, error) {
	var jc proto.JobChain
	var jcBytes []byte // raw job chains are stored as blobs in the db.

	ctx := context.TODO()

	// Get the job chain from the request_archives table.
	q := "SELECT job_chain FROM request_archives WHERE request_id = ?"
	if err := m.dbc.QueryRowContext(ctx, q, requestId).Scan(&jcBytes); err != nil {
		switch err {
		case sql.ErrNoRows:
			return jc, serr.RequestNotFound{requestId}
		default:
			return jc, serr.NewDbError(err, "SELECT request_archives")
		}
	}

	// Unmarshal the job chain into a proto.JobChain.
	if err := json.Unmarshal(jcBytes, &jc); err != nil {
		return jc, fmt.Errorf("cannot unmarshal job chain: %s", err)
	}

	return jc, nil
}

// Get a request with proto.Request.JobChain and proto.Request.Params set
func (m *manager) GetWithJC(requestId string) (proto.Request, error) {
	req, err := m.Get(requestId)
	if err != nil {
		return req, err
	}

	ctx := context.TODO()

	var jcBytes []byte
	q := "SELECT job_chain FROM request_archives WHERE request_id = ?"
	notFound := false
	err = retry.Do(DB_TRIES, DB_RETRY_WAIT, func() error {
		err := m.dbc.QueryRowContext(ctx, q, requestId).Scan(&jcBytes)
		if err != nil {
			switch err {
			case sql.ErrNoRows:
				notFound = true
				return nil // don't try again
			default:
				return err
			}
		}
		return nil
	}, nil)
	if err != nil {
		return req, serr.NewDbError(err, "SELECT request_archives")
	}
	if notFound {
		return req, serr.RequestNotFound{requestId}
	}

	var jc proto.JobChain
	if err := json.Unmarshal(jcBytes, &jc); err != nil {
		return req, fmt.Errorf("cannot unmarshal job chain: %s", err)
	}
	req.JobChain = &jc

	return req, nil
}

func (m *manager) Find(filter proto.RequestFilter) ([]proto.Request, error) {
	// Build the query from the filter.
	query := "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, finished_jobs, jr_url FROM requests "

	var fields []string
	var values []interface{}
	if filter.Type != "" {
		fields = append(fields, "type = ?")
		values = append(values, filter.Type)
	}
	if filter.User != "" {
		fields = append(fields, "user = ?")
		values = append(values, filter.User)
	}
	if len(filter.States) != 0 {
		stateSQL := fmt.Sprintf("state IN (%s)", strings.TrimRight(strings.Repeat("?, ", len(filter.States)), ", "))
		fields = append(fields, stateSQL)
		for _, state := range filter.States {
			values = append(values, state)
		}
	}
	if !filter.Since.IsZero() {
		fields = append(fields, "(finished_at > ? OR finished_at IS NULL)")
		values = append(values, filter.Since.Format(time.RFC3339Nano))
	}
	if !filter.Until.IsZero() {
		fields = append(fields, "(created_at < ?)")
		values = append(values, filter.Until.Format(time.RFC3339Nano))
	}

	if len(fields) > 0 {
		query += "WHERE " + strings.Join(fields, " AND ")
	}

	query += " ORDER BY created_at DESC, request_id "

	if filter.Limit != 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)

		if filter.Offset != 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	// Query the db and parse results.
	ctx := context.Background()
	var rows *sql.Rows
	err := retry.Do(DB_TRIES, DB_RETRY_WAIT, func() error {
		var err error
		rows, err = m.dbc.QueryContext(ctx, query, values...)
		if err != nil {
			return err
		}
		return nil
	}, nil)
	if err != nil {
		return []proto.Request{}, serr.NewDbError(err, "SELECT request_id")
	}

	var requests []proto.Request
	defer rows.Close()
	for rows.Next() {
		var req proto.Request
		// Nullable columns:
		var user sql.NullString
		var jrURL sql.NullString
		startedAt := mysql.NullTime{}
		finishedAt := mysql.NullTime{}

		err := rows.Scan(
			&req.Id,
			&req.Type,
			&req.State,
			&user,
			&req.CreatedAt,
			&startedAt,
			&finishedAt,
			&req.TotalJobs,
			&req.FinishedJobs,
			&jrURL,
		)
		if err != nil {
			return []proto.Request{}, fmt.Errorf("Error scanning row returned from MySQL: %s", err)
		}

		if user.Valid {
			req.User = user.String
		}
		if jrURL.Valid {
			req.JobRunnerURL = jrURL.String
		}
		if startedAt.Valid {
			req.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			req.FinishedAt = &finishedAt.Time
		}

		requests = append(requests, req)
	}
	if rows.Err() != nil {
		return []proto.Request{}, fmt.Errorf("Error iterating over rows returned from MySQL: %s", err)
	}

	return requests, nil
}

// ------------------------------------------------------------------------- //

// Updates the state, started/finished timestamps, and JR url of the provided
// request. The request is updated only if its current state (in the db) matches
// the state provided.
func (m *manager) updateRequest(req proto.Request, curState byte) error {
	ctx := context.TODO()

	// If JobRunnerURL is empty, we want to set the db field to NULL (not an empty string).
	var jrURL interface{}
	if req.JobRunnerURL != "" {
		jrURL = req.JobRunnerURL
	}

	// Fields that should never be updated by this package are not listed in this query.
	q := "UPDATE requests SET state = ?, started_at = ?, finished_at = ?, finished_jobs = ?, jr_url = ?  WHERE request_id = ? AND state = ?"
	var res sql.Result
	err := retry.Do(DB_TRIES, DB_RETRY_WAIT, func() error {
		var err error
		res, err = m.dbc.ExecContext(ctx, q,
			req.State,
			req.StartedAt,
			req.FinishedAt,
			req.FinishedJobs,
			jrURL,
			req.Id,
			curState,
		)
		return err
	}, nil)
	if err != nil {
		return serr.NewDbError(err, "UPDATE requests")
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	switch cnt {
	case 0:
		return ErrNotUpdated
	case 1:
		break
	default:
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return ErrMultipleUpdated
	}

	return nil
}
