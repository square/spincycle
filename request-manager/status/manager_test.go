// Copyright 2017-2019, Square, Inc.

package status_test

import (
	"database/sql"
	"testing"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/status"
	rmtest "github.com/square/spincycle/request-manager/test"
	testdb "github.com/square/spincycle/request-manager/test/db"
	"github.com/square/spincycle/test/mock"
)

var dbm testdb.Manager
var dbc *sql.DB
var dbSuffix string

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

	db, err := dbm.Connect(dbName)
	if err != nil {
		t.Fatal(err)
	}
	dbc = db

	return dbName
}

func teardown(t *testing.T, dbName string) {
	if err := dbm.Destroy(dbName); err != nil {
		t.Fatal(err)
	}
	dbc.Close()
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestIncrementFinishedJobs(t *testing.T) {
	// In this test data, finished_jobs=4 and the request is running
	reqId := "454ae2f98a05cv16sdwt"
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	m := status.NewManager(dbc, &mock.JRClient{})

	prg := proto.RequestProgress{
		RequestId:    reqId,
		FinishedJobs: 8, // 4 -> 8
	}
	err := m.UpdateProgress(prg)
	if err != nil {
		t.Error(err)
	}

	gotFinishedJobs := 0
	err = dbc.QueryRow("SELECT finished_jobs FROM requests WHERE request_id=?", reqId).Scan(&gotFinishedJobs)
	if err != nil {
		t.Fatal(err)
	}
	if gotFinishedJobs != 8 {
		t.Errorf("got finished_jobs = %d, expected 8", gotFinishedJobs)
	}
}
