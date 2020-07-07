// Copyright 2017, Square, Inc.

package joblog_test

import (
	"database/sql"
	"sort"
	"testing"

	"github.com/go-test/deep"

	serr "github.com/square/spincycle/v2/errors"
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/joblog"
	"github.com/square/spincycle/v2/request-manager/test"
	testdb "github.com/square/spincycle/v2/request-manager/test/db"
)

var dbm testdb.Manager
var dbc *sql.DB

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

func TestGetNotFound(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jl-default.sql")
	defer teardown(t, dbName)

	reqId := "invalid"
	jobId := "abcd"
	s := joblog.NewStore(dbc)
	_, err := s.Get(reqId, jobId)
	if err != nil {
		switch v := err.(type) {
		case serr.JobNotFound:
			break // this is what we expect
		default:
			t.Errorf("error is of type %s, expected serr.JobNotFound", v)
		}
	} else {
		t.Error("expected an error, did not get one")
	}
}

func TestCreateAndGet(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jl-default.sql")
	defer teardown(t, dbName)

	// Insert two JLs.
	reqId := "fa0d862f16casg200lkf"
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

	s := joblog.NewStore(dbc)
	for _, j := range jls {
		_, err := s.Create(reqId, j)
		if err != nil {
			t.Errorf("error = %s, expected nil", err)
		}
	}

	// Get the JL back.
	actualJl, err := s.Get(reqId, jobId2)
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

	reqId := "fa0d862f16casg200lkf"
	s := joblog.NewStore(dbc)
	a, err := s.GetFull(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	e := testdb.SavedJLs[reqId]

	// Convert actual and expected from []proto.JobLog to proto.JobLogById so
	// that we can sort them.
	var actual proto.JobLogById
	for _, j := range a {
		actual = append(actual, j)
	}
	var expected proto.JobLogById
	for _, j := range e {
		expected = append(expected, j)
	}
	sort.Sort(actual)
	sort.Sort(expected)

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}
