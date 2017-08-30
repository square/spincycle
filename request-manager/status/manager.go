// Copyright 2017, Square, Inc.

// Package status provides system-wide status.
package status

import (
	"sort"

	"github.com/go-sql-driver/mysql"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
)

type Manager interface {
	Running(Filter) (proto.RunningStatus, error)
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
	ORDER_BY_RUNTIME byte = iota
)

type manager struct {
	dbc db.Connector
	jrc jr.Client
}

func NewManager(dbc db.Connector, jrClient jr.Client) Manager {
	return &manager{
		dbc: dbc,
		jrc: jrClient,
	}
}

func (m *manager) Running(f Filter) (proto.RunningStatus, error) {
	status := proto.RunningStatus{}

	running, err := m.jrc.SysStatRunning()
	if err != nil {
		return status, err
	}

	switch f.OrderBy {
	case ORDER_BY_RUNTIME:
		sort.Sort(proto.JobStatusByRuntime(running))
	default:
		sort.Sort(proto.JobStatusByRuntime(running))
	}

	if f.Limit > 0 && len(running) > f.Limit {
		running = running[0:f.Limit]
	}

	status.Jobs = running

	if len(running) == 0 {
		return status, nil
	}

	// Do not close the db. It's a long-running connection pool.
	conn, err := m.dbc.Connect()
	if err != nil {
		return status, err
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

	q := "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, finished_jobs" +
		" FROM requests WHERE request_id IN (" + db.IN(ids) + ")"
	rows, err := conn.Query(q)
	if err != nil {
		return status, err
	}
	defer rows.Close()

	requests := map[string]proto.Request{}
	for rows.Next() {
		r := proto.Request{}
		startedAt := mysql.NullTime{}
		finishedAt := mysql.NullTime{}
		err := rows.Scan(
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
