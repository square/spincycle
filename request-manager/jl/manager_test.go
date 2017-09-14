// Copyright 2017, Square, Inc.

package jl_test

import (
	"database/sql"
	"sort"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
	"github.com/square/spincycle/request-manager/jl"
	"github.com/square/spincycle/request-manager/test"
	testdb "github.com/square/spincycle/request-manager/test/db"
	"github.com/square/spincycle/test/mock"
)

var dbm testdb.Manager
var dbc db.Connector

func setup(t *testing.T, dataFile string) string {
	// Setup a db manager to handle databases for all tests.
	var err error
	if dbm == nil {
		dbm, err = testdb.NewManager()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Setup a db for this specific test, and seed it with some default data.
	dbName, err := dbm.Create(dataFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock connector the connects to the test db.
	dbc = &mock.Connector{
		ConnectFunc: func() (*sql.DB, error) {
			return dbm.Connect(dbName)
		},
	}

	return dbName
}

func teardown(t *testing.T, dbName string) {
	if err := dbm.Destroy(dbName); err != nil {
		t.Fatal(err)
	}
	dbc = nil
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestGetNotFound(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jl-default.sql")
	defer teardown(t, dbName)

	reqId := "invalid"
	jobId := "abcd"
	m := jl.NewManager(dbc)
	_, err := m.Get(reqId, jobId)
	if err != nil {
		switch v := err.(type) {
		case db.ErrNotFound:
			break // this is what we expect
		default:
			t.Errorf("error is of type %s, expected db.ErrNotFound", v)
		}
	} else {
		t.Error("expected an error, did not get one")
	}
}

func TestCreateAndGet(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jl-default.sql")
	defer teardown(t, dbName)

	// Insert two JLs.
	reqId := "fa0d862f16ca4f14a0613e2c26562de6"
	jobId1 := "fh17"
	jl1 := proto.JobLog{
		RequestId: reqId,
		JobId:     jobId1,
		Type:      "something",
		State:     proto.STATE_FAIL,
	}
	jobId2 := "df2j"
	jl2 := proto.JobLog{
		RequestId: reqId,
		JobId:     jobId2,
		Type:      "something-else",
		State:     proto.STATE_COMPLETE,
	}
	jls := []proto.JobLog{jl1, jl2}

	m := jl.NewManager(dbc)
	for _, j := range jls {
		_, err := m.Create(reqId, j)
		if err != nil {
			t.Errorf("error = %s, expected nil", err)
		}
	}

	// Run a direct query against the db and make sure the finished_jobs field
	// on the request got updated. It should only have incremented by 1 since
	// only one of the JLs had stats COMPLETE.
	conn, err := dbc.Connect()
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}
	var count int
	err = conn.QueryRow("SELECT finished_jobs FROM requests WHERE request_id = ?", reqId).Scan(&count)

	if count != 8 {
		t.Errorf("finished jobs = %d, expected 8", count)
	}

	// Get the JL back.
	actualJl, err := m.Get(reqId, jobId2)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if diff := deep.Equal(actualJl, jl2); diff != nil {
		t.Error(diff)
	}
}

func TestGetFull(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jl-default.sql")
	defer teardown(t, dbName)

	reqId := "fa0d862f16ca4f14a0613e2c26562de6"
	m := jl.NewManager(dbc)
	a, err := m.GetFull(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	e := testdb.SavedJLs[reqId]

	// Convert actual and expected from []proto.JobLog to proto.JobLogs so
	// that we can sort them.
	var actual proto.JobLogs
	for _, j := range a {
		actual = append(actual, j)
	}
	var expected proto.JobLogs
	for _, j := range e {
		expected = append(expected, j)
	}
	sort.Sort(actual)
	sort.Sort(expected)

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}
