// Copyright 2017, Square, Inc.

package request

import (
	"database/sql"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/go-test/deep"
	"github.com/square/spincycle/config"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/util"
)

var db *sql.DB
var dbA DBAccessor

func setup(t *testing.T) {
	// Get db connection info from the test config.
	var cfg config.RequestManager
	cfgFile := "../config/test.yaml"
	err := config.Load(cfgFile, &cfg)
	if err != nil {
		t.Fatalf("error loading config at %s: %s", cfgFile, err)
	}

	dsn := cfg.Db.DSN + "?parseTime=true"

	// Parse the dsn since we need to access the individual fields below.
	dsnConfig, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("error parsing dsn: %s", err)
	}
	addrSplit := strings.Split(dsnConfig.Addr, ":")
	var host string
	if len(addrSplit) > 0 {
		host = addrSplit[0]
	}

	// Create a fresh db.
	cmd := exec.Command("mysql", "-h", host, "-u", dsnConfig.User,
		"-e", "DROP DATABASE IF EXISTS "+dsnConfig.DBName+"; "+
			"CREATE DATABASE "+dsnConfig.DBName, "--password="+dsnConfig.Passwd)
	if _, err = cmd.Output(); err != nil {
		t.Fatalf("error creating a fresh db: %s", err)
	}

	// Source the schema.
	schemaFile, _ := filepath.Abs("../resources/request_manager_schema.sql")
	// todo: support all params that could be in the dsn (ex: port).
	cmd = exec.Command("mysql", "-h", host, "-u", dsnConfig.User,
		dsnConfig.DBName, "-e", "SOURCE "+schemaFile, "--password="+dsnConfig.Passwd)
	if _, err = cmd.Output(); err != nil {
		t.Fatalf("error sourcing schema: %s", err)
	}

	// Open the db.
	db, err = sql.Open(cfg.Db.Type, dsn)
	if err != nil {
		t.Fatalf("error opening sql db: %s", err)
	}
	if err = db.Ping(); err != nil {
		t.Fatalf("error connecting to sql db: %s", err)
	}

	dbA = NewDBAccessor(db)
}

func cleanup() {
	db.Close()
	db = nil
	dbA = nil
}

func getCount(table string) (int, error) {
	rows, err := db.Query("SELECT COUNT(*) AS COUNT FROM " + table)
	if err != nil {
		return 0, err
	}

	var count int
	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}

	}
	return count, nil
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestSaveRequestRollback(t *testing.T) {
	setup(t)
	defer cleanup()

	reqId := util.UUID() // don't change this
	req := proto.Request{
		Id: reqId,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			State:     proto.STATE_COMPLETE,
		},
	}
	reqParams := proto.CreateRequestParams{Type: "something"}

	// Inserting this record will cause a duplicate key insert error below.
	if _, err := db.Exec("INSERT INTO raw_requests (request_id) VALUES (?)",
		reqId); err != nil {
		t.Fatal(err)
	}

	err := dbA.SaveRequest(req, reqParams)
	if err == nil {
		t.Errorf("expected an error, did not get one") // duplicate key error
	}

	// Get the # of records in the requests and raw_requests tables.
	rCount, err := getCount("requests")
	if err != nil {
		t.Fatal(err)
	}
	rrCount, err := getCount("raw_requests")
	if err != nil {
		t.Fatal(err)
	}

	// Check that the record count matches what we expect.
	if rCount != 0 {
		t.Errorf("rows in requests table = %d, expected 0", rCount)
	}
	if rrCount != 1 {
		t.Errorf("rows in raw requests table = %d, expected 1", rrCount)
	}
}

func TestSaveGetRequest(t *testing.T) {
	setup(t)
	defer cleanup()

	reqId := util.UUID()
	req := proto.Request{
		Id:    reqId,
		State: proto.STATE_RUNNING,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			State:     proto.STATE_RUNNING,
		},
	}
	reqParams := proto.CreateRequestParams{Type: "something"}

	// Create a request in the db.
	err := dbA.SaveRequest(req, reqParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get the jobchain from the db.
	gotJc, err := dbA.GetJobChain(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(gotJc, *req.JobChain); diff != nil {
		t.Error(diff)
	}

	// Get the request from the db.
	gotReq, err := dbA.GetRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	req.JobChain = nil // will always be nil on struct returned by dbA
	if diff := deep.Equal(gotReq, req); diff != nil {
		t.Error(diff)
	}

	// Get a non-existant request from the db.
	_, err = dbA.GetRequest("efgh5678")
	if err == nil {
		t.Errorf("expected an error but did not get one")
	} else {
		switch err.(type) {
		case ErrNotFound: // this what we expect
		default:
			t.Errorf("err = %s, expected %s", err, reflect.TypeOf(ErrNotFound{}))
		}
	}
}

func TestUpdateRequest(t *testing.T) {
	setup(t)
	defer cleanup()

	reqId := util.UUID()
	req := proto.Request{
		Id:       reqId,
		State:    proto.STATE_RUNNING,
		JobChain: &proto.JobChain{},
	}
	reqParams := proto.CreateRequestParams{Type: "something"}

	// Create a request in the db.
	err := dbA.SaveRequest(req, reqParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Update the request.
	req.State = proto.STATE_COMPLETE
	// If we don't round here, tests will sometimes fail with records in the db
	// being 1 second off from expectations.
	now := time.Now().Round(time.Second)
	req.FinishedAt = &now
	err = dbA.UpdateRequest(req)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get the request from the db.
	gotReq, err := dbA.GetRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if gotReq.State != req.State {
		t.Errorf("request state = %d, expected %d", gotReq.State, req.State)
	}
	if gotReq.FinishedAt == nil {
		t.Errorf("finished at = null, expected %d", req.FinishedAt)
	} else {
		if gotReq.FinishedAt.Unix() != now.Unix() {
			t.Errorf("finished at = %d, expected %d",
				gotReq.FinishedAt.Unix(), now.Unix())
		}
	}

	// Update a non-existant request from the db.
	req.Id = "abcd1234"
	err = dbA.UpdateRequest(req)
	if err != ErrNotUpdated {
		t.Errorf("err = %s, expected %s", err, ErrNotUpdated)
	}
}

func TestIncrementRequestFinishedJobs(t *testing.T) {
	setup(t)
	defer cleanup()

	reqId := util.UUID()
	req := proto.Request{
		Id:       reqId,
		State:    proto.STATE_RUNNING,
		JobChain: &proto.JobChain{},
	}
	reqParams := proto.CreateRequestParams{Type: "something"}

	// Create a request in the db.
	err := dbA.SaveRequest(req, reqParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Increment the finished jobs counter on the request.
	for i := 1; i <= 5; i++ {
		err = dbA.IncrementRequestFinishedJobs(reqId)
		if err != nil {
			t.Errorf("err = %s, expected nil", err)
		}
	}

	// Get the request from the db.
	gotReq, err := dbA.GetRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedCount := 5
	if gotReq.FinishedJobs != expectedCount {
		t.Errorf("finished jobs = %d, expected %d", gotReq.FinishedJobs, expectedCount)
	}

	// Increment the finished jobs for a non-existant request.
	err = dbA.IncrementRequestFinishedJobs("abcd1234")
	if err != ErrNotUpdated {
		t.Errorf("err = %s, expected %s", err, ErrNotUpdated)
	}
}

func TestCreateGetJL(t *testing.T) {
	setup(t)
	defer cleanup()

	reqId := util.UUID()
	jobId := "job1"
	jl := proto.JobLog{
		RequestId: reqId,
		JobId:     jobId,
		Try:       1,
	}

	// Create a jl in the db.
	err := dbA.SaveJL(jl)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	jl = proto.JobLog{
		RequestId: reqId,
		JobId:     jobId,
		Try:       2,
	}

	// Create a second jl in the db.
	err = dbA.SaveJL(jl)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get the jl from the db.
	gotjl, err := dbA.GetLatestJL(reqId, jobId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(gotjl, jl); diff != nil {
		t.Error(diff)
	}
}

func TestGetRequestJobStatuses(t *testing.T) {
	setup(t)
	defer cleanup()

	reqId1 := util.UUID()
	jobId := "job1"
	jl1 := proto.JobLog{
		RequestId: reqId1,
		JobId:     jobId,
		State:     proto.STATE_FAIL,
		Try:       1, // first attempt
	}
	jl2 := proto.JobLog{
		RequestId: reqId1,
		JobId:     jobId,
		State:     proto.STATE_COMPLETE,
		Try:       2, // second attempt
	}
	jobId = "job2"
	jl3 := proto.JobLog{
		RequestId: reqId1,
		JobId:     jobId,
		State:     proto.STATE_FAIL,
		Try:       1, // first attempt
	}
	jl4 := proto.JobLog{
		RequestId: reqId1,
		JobId:     jobId,
		State:     proto.STATE_FAIL,
		Try:       2, // second attempt
	}
	jobId = "job3"
	jl5 := proto.JobLog{
		RequestId: reqId1,
		JobId:     jobId,
		State:     proto.STATE_FAIL,
		Try:       1,
	}
	reqId2 := util.UUID() // this one has a different request id
	jobId = "job4"
	jl6 := proto.JobLog{
		RequestId: reqId2,
		JobId:     jobId,
		State:     proto.STATE_FAIL,
		Try:       1,
	}
	jls := []proto.JobLog{jl1, jl2, jl3, jl4, jl5, jl6}

	// Create the jls in the db.
	for _, jl := range jls {
		err := dbA.SaveJL(jl)
		if err != nil {
			t.Errorf("err = %s, expected nil", err)
		}
	}

	// Get the status of all jobs.
	s, err := dbA.GetRequestJobStatuses(reqId1)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedS := proto.JobStatuses{
		proto.JobStatus{Id: "job1", State: proto.STATE_COMPLETE},
		proto.JobStatus{Id: "job2", State: proto.STATE_FAIL},
		proto.JobStatus{Id: "job3", State: proto.STATE_FAIL},
	}

	sort.Sort(s)
	sort.Sort(expectedS)

	if diff := deep.Equal(s, expectedS); diff != nil {
		t.Error(diff)
	}
}
