// Copyright 2017, Square, Inc.

// Package jc provides and interface for managing Job Chains (JCs).
package jc

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
)

// A Manager is used to manage JCs.
type Manager interface {
	// Get retrieves the JC corresponding to the provided request id.
	Get(requestId string) (proto.JobChain, error)
}

type manager struct {
	dbc db.Connector
}

func NewManager(dbc db.Connector) Manager {
	return &manager{
		dbc: dbc,
	}
}

func (m *manager) Get(requestId string) (proto.JobChain, error) {
	var jc proto.JobChain
	var rawJc []byte // raw job chains are stored as blobs in the db.

	conn, err := m.dbc.Connect() // connection is from a pool. do not close
	if err != nil {
		return jc, err
	}

	// Get the job chain from the raw_requests table.
	q := "SELECT job_chain FROM raw_requests WHERE request_id = ?"
	if err := conn.QueryRow(q, requestId).Scan(&rawJc); err != nil {
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
