// Copyright 2017-2019, Square, Inc.

// Package joblog provides an interface for reading and writing job logs.
package joblog

import (
	"context"
	"database/sql"

	serr "github.com/square/spincycle/errors"
	"github.com/square/spincycle/proto"
)

// A Store reads and writes job logs to/from a persistent datastore.
type Store interface {
	// Create saves a JL to the db.
	Create(requestId string, jl proto.JobLog) (proto.JobLog, error)

	// Get gets a single JL.
	Get(requestId string, jobId string) (proto.JobLog, error)

	// GetFull gets all of the JLs for a request.
	GetFull(requestId string) ([]proto.JobLog, error)
}

// store implements the Store interface
type store struct {
	dbc *sql.DB
}

func NewStore(dbc *sql.DB) Store {
	return &store{
		dbc: dbc,
	}
}

func (s *store) Create(requestId string, jl proto.JobLog) (proto.JobLog, error) {
	jl.RequestId = requestId
	ctx := context.TODO()

	q := "INSERT INTO job_log (request_id, job_id, name, try, type, started_at, finished_at, state, `exit`, " +
		"error, stdout, stderr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := s.dbc.ExecContext(ctx, q,
		&jl.RequestId,
		&jl.JobId,
		&jl.Name,
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

	return jl, nil
}

func (s *store) Get(requestId, jobId string) (proto.JobLog, error) {
	var jl proto.JobLog
	ctx := context.TODO()

	var jErr, stdout, stderr sql.NullString // nullable columns
	var exit sql.NullInt64

	q := "SELECT request_id, job_id, name, type, state, started_at, finished_at, error, `exit`, stdout, stderr, try " +
		" FROM job_log WHERE request_id = ? AND job_id = ? ORDER BY try DESC LIMIT 1"
	err := s.dbc.QueryRowContext(ctx, q, requestId, jobId).Scan(
		&jl.RequestId,
		&jl.JobId,
		&jl.Name,
		&jl.Type,
		&jl.State,
		&jl.StartedAt,
		&jl.FinishedAt,
		&jErr,
		&exit,
		&stdout,
		&stderr,
		&jl.Try,
	)
	switch {
	case err == sql.ErrNoRows:
		return jl, serr.JobNotFound{RequestId: jl.RequestId, JobId: jl.JobId}
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

func (s *store) GetFull(requestId string) ([]proto.JobLog, error) {
	ctx := context.TODO()

	var jErr, stdout, stderr sql.NullString // nullable columns
	var exit sql.NullInt64

	q := "SELECT job_id, name, try, type, state, started_at, finished_at, error, `exit`, stdout, stderr" +
		" FROM job_log WHERE request_id = ?"
	rows, err := s.dbc.QueryContext(ctx, q, requestId)
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
			&l.Name,
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
