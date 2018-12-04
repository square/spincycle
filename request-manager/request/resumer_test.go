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

	// Try suspending a request that's running (should succeed)
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
	}
	r := request.NewResumer(cfg)
	err := r.Suspend(sjc)
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

	// Try suspending a request that isn't running (should fail).
	reqId = "93ec156e204ety45sgf0" // request is complete
	jobTries = map[string]uint{"job1": 2, "job2": 2}
	stoppedJobTries = map[string]uint{"job2": 1}
	sequenceRetries = map[string]uint{"job1": 1}
	sjc = proto.SuspendedJobChain{
		RequestId:         reqId,
		JobChain:          &proto.JobChain{},
		TotalJobTries:     jobTries,
		LatestRunJobTries: stoppedJobTries,
		SequenceTries:     sequenceRetries,
	}

	err = r.Suspend(sjc)
	switch err.(type) {
	case request.ErrInvalidState:
	default:
		t.Errorf("error = %s, expected %s", err, request.NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[proto.STATE_PENDING]))
	}
}

func TestResume(t *testing.T) {
	dbName := setupResumer(t, rmtest.DataPath+"/request-default.sql")
	defer teardownResumer(t, dbName)

	ctx := context.TODO()
	conn, err := dbc.Open(ctx)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	defer dbc.Close(conn)

	var receivedSJC proto.SuspendedJobChain
	jrc := &mock.JRClient{
		ResumeJobChainFunc: func(sjc proto.SuspendedJobChain) (string, error) {
			receivedSJC = sjc
			return "returned_by_jr", nil
		},
	}

	cfg := request.ResumerConfig{
		RequestManager: rm,
		DBConnector:    dbc,
		JRClient:       jrc,
		RMHost:         "hostname",
		ShutdownChan:   shutdownChan,
	}
	r := request.NewResumer(cfg)

	// 1: Test successfully resuming an SJC.
	// Fake claiming the SJC, so Resume works right.
	id := "suspended___________"
	q := "UPDATE suspended_job_chains SET rm_host = ? WHERE request_id = ?"
	_, err = conn.ExecContext(ctx, q, "hostname", id)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}

	err = r.Resume(id)
	if err != nil {
		t.Errorf("got error resuming request: %s", err)
	}

	// Make sure SJC is sent to JR.
	if diff := deep.Equal(testdb.SavedSJCs[id], receivedSJC); diff != nil {
		t.Error(diff)
	}

	// Make sure SJC is deleted from DB.
	var count int
	q = "SELECT COUNT(*) FROM suspended_job_chains WHERE request_id = ?"
	err = conn.QueryRowContext(ctx, q, id).Scan(&count)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}
	if count != 0 {
		t.Errorf("expected SJC %s to be deleted but is still in db", id)
	}

	// Check request state + JR host were updated.
	req, err := rm.Get(id)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if req.State != proto.STATE_RUNNING {
		t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], "RUNNING")
	}

	if req.JobRunnerURL != "returned_by_jr" {
		t.Errorf("request %s JR url = %s, expected %s", req.Id, req.JobRunnerURL, "returned_by_jr")
	}

	// 2: Test resuming an SJC whose request state != Suspended. This should delete
	//    the SJC, without sending it to the JR.
	// Fake claiming the SJC, so Resume works right.
	receivedSJC = proto.SuspendedJobChain{}
	id = "running_with_old_sjc"
	q = "UPDATE suspended_job_chains SET rm_host = ? WHERE request_id = ?"
	_, err = conn.ExecContext(ctx, q, "hostname", id)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}

	err = r.Resume(id)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Make sure SJC was NOT sent to JR.
	if diff := deep.Equal(proto.SuspendedJobChain{}, receivedSJC); diff != nil {
		t.Error(diff)
	}

	// Make sure SJC is deleted from DB.
	q = "SELECT COUNT(*) FROM suspended_job_chains WHERE request_id = ?"
	err = conn.QueryRowContext(ctx, q, id).Scan(&count)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}
	if count != 0 {
		t.Errorf("expected SJC %s to be deleted but is still in db", id)
	}

	// 3: Test resuming an SJC and getting an error from the JR Client.
	// Create a new resumer so we can force a JRC error.
	jrc = &mock.JRClient{
		ResumeJobChainFunc: func(sjc proto.SuspendedJobChain) (string, error) {
			return "", mock.ErrJRClient
		},
	}

	cfg = request.ResumerConfig{
		RequestManager: rm,
		DBConnector:    dbc,
		JRClient:       jrc,
		RMHost:         "hostname",
		ShutdownChan:   shutdownChan,
	}
	r = request.NewResumer(cfg)

	id = "old_sjc_____________"
	// Fake claiming the SJC so Resume works right
	q = "UPDATE suspended_job_chains SET rm_host = ? WHERE request_id = ?"
	_, err = conn.ExecContext(ctx, q, "hostname", id)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}

	err = r.Resume(id)
	if err == nil {
		t.Errorf("didn't get an error but expected one")
	}

	// Make sure SJC was NOT sent to JR.
	if diff := deep.Equal(proto.SuspendedJobChain{}, receivedSJC); diff != nil {
		t.Error(diff)
	}

	// Make sure SJC is NOT deleted from DB.
	q = "SELECT COUNT(*) FROM suspended_job_chains WHERE request_id = ?"
	err = conn.QueryRowContext(ctx, q, id).Scan(&count)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}
	if count != 1 {
		t.Errorf("SJC %s was deleted from the db", id)
	}

	// Check request state was NOT updated.
	req, err = rm.Get(id)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
		return
	}

	if req.State != proto.STATE_SUSPENDED {
		t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], "RUNNING")
	}
}

func TestResumeAll(t *testing.T) {
	dbName := setupResumer(t, rmtest.DataPath+"/request-default.sql")
	defer teardownResumer(t, dbName)

	receivedSJCs := map[string]proto.SuspendedJobChain{}
	jrc := &mock.JRClient{
		ResumeJobChainFunc: func(sjc proto.SuspendedJobChain) (string, error) {
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
	}
	r := request.NewResumer(cfg)

	r.ResumeAll()

	// Expectation for each SJC:
	//  suspended___________: resumed (sent to JR, req = RUNNING, + SJC deleted)
	//  abandoned_sjc_______: nothing
	//  running_abandoned___: nothing
	//  running_with_old_sjc: SJC deleted
	//  old_sjc_____________: resumed (sent to JR, req = RUNNING, + SJC deleted)

	// Check SJC sent correctly to JR.
	expectedSJCs := map[string]proto.SuspendedJobChain{
		"suspended___________": testdb.SavedSJCs["suspended___________"],
		"old_sjc_____________": testdb.SavedSJCs["old_sjc_____________"],
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

	q := "SELECT request_id FROM suspended_job_chains"
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}

	sjcs := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Errorf("error scanning rows: %s", err)
			return
		}

		sjcs[id] = true
	}
	expectedDBSJCs := map[string]bool{
		"abandoned_sjc_______": true,
		"running_abandoned___": true,
		"abandoned_old_sjc___": true,
	}
	if diff := deep.Equal(sjcs, expectedDBSJCs); diff != nil {
		t.Error(diff)
	}

	// Check request states + JR Host.
	expectedStates := map[string]byte{
		"suspended___________": proto.STATE_RUNNING,
		"running_with_old_sjc": proto.STATE_RUNNING,
		"abandoned_sjc_______": proto.STATE_SUSPENDED,
		"running_abandoned___": proto.STATE_RUNNING,
		"old_sjc_____________": proto.STATE_RUNNING,
		"abandoned_old_sjc___": proto.STATE_SUSPENDED,
	}
	for reqId, _ := range testdb.SavedSJCs {
		req, err := rm.Get(reqId)
		if err != nil {
			t.Errorf("err = %s, expected nil", err)
			return
		}

		if req.State != expectedStates[reqId] {
			t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], proto.StateName[expectedStates[reqId]])
		}

		if req.Id == "suspended___________" || req.Id == "old_sjc_____________" {
			if req.JobRunnerURL != "returned_by_jr" {
				t.Errorf("request %s JR url = %s, expected %s", req.Id, req.JobRunnerURL, "returned_by_jr")
			}
		}
	}
}

func TestCleanup(t *testing.T) {
	dbName := setupResumer(t, rmtest.DataPath+"/request-default.sql")
	defer teardownResumer(t, dbName)

	cfg := request.ResumerConfig{
		RequestManager:       rm,
		DBConnector:          dbc,
		JRClient:             &mock.JRClient{},
		RMHost:               "hostname",
		ShutdownChan:         shutdownChan,
		SuspendedJobChainTTL: time.Hour,
	}
	r := request.NewResumer(cfg)

	r.Cleanup()

	// Expectation for each SJC:
	//  suspended___________: nothing
	//  abandoned_sjc_______: unclaimed
	//  running_abandoned___: unclaimed
	//  running_with_old_sjc: SJC deleted
	//  old_sjc_____________: SJC deleted, req set to FAILED
	//  abandoned_old_sjc___: unclaimed, then deleted

	// Check SJCs present in db.// Check SJCs present in db.
	ctx := context.TODO()
	conn, err := dbc.Open(ctx)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	defer dbc.Close(conn)

	q := "SELECT request_id, rm_host FROM suspended_job_chains"
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}

	sjcsClaimed := map[string]bool{}
	for rows.Next() {
		var id string
		var rmHost sql.NullString
		if err := rows.Scan(&id, &rmHost); err != nil {
			t.Errorf("error scanning rows: %s", err)
			return
		}

		sjcsClaimed[id] = rmHost.Valid
	}
	expectedSJCClaims := map[string]bool{
		"suspended___________": false,
		"abandoned_sjc_______": false,
		"running_abandoned___": false,
	}
	if diff := deep.Equal(sjcsClaimed, expectedSJCClaims); diff != nil {
		t.Error(diff)
	}

	// Check request states
	req, err := rm.Get("running_with_old_sjc")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
		return
	}
	if req.State != proto.STATE_RUNNING { // not set to FAIL because was RUNNING
		t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], "RUNNING")
	}

	req, err = rm.Get("old_sjc_____________")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
		return
	}
	if req.State != proto.STATE_FAIL {
		t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], "FAIL")
	}

	req, err = rm.Get("abandoned_old_sjc___")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
		return
	}
	if req.State != proto.STATE_FAIL {
		t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], "FAIL")
	}

	r.Cleanup()

	// Expectation for each SJC:
	//  suspended___________: nothing
	//  abandoned_sjc_______: nothing
	//  running_abandoned___: nothing
	//  abandoned_old_sjc___: SJC deleted + req set to FAILED

	// Check SJCs present in db.
	q = "SELECT request_id FROM suspended_job_chains"
	rows, err = conn.QueryContext(ctx, q)
	if err != nil {
		t.Errorf("error querying db: %s", err)
		return
	}

	sjcsPresent := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Errorf("error scanning rows: %s", err)
			return
		}

		sjcsPresent[id] = true
	}
	expectedSJCs := map[string]bool{
		"suspended___________": true,
		"abandoned_sjc_______": true,
		"running_abandoned___": true,
	}
	if diff := deep.Equal(sjcsPresent, expectedSJCs); diff != nil {
		t.Error(diff)
	}

	// Check request state.
	req, err = rm.Get("abandoned_old_sjc___")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
		return
	}
	if req.State != proto.STATE_FAIL {
		t.Errorf("request %s state = %s, expected %s", req.Id, proto.StateName[req.State], "FAIL")
	}
}
