// Copyright 2017-2019, Square, Inc.

// Package status provides system-wide status.
package status

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/go-sql-driver/mysql"

	serr "github.com/square/spincycle/errors"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/retry"
)

const (
	DB_TRIES      = 3
	DB_RETRY_WAIT = time.Duration(500 * time.Millisecond)
)

type Manager interface {
	Running(Filter) (proto.RunningStatus, error)
	UpdateProgress(proto.RequestProgress) error
}

type Filter struct {
	State   byte
	Limit   int
	OrderBy byte
	// After time.Time
	// Before time.Time
}

// NoFilter is a convenience var for calls like Running(status.NoFilter). Other
// packages must not modify this var.
var NoFilter Filter

const (
	ORDER_BY_START_TIME byte = iota
)

type manager struct {
	dbc *sql.DB
	jrc jr.Client
}

func NewManager(dbc *sql.DB, jrClient jr.Client) Manager {
	return &manager{
		dbc: dbc,
		jrc: jrClient,
	}
}

func (m *manager) Running(f Filter) (proto.RunningStatus, error) {
	status := proto.RunningStatus{}
	ctx := context.TODO()

	// Make a list of the URLs of all JR hosts currently running any requests.
	q := "SELECT jr_url FROM requests WHERE state = ? AND jr_url IS NOT NULL"
	rows, err := m.dbc.QueryContext(ctx, q, proto.STATE_RUNNING)
	if err != nil {
		return status, serr.NewDbError(err, "SELECT requests")
	}
	defer rows.Close()

	jrURLs := map[string]struct{}{}
	for rows.Next() {
		var jrURL string
		err := rows.Scan(&jrURL)
		if err != nil {
			return status, err
		}

		// We only care about the presence of the key in the map, not the value.
		jrURLs[jrURL] = struct{}{}
	}

	// Get the status of all running jobs from each JR host.
	var running []proto.JobStatus
	for url := range jrURLs {
		runningFromHost, err := m.jrc.SysStatRunning(url)
		if err != nil {
			return status, err
		}

		running = append(running, runningFromHost...)
	}

	switch f.OrderBy {
	case ORDER_BY_START_TIME:
		sort.Sort(proto.JobStatusByStartTime(running))
	default:
		sort.Sort(proto.JobStatusByStartTime(running))
	}

	if f.Limit > 0 && len(running) > f.Limit {
		running = running[0:f.Limit]
	}

	status.Jobs = running

	if len(running) == 0 {
		return status, nil
	}

	seen := map[string]bool{}
	ids := []string{}
	for _, r := range running {
		if seen[r.RequestId] {
			continue
		}
		ids = append(ids, r.RequestId)
		seen[r.RequestId] = true
	}

	q = "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, finished_jobs" +
		" FROM requests WHERE request_id IN (" + inList(ids) + ")"
	rows2, err := m.dbc.QueryContext(ctx, q)
	if err != nil {
		return status, serr.NewDbError(err, "SELECT requests")
	}
	defer rows2.Close()

	requests := map[string]proto.Request{}
	for rows2.Next() {
		r := proto.Request{}
		startedAt := mysql.NullTime{}
		finishedAt := mysql.NullTime{}
		err := rows2.Scan(
			&r.Id,
			&r.Type,
			&r.State,
			&r.User,
			&r.CreatedAt,
			&startedAt,
			&finishedAt,
			&r.TotalJobs,
			&r.FinishedJobs,
		)
		if err != nil {
			return status, err
		}
		if startedAt.Valid {
			r.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			r.FinishedAt = &finishedAt.Time
		}
		requests[r.Id] = r
	}

	status.Requests = requests

	return status, err
}

// ["a","b"] -> "'a','b'"
func inList(vals []string) string {
	in := ""
	n := len(vals) - 1
	for i, val := range vals {
		in += "'" + val + "'"
		if i < n {
			in += ","
		}
	}
	return in
}

func (m *manager) UpdateProgress(prg proto.RequestProgress) error {
	ctx := context.TODO()
	q := "UPDATE requests SET finished_jobs = ? WHERE request_id = ? AND state = ?"
	var res sql.Result
	err := retry.Do(DB_TRIES, DB_RETRY_WAIT, func() error {
		var err error
		res, err = m.dbc.ExecContext(ctx, q, prg.FinishedJobs, prg.RequestId, proto.STATE_RUNNING)
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
		// @todo: check if it didn't match because state != RUNNING
		return serr.RequestNotFound{prg.RequestId}
	case 1:
		return nil
	default:
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return fmt.Errorf("UpdateProgress: request_id = %s matched %d rows, expected 1", prg.RequestId, cnt)
	}
}
