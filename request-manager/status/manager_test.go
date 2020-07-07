// Copyright 2017-2019, Square, Inc.

package status_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/status"
	rmtest "github.com/square/spincycle/v2/request-manager/test"
	testdb "github.com/square/spincycle/v2/request-manager/test/db"
	"github.com/square/spincycle/v2/test/mock"
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

func TestRunningJobRetried(t *testing.T) {
	// A test copied from rm/manager_test.go when RM did status stuff:
	//   Bug fix: RM does not report live status for retried jobs. Problem is:
	//   RM uses job log and a retried job will have a JLE which masks its
	//   realtime status from the JR. So for this test, we need a JLE where the
	//   job failed + JR still reporting job on 2nd+ try.
	// This shouldn't be an issue in status.Manager because it works differently,
	// avoiding this problem. So this test is realy just a basic Running() test.
	reqId := "aaabbbcccdddeeefff00" // used in this data file:
	dbName := setup(t, rmtest.DataPath+"/retry-job-live-status.sql")
	defer teardown(t, dbName)

	// @todo: the JobStatus fields are inconsistent (i.e. which are included or not)
	//        because it depends on whether info comes from JR, JL, or neither and is implied.
	job1Status := proto.JobStatus{
		RequestId: reqId,
		JobId:     "0001",
		Name:      "job1Name",
		State:     proto.STATE_RUNNING, // =2
		Status:    "2nd try...",
		//Type:      "job1Type",  // currently, we don't fetch these cols
		//StartedAt: 1551711877191506000,
	}
	job2Status := proto.JobStatus{
		JobId: "0002",
		//Name:  "job2Name",
		State: proto.STATE_PENDING,
	}
	job3Status := proto.JobStatus{
		JobId: "0003",
		//Name:  "job3Name",
		State: proto.STATE_PENDING,
	}

	mockJRC := &mock.JRClient{
		RunningFunc: func(baseURL string, f proto.StatusFilter) ([]proto.JobStatus, error) {
			t.Logf("RunningFunc: url = %s", baseURL)
			return []proto.JobStatus{job1Status, job2Status, job3Status}, nil
		},
	}

	m := status.NewManager(dbc, mockJRC)
	got, err := m.Running(proto.StatusFilter{})
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := time.Parse("2006-01-02 15:04:05", "2019-03-04 00:00:00")
	expect := proto.RunningStatus{
		Requests: map[string]proto.Request{
			reqId: proto.Request{
				Id:           reqId,
				Type:         "req-name",
				State:        proto.STATE_RUNNING,
				User:         "finch",
				CreatedAt:    ts,
				StartedAt:    &ts,
				TotalJobs:    3,
				FinishedJobs: 0,
			},
		},
		Jobs: []proto.JobStatus{job1Status, job2Status, job3Status},
	}
	if diff := deep.Equal(got, expect); diff != nil {
		t.Error(diff)
	}
}
