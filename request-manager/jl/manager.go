// Copyright 2017, Square, Inc.

// Package jl provides and interface for managing Job Logs (JLs).
package jl

import (
	"database/sql"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
)

// A Manager is used to create and manage JLs.
type Manager interface {
	// Create saves a JL to the db.
	Create(requestId string, jl proto.JobLog) (proto.JobLog, error)

	// Get gets a single JL.
	Get(requestId string, jobId string) (proto.JobLog, error)

	// GetFull gets all of the JLs for a request.
	GetFull(requestId string) ([]proto.JobLog, error)
}

type manager struct {
	dbc db.Connector
}

func NewManager(dbc db.Connector) Manager {
	return &manager{
		dbc: dbc,
	}
}

func (m *manager) Create(requestId string, jl proto.JobLog) (proto.JobLog, error) {
	jl.RequestId = requestId

	conn, err := m.dbc.Connect() // connection is from a pool. do not close
	if err != nil {
		return jl, err
	}

	q := "INSERT INTO job_log (request_id, job_id, try, type, started_at, finished_at, state, `exit`, " +
		"error, stdout, stderr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err = conn.Exec(q,
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
	if err != nil {
		return jl, err
	}

	// Update the finished job count on the request if the job completed successfully.
	if jl.State == proto.STATE_COMPLETE {
		q = "UPDATE requests SET finished_jobs = finished_jobs + 1 WHERE request_id = ?"
		res, err := conn.Exec(q, &requestId)
		if err != nil {
			return jl, err
		}

		cnt, err := res.RowsAffected()
		if err != nil {
			return jl, err
		}

		switch cnt {
		case 0:
			return jl, db.ErrNotUpdated
		case 1:
			return jl, nil
		default:
			// This should be impossible since we specify the primary key
			// in the WHERE clause of the update.
			return jl, db.ErrMultipleUpdated
		}
	}

	return jl, nil
}

func (m *manager) Get(requestId, jobId string) (proto.JobLog, error) {
	var jl proto.JobLog

	conn, err := m.dbc.Connect() // connection is from a pool. do not close
	if err != nil {
		return jl, err
	}

	var jErr, stdout, stderr sql.NullString // nullable columns
	var exit sql.NullInt64

	q := "SELECT request_id, job_id, type, state, started_at, finished_at, error, `exit`, stdout, stderr, try " +
		"FROM job_log WHERE request_id = ? AND job_id = ? ORDER BY try DESC LIMIT 1"
	err = conn.QueryRow(q, requestId, jobId).Scan(
		&jl.RequestId,
		&jl.JobId,
		&jl.Type,
		&jl.State,
		&jl.StartedAt,
		&jl.FinishedAt,
		&jErr,
		&exit,
		&stdout,
		&stderr,
		&jl.Try)
	switch {
	case err == sql.ErrNoRows:
		return jl, db.NewErrNotFound("job log")
	case err != nil:
		return jl, err
	}

	if jErr.Valid {
		jl.Error = jErr.String
	}
	if stdout.Valid {
		jl.Stdout = stdout.String
	}
	if stderr.Valid {
		jl.Stderr = stderr.String
	}
	if exit.Valid {
		jl.Exit = exit.Int64
	}

	return jl, nil
}

func (m *manager) GetFull(requestId string) ([]proto.JobLog, error) {
	conn, err := m.dbc.Connect() // connection is from a pool. do not close
	if err != nil {
		return nil, err
	}

	var jErr, stdout, stderr sql.NullString // nullable columns
	var exit sql.NullInt64

	q := "SELECT job_id, try, type, state, started_at, finished_at, error, `exit`, stdout, stderr " +
		"FROM job_log WHERE request_id = ?"
	rows, err := conn.Query(q, requestId)
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
			&l.Type,
			&l.State,
			&l.StartedAt,
			&l.FinishedAt,
			&jErr,
			&exit,
			&stdout,
			&stderr,
		)
		if err != nil {
			return nil, err
		}

		if jErr.Valid {
			l.Error = jErr.String
		}
		if stdout.Valid {
			l.Stdout = stdout.String
		}
		if stderr.Valid {
			l.Stderr = stderr.String
		}
		if exit.Valid {
			l.Exit = exit.Int64
		}

		jl = append(jl, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jl, nil
}
