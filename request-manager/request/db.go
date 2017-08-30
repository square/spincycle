// Copyright 2017, Square, Inc.

package request

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/square/spincycle/proto"
)

const (
	// Request queries.
	insertRequest = "INSERT INTO requests (request_id, type, state, user, created_at, total_jobs) VALUES (?, ?, ?, ?, ?, ?)"
	selectRequest = "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, " +
		"finished_jobs FROM requests WHERE request_id = ?"
	updateRequest           = "UPDATE requests SET state = ?, started_at = ?, finished_at = ? WHERE request_id = ?"
	incrRequestFinishedJobs = "UPDATE requests SET finished_jobs = finished_jobs + 1 WHERE request_id = ?"

	// Raw Request queries.
	insertRawRequest = "INSERT INTO raw_requests (request_id, request, job_chain) VALUES (?, ?, ?)"
	getJobChain      = "SELECT job_chain FROM raw_requests WHERE request_id = ?"

	// JL queries.
	createJL = "INSERT INTO job_log (request_id, job_id, try, type, started_at, finished_at, state, `exit`, " +
		"error, stdout, stderr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	selectLatestJL = "SELECT request_id, job_id, state, started_at, finished_at, error, `exit`, stdout, stderr, try " +
		"FROM job_log WHERE request_id = ? AND job_id = ? ORDER BY try DESC LIMIT 1"
	// Select the job id and state of all jobs in a request, using the latest JL for each job.
	selectRequestLatestJLStates = "SELECT j1.job_id, j1.state FROM job_log j1 LEFT JOIN job_log j2 ON (j1.request_id = " +
		"j2.request_id AND j1.job_id = j2.job_id AND j1.try < j2.try) WHERE j1.request_id = ? AND j2.try IS NULL"
	selectFullJL = "SELECT job_id, try, state, started_at, finished_at, error, `exit`, stdout, stderr " +
		"FROM job_log WHERE request_id = ?"
)

// A DBAccessor persists requests to a database.
type DBAccessor interface {
	// SaveRequest saves a proto.Request, along with it's job chain and the
	// request params that created it (both as byte arrays), in the database.
	SaveRequest(proto.Request, proto.CreateRequestParams) error

	// GetRequest retrieves a request from the database.
	GetRequest(string) (proto.Request, error)

	// UpdateRequest updates a request in the database.
	UpdateRequest(proto.Request) error

	// IncrementRequestFinishedJobs increments the finished_jobs field on a
	// request in the database. This is so that, when you just get the high-
	// level status of a request, you can see what % of jobs are finished.
	IncrementRequestFinishedJobs(string) error

	// GetRequestJobStatuses takes a request id and returns the status of
	// all of the jobs in the request in the form of a proto.RequestStatus.
	// It uses the most recent JL entry for each job to determine its status.
	GetRequestJobStatuses(string) (proto.JobStatuses, error)

	// GetJobChain retrieves a job chain from the database.
	GetJobChain(string) (proto.JobChain, error)

	// Get full job log of a request.
	GetFullJL(string) ([]proto.JobLog, error)

	// GetLatestJL takes a request id and a job id and returns the latest
	// corresponding jog log.
	GetLatestJL(string, string) (proto.JobLog, error)

	// SaveJL saves a proto.JobLog to the database.
	SaveJL(proto.JobLog) error
}

type dbAccessor struct {
	db *sql.DB
}

func NewDBAccessor(db *sql.DB) DBAccessor {
	return &dbAccessor{
		db: db,
	}
}

func (d *dbAccessor) SaveRequest(req proto.Request, reqParams proto.CreateRequestParams) error {
	// Marshal the the job chain and request params.
	rawJc, err := json.Marshal(req.JobChain)
	if err != nil {
		return fmt.Errorf("cannot marshal job chain: %s", err)
	}
	rawParams, err := json.Marshal(reqParams)
	if err != nil {
		return fmt.Errorf("cannot marshal request params: %s", err)
	}

	// Begin a transaction.
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	// Insert the request into the requests table.
	_, err = txn.Exec(insertRequest,
		req.Id,
		req.Type,
		req.State,
		req.User,
		req.CreatedAt,
		req.TotalJobs,
	)
	if err != nil {
		return err
	}

	// Insert the job chain and raw request params into the raw_requests table.
	if _, err = txn.Exec(insertRawRequest,
		req.Id,
		rawParams,
		rawJc); err != nil {
		return err
	}

	return txn.Commit()
}

func (d *dbAccessor) GetRequest(requestId string) (proto.Request, error) {
	var request proto.Request

	// Prepare for handling non-standard columns.
	startedAt := mysql.NullTime{}
	finishedAt := mysql.NullTime{}

	// Get the request from the requests table.
	if err := d.db.QueryRow(selectRequest, requestId).Scan(
		&request.Id,
		&request.Type,
		&request.State,
		&request.User,
		&request.CreatedAt,
		&startedAt,
		&finishedAt,
		&request.TotalJobs,
		&request.FinishedJobs); err != nil {
		switch err {
		case sql.ErrNoRows:
			return request, NewErrNotFound("request")
		default:
			return request, err
		}
	}

	// Add potentially null columns (that aren't actually null for this request)
	// to the request struct.
	if startedAt.Valid {
		request.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		request.FinishedAt = &finishedAt.Time
	}

	return request, nil
}

func (d *dbAccessor) UpdateRequest(request proto.Request) error {
	// Update the request.
	res, err := d.db.Exec(updateRequest,
		request.State,
		request.StartedAt,
		request.FinishedAt,
		request.Id)
	if err != nil {
		return err
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	switch cnt {
	case 0:
		return ErrNotUpdated
	case 1:
		return nil
	default:
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return ErrMultipleUpdated
	}
}

func (d *dbAccessor) IncrementRequestFinishedJobs(requestId string) error {
	// Update (increment) the finished job count on the request.
	res, err := d.db.Exec(incrRequestFinishedJobs, &requestId)
	if err != nil {
		return err
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	switch cnt {
	case 0:
		return ErrNotUpdated
	case 1:
		return nil
	default:
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return ErrMultipleUpdated
	}

	return err
}

func (d *dbAccessor) GetRequestJobStatuses(requestId string) (proto.JobStatuses, error) {
	var js proto.JobStatuses

	rows, err := d.db.Query(selectRequestLatestJLStates, requestId)
	if err != nil {
		return js, err
	}
	defer rows.Close()

	for rows.Next() {
		var status proto.JobStatus
		if err := rows.Scan(&status.JobId, &status.State); err != nil {
			return js, err
		}

		js = append(js, status)
	}

	return js, nil
}

func (d *dbAccessor) GetJobChain(requestId string) (proto.JobChain, error) {
	var jc proto.JobChain
	var rawJc []byte // raw job chains are stored as blobs in the db.

	// Get the job chain from the raw_requests table.
	if err := d.db.QueryRow(getJobChain, requestId).Scan(&rawJc); err != nil {
		switch err {
		case sql.ErrNoRows:
			return jc, NewErrNotFound("job chain")
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

func (d *dbAccessor) GetFullJL(requestId string) ([]proto.JobLog, error) {
	rows, err := d.db.Query(selectFullJL, requestId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jl := []proto.JobLog{}
	for rows.Next() {
		// Get the JL from the job_log table.
		l := proto.JobLog{
			RequestId: requestId,
		}
		err := rows.Scan(
			&l.JobId,
			&l.Try,
			&l.State,
			&l.StartedAt,
			&l.FinishedAt,
			&l.Error,
			&l.Exit,
			&l.Stdout,
			&l.Stderr,
		)
		if err != nil {
			return nil, err
		}
		jl = append(jl, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jl, nil
}

func (d *dbAccessor) GetLatestJL(requestId, jobId string) (proto.JobLog, error) {
	var jl proto.JobLog

	// Get the JL from the job_log table.
	err := d.db.QueryRow(selectLatestJL, requestId, jobId).Scan(
		&jl.RequestId,
		&jl.JobId,
		&jl.State,
		&jl.StartedAt,
		&jl.FinishedAt,
		&jl.Error,
		&jl.Exit,
		&jl.Stdout,
		&jl.Stderr,
		&jl.Try)
	switch {
	case err == sql.ErrNoRows:
		return jl, NewErrNotFound("job log")
	case err != nil:
		return jl, err
	}

	return jl, nil
}

func (d *dbAccessor) SaveJL(jl proto.JobLog) error {
	// Insert the JL in the job_log table.
	_, err := d.db.Exec(createJL,
		&jl.RequestId,
		&jl.JobId,
		&jl.Try,
		&jl.Type,
		&jl.StartedAt,
		&jl.FinishedAt,
		&jl.State,
		&jl.Exit,
		&jl.Error,
		&jl.Stdout,
		&jl.Stderr,
	)

	return err
}
