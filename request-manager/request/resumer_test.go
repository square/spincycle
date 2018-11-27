// Copyright 2018, Square, Inc.

package request_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	myconn "github.com/go-mysql/conn"
	"github.com/go-test/deep"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/request"
	rmtest "github.com/square/spincycle/request-manager/test"
	testdb "github.com/square/spincycle/request-manager/test/db"
	"github.com/square/spincycle/test/mock"
)

// also gets all the other testing vars from manager_test
var rm request.Manager

func setupResumer(t *testing.T, dataFile string) string {
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

	// Create a real myconn.Pool using the db and sql.DB created above.
	dbc = myconn.NewPool(db)

	// Create a shutdown channel - this is a package var from manager_test.go
	shutdownChan = make(chan struct{})

	rm = request.NewManager(&grapher.MockGrapherFactory{}, dbc, &mock.JRClient{}, shutdownChan)

	return dbName
}

func teardownResumer(t *testing.T, dbName string) {
	if err := dbm.Destroy(dbName); err != nil {
		t.Fatal(err)
	}
	dbc = nil
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestSuspend(t *testing.T) {
	dbName := setupResumer(t, rmtest.DataPath+"/request-default.sql")
	defer teardownResumer(t, dbName)

	reqId := "454ae2f98a05cv16sdwt" // request is running
	jobTries := map[string]uint{"job1": 2, "job2": 2}
	stoppedJobTries := map[string]uint{"job2": 1}
	sequenceRetries := map[string]uint{"job1": 1}
	sjc := proto.SuspendedJobChain{
		RequestId:         reqId,
		JobChain:          testdb.SavedRequests[reqId].JobChain,
		TotalJobTries:     jobTries,
		LatestRunJobTries: stoppedJobTries,
		SequenceTries:     sequenceRetries,
	}

	cfg := request.ResumerConfig{
		RequestManager: rm,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		RMHost:         "hostname",
		ShutdownChan:   shutdownChan,
		ResumeInterval: request.ResumeInterval,
	}
	r := request.NewResumer(cfg)
	err := r.Suspend(reqId, sjc)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Get the request from the db and make sure its state was updated.
	req, err := rm.Get(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if req.State != proto.STATE_SUSPENDED {
		t.Errorf("request state = %d, expected %d", req.State, proto.STATE_SUSPENDED)
	}

	// Make sure SJC was saved in db.
	ctx := context.TODO()
	conn, err := dbc.Open(ctx)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	defer dbc.Close(conn)

	var actualSJC proto.SuspendedJobChain
	var rawSJC []byte
	q := "SELECT suspended_job_chain FROM suspended_job_chains WHERE request_id = ?"
	if err := conn.QueryRowContext(ctx, q, reqId).Scan(&rawSJC); err != nil {
		switch err {
		case sql.ErrNoRows:
			t.Error("suspended job chain not saved in db")
		default:
			t.Errorf("err = %s, expected nil", err)
		}
	}

	// Unmarshal sjc and check.
	if err := json.Unmarshal(rawSJC, &actualSJC); err != nil {
		t.Errorf("cannot unmarshal suspended job chain: %s", err)
	}
	if diff := deep.Equal(actualSJC, sjc); diff != nil {
		t.Error(diff)
	}
}

// Try suspending a request that isn't running - should fail.
func TestSuspendNotRunning(t *testing.T) {
	dbName := setupResumer(t, rmtest.DataPath+"/request-default.sql")
	defer teardownResumer(t, dbName)

	reqId := "93ec156e204ety45sgf0" // request is complete
	jobTries := map[string]uint{"job1": 2, "job2": 2}
	stoppedJobTries := map[string]uint{"job2": 1}
	sequenceRetries := map[string]uint{"job1": 1}
	sjc := proto.SuspendedJobChain{
		RequestId:         reqId,
		JobChain:          &proto.JobChain{},
		TotalJobTries:     jobTries,
		LatestRunJobTries: stoppedJobTries,
		SequenceTries:     sequenceRetries,
	}

	cfg := request.ResumerConfig{
		RequestManager: rm,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		RMHost:         "hostname",
		ShutdownChan:   shutdownChan,
		ResumeInterval: request.ResumeInterval,
	}
	r := request.NewResumer(cfg)
	err := r.Suspend(reqId, sjc)
	switch err.(type) {
	case request.ErrInvalidState:
	default:
		t.Errorf("error = %s, expected %s", err, request.NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[proto.STATE_PENDING]))
	}
}

func TestResume(t *testing.T) {
	dbName := setupResumer(t, rmtest.DataPath+"/request-default.sql")
	defer teardownResumer(t, dbName)

	receivedSJCs := make(map[string]proto.SuspendedJobChain)
	jrc := &mock.JRClient{
		ResumeJobChainFunc: func(sjc proto.SuspendedJobChain) (string, error) {
			if sjc.RequestId == "old_sjc_____________" {
				return "", mock.ErrJRClient
			}
			receivedSJCs[sjc.RequestId] = sjc
			return "returned_by_jr", nil
		},
	}

	cfg := request.ResumerConfig{
		RequestManager: rm,
		DBConnector:    dbc,
		JRClient:       jrc,
		RMHost:         "hostname",
		ShutdownChan:   shutdownChan,
		ResumeInterval: 100 * time.Millisecond,
	}
	r := request.NewResumer(cfg)

	// Run the resumer. Shut it down after a while, so it doesn't keep running forever.
	runDone := make(chan struct{})
	go func() {
		r.Run()
		close(runDone)
	}()
	time.Sleep(500 * time.Millisecond)
	close(shutdownChan)
	<-runDone

	// Expected to happen:
	//  suspended___________: sent to JR, req = Running, + SJC deleted
	//  abandoned_sjc_______: SJC unclaimed, then resumed (sent to JR, Running, SJC deleted)
	//  running_abandoned___: SJC unclaimed, then SJC deleted
	//  running_with_old_sjc: SJC deleted
	//  old_sjc_____________: try to resume (forced fail), delete SJC, req = Failed
	//  no_sjc______________: req = Failed

	// Check SJC sent correctly to JR.
	expectedSJCs := map[string]proto.SuspendedJobChain{
		"suspended___________": testdb.SavedSJCs["suspended___________"],
		"abandoned_sjc_______": testdb.SavedSJCs["abandoned_sjc_______"],
	}
	if diff := deep.Equal(receivedSJCs, expectedSJCs); diff != nil {
		t.Error(diff)
	}

	// Check SJCs present in db.
	ctx := context.TODO()
	conn, err := dbc.Open(ctx)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	defer dbc.Close(conn)

	var count int
	q := "SELECT COUNT(*) FROM suspended_job_chains"
	err = conn.QueryRowContext(ctx, q).Scan(&count)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}
	if count != 0 {
		t.Errorf("expected all SJCs to be deleted - %d still in db", count)
	}

	// Check request states + JR Host.
	expectedStates := map[string]byte{
		"suspended___________": proto.STATE_RUNNING,
		"running_with_old_sjc": proto.STATE_RUNNING,
		"abandoned_sjc_______": proto.STATE_RUNNING,
		"running_abandoned___": proto.STATE_RUNNING,
		"old_sjc_____________": proto.STATE_FAIL,
		"no_sjc______________": proto.STATE_FAIL,
	}
	for reqId, _ := range testdb.SavedSJCs {
		req, err := rm.Get(reqId)
		if err != nil {
			t.Errorf("err = %s, expected nil", err)
			return
		}

		if req.State != expectedStates[reqId] {
			t.Errorf("request state = %d, expected %d", req.State, expectedStates[reqId])
		}

		if req.Id == "suspended___________" {
			if req.JobRunnerHost == nil {
				t.Errorf("request JR host = nil, expected %s", "returned_by_jr")
			} else if *req.JobRunnerHost != "returned_by_jr" {
				t.Errorf("request JR host = %s, expected %s", *req.JobRunnerHost, "returned_by_jr")
			}
		}
	}
}
