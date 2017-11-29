// Copyright 2017, Square, Inc.

package jobchain_test

import (
	"database/sql"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/request-manager/db"
	"github.com/square/spincycle/request-manager/jobchain"
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
	dbName := setup(t, test.DataPath+"/jc-default.sql")
	defer teardown(t, dbName)

	reqId := "invalid"
	s := jobchain.NewStore(dbc)
	_, err := s.Get(reqId)
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

func TestGetInvalid(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jc-bad.sql")
	defer teardown(t, dbName)

	reqId := "cd724fd12092"
	s := jobchain.NewStore(dbc)
	_, err := s.Get(reqId)
	if err == nil {
		t.Errorf("expected an error unmarshaling the job chain, did not get one")
	}
}

func TestGet(t *testing.T) {
	dbName := setup(t, test.DataPath+"/jc-default.sql")
	defer teardown(t, dbName)

	reqId := "8bff5def4f3fvh78skjy"
	s := jobchain.NewStore(dbc)
	actual, err := s.Get(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if diff := deep.Equal(actual, testdb.SavedJCs[reqId]); diff != nil {
		t.Error(diff)
	}
}
