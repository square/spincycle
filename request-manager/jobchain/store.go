// Copyright 2017, Square, Inc.

// Package jobchain provides an interface for reading and writing job chains.
package jobchain

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
)

// A Store reads and writes job chains to/from a persistent datastore.
type Store interface {
	// Get retrieves the JC corresponding to the provided request id.
	Get(requestId string) (proto.JobChain, error)
}

// store implements the Store interface.
type store struct {
	dbc db.Connector
}

func NewStore(dbc db.Connector) Store {
	return &store{
		dbc: dbc,
	}
}

func (s *store) Get(requestId string) (proto.JobChain, error) {
	var jc proto.JobChain
	var rawJc []byte // raw job chains are stored as blobs in the db.

	conn, err := s.dbc.Connect() // connection is from a pool. do not close
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
