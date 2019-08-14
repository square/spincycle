// Copyright 2017-2019, Square, Inc.

// Package status provides system-wide status.
package status

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"

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
	Running(proto.StatusFilter) (proto.RunningStatus, error)
	UpdateProgress(proto.RequestProgress) error
}

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

func (m *manager) Running(f proto.StatusFilter) (proto.RunningStatus, error) {
	var noStatus proto.RunningStatus // returned on error
	ctx := context.TODO()

	// -------------------------------------------------------------------------
	// Get running jobs from all JRs in parallel
	// -------------------------------------------------------------------------

	jrURLs, err := m.jrURLS()
	if err != nil {
		return noStatus, err
	}
	if len(jrURLs) == 0 {
		return noStatus, nil
	}

	var wg sync.WaitGroup
	jobStatusChan := make(chan []proto.JobStatus, len(jrURLs))
	for _, url := range jrURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			runningJobs, err := m.jrc.Running(url, f)
			if err != nil {
				log.Warnf("error getting running status from %s: %s", url, err)
				return
			}
			jobStatusChan <- runningJobs
		}(url)
	}
	wg.Wait()
	close(jobStatusChan)

	// -------------------------------------------------------------------------
	// Combine and sort results
	// -------------------------------------------------------------------------

	all := proto.RunningStatus{
		Jobs:     []proto.JobStatus{},
		Requests: map[string]proto.Request{},
	}
	for jobs := range jobStatusChan {
		all.Jobs = append(all.Jobs, jobs...)
	}

	if len(all.Jobs) == 0 {
		return all, nil
	}

	switch strings.ToLower(f.OrderBy) {
	case "starttime":
		sort.Sort(proto.JobStatusByStartTime(all.Jobs))
	default:
		sort.Sort(proto.JobStatusByStartTime(all.Jobs))
	}

	// -------------------------------------------------------------------------
	// Get request info
	// -------------------------------------------------------------------------

	seen := map[string]bool{}
	ids := []string{}
	for _, j := range all.Jobs {
		if seen[j.RequestId] {
			continue
		}
		seen[j.RequestId] = true
		ids = append(ids, j.RequestId)
	}

	q := "SELECT request_id, type, state, user, created_at, started_at, finished_at, total_jobs, finished_jobs" +
		" FROM requests WHERE request_id IN (" + inList(ids) + ")"
	rows, err := m.dbc.QueryContext(ctx, q)
	if err != nil {
		return noStatus, serr.NewDbError(err, "SELECT requests")
	}
	defer rows.Close()

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
			return noStatus, err
		}
		if startedAt.Valid {
			r.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			r.FinishedAt = &finishedAt.Time
		}
		all.Requests[r.Id] = r
	}

	return all, err
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
	if cnt > 1 {
		// This should be impossible since we specify the primary key
		// in the WHERE clause of the update.
		return fmt.Errorf("UpdateProgress: request_id = %s matched %d rows, expected 1", prg.RequestId, cnt)
	}

	// Presuming the request ID is correct and request is running, cnt = 0 happens
	// when finished_jobs = prg.FinishedJobs, i.e. no more finished jobs since last
	// update. cnt = 1 happens when we increment finished jobs.
	return nil
}

func (m *manager) jrURLS() ([]string, error) {
	// Make a list of the URLs of all JR hosts currently running any requests.
	ctx := context.TODO()
	q := "SELECT DISTINCT jr_url FROM requests WHERE state = ? AND jr_url IS NOT NULL"
	rows, err := m.dbc.QueryContext(ctx, q, proto.STATE_RUNNING)
	if err != nil {
		return nil, serr.NewDbError(err, "SELECT requests")
	}
	defer rows.Close()
	jrURLs := []string{}
	for rows.Next() {
		var url string
		err := rows.Scan(&url)
		if err != nil {
			return nil, err
		}
		jrURLs = append(jrURLs, url)
	}
	return jrURLs, nil
}
