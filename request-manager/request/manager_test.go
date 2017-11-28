// Copyright 2017, Square, Inc.

package request_test

import (
	"database/sql"
	"fmt"
	"sort"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/request-manager/request"
	rmtest "github.com/square/spincycle/request-manager/test"
	testdb "github.com/square/spincycle/request-manager/test/db"
	"github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

var dbm testdb.Manager
var dbc db.Connector
var grf *grapher.MockGrapherFactory
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

	// Create a mock connector the connects to the test db.
	dbc = &mock.Connector{
		ConnectFunc: func() (*sql.DB, error) {
			return dbm.Connect(dbName)
		},
	}

	// Create a mock grapher factory.
	if grf == nil {
		spec, err := grapher.ReadConfig(rmtest.SpecPath + "/a-b-c.yaml")
		if err != nil {
			t.Fatal(err)
		}
		testJobFactory := &mock.JobFactory{
			MockJobs: map[string]*mock.Job{},
		}
		for i, c := range []string{"a", "b", "c"} {
			jobType := c + "JobType"
			testJobFactory.MockJobs[jobType] = &mock.Job{
				NameResp: fmt.Sprintf("%s@%d", c, i),
				TypeResp: jobType,
			}
		}
		testJobFactory.MockJobs["aJobType"].SetJobArgs = map[string]interface{}{
			"aArg": "aValue",
		}
		gr := grapher.NewGrapher(testJobFactory, spec, id.NewGenerator(4, 100))
		grf = &grapher.MockGrapherFactory{
			MakeFunc: func() *grapher.Grapher {
				return gr
			},
		}
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

func TestCreateMissingType(t *testing.T) {
	m := request.NewManager(grf, dbc, &mock.JRClient{})

	_, err := m.Create(proto.CreateRequestParams{})
	if err != request.ErrInvalidParams {
		t.Errorf("err = %s, expected %s", err, request.ErrInvalidParams)
	}
}

func TestCreate(t *testing.T) {
	dbName := setup(t, "")
	defer teardown(t, dbName)

	m := request.NewManager(grf, dbc, &mock.JRClient{})

	// gr uses spec a-b-c.yaml which has reqest "three-nodes"
	reqParams := proto.CreateRequestParams{
		Type: "three-nodes",
		User: "john",
		Args: map[string]interface{}{
			"foo": "foo-value",
		},
	}

	actualReq, err := m.Create(reqParams)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Make sure the returned request is legit.
	if actualReq.Id == "" {
		t.Errorf("request id is an empty string, expected it to be set")
	}
	if actualReq.CreatedAt.IsZero() {
		t.Errorf("request created at is a zero time, should not be")
	}

	// Job names in requests are non-deterministic because the nodes in a sequence
	// are built from a map (i.e. hash order randomness). So sometimes we get a@3
	// and other times a@4, etc. So we'll check some specific, deterministic stuff.
	// But an example of a job chain is shown in the comment block below.
	actualJobChain := actualReq.JobChain
	actualReq.JobChain = nil

	/*
		expectedJc := proto.JobChain{
			RequestId: actualReq.Id, // no other way of getting this from outside the package
			State:     proto.STATE_PENDING,
			Jobs: map[string]proto.Job{
				"sequence_three-nodes_start@1": proto.Job{
					Id:   "sequence_three-nodes_start@1",
					Type: "no-op",
				},
				"a@3": proto.Job{
					Id:        "a@3",
					Type:      "aJobType",
					Retry:     1,
					RetryWait: 500,
				},
				"b@4": proto.Job{
					Id:    "b@4",
					Type:  "bJobType",
					Retry: 3,
				},
				"c@5": proto.Job{
					Id:   "c@5",
					Type: "cJobType",
				},
				"sequence_three-nodes_end@2": proto.Job{
					Id:   "sequence_three-nodes_end@2",
					Type: "no-op",
				},
			},
			AdjacencyList: map[string][]string{
				"sequence_three-nodes_start@1": []string{"a@3"},
				"a@3": []string{"b@4"},
				"b@4": []string{"c@5"},
				"c@5": []string{"sequence_three-nodes_end@2"},
			},
		}
	*/

	expectedReq := proto.Request{
		Id:        actualReq.Id, // no other way of getting this from outside the package
		Type:      reqParams.Type,
		CreatedAt: actualReq.CreatedAt, // same deal as request id
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
		JobChain:  nil,
		TotalJobs: 5,
	}
	if diff := deep.Equal(actualReq, expectedReq); diff != nil {
		test.Dump(actualReq)
		t.Error(diff)
	}

	// Check the job chain
	if actualJobChain.RequestId == "" {
		t.Error("job chain RequestId not set, expected it to be set")
	}
	if actualJobChain.State != proto.STATE_PENDING {
		t.Error("job chain state = %s, expected PENDING", proto.StateName[actualJobChain.State])
	}
	if len(actualJobChain.Jobs) != 5 {
		test.Dump(actualJobChain.Jobs)
		t.Error("job chain has %d jobs, expected 5", len(actualJobChain.Jobs))
	}
	if len(actualJobChain.AdjacencyList) != 4 {
		test.Dump(actualJobChain.Jobs)
		t.Error("job chain AdjacencyList len = %d, expected 4", len(actualJobChain.AdjacencyList))
	}
}

func TestGetNotFound(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "invalid"
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	_, err := m.Get(reqId)
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

func TestGet(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "0874a524aa1e"
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	actual, err := m.Get(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected := testdb.SavedRequests[reqId]
	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}

func TestStartNotPending(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "454ae2f98a05" // request is running
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	err := m.Start(reqId)
	if err != db.ErrNotUpdated {
		t.Errorf("error = %s, expected %s", err, db.ErrNotUpdated)
	}
}

func TestStart(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	// Create a mock JR client that records the JC it receives.
	var recvdJc proto.JobChain
	mockJRc := &mock.JRClient{
		NewJobChainFunc: func(jc proto.JobChain) error {
			recvdJc = jc
			return nil
		},
	}

	reqId := "0874a524aa1e" // request is pending
	m := request.NewManager(grf, dbc, mockJRc)
	err := m.Start(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if diff := deep.Equal(recvdJc, *testdb.SavedRequests[reqId].JobChain); diff != nil {
		t.Error(diff)
	}

	// Get the request from the db and make sure its state was updated.
	req, err := m.Get(reqId)
	if err != nil {
		t.Error(err)
	}

	if req.State != proto.STATE_RUNNING {
		t.Errorf("request state = %d, expected %d", req.State, proto.STATE_RUNNING)
	}
}

func TestStopNotRunning(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "0874a524aa1e" // request is pending
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	err := m.Stop(reqId)
	if err != nil {
		switch v := err.(type) {
		case request.ErrInvalidState:
			break // this is what we expect
		default:
			t.Errorf("error is of type %s, expected request.ErrInvalidState", v)
		}
	} else {
		t.Error("expected an error, did not get one")
	}
}

func TestStopComplete(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	// Create a mock JR client that records the requestId it receives. This shouldn't
	// be hit.
	var recvdId string
	mockJRc := &mock.JRClient{
		StopRequestFunc: func(reqId string) error {
			recvdId = reqId
			return nil
		},
	}

	reqId := "93ec156e204e" // request is running
	m := request.NewManager(grf, dbc, mockJRc)
	err := m.Stop(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if recvdId != "" {
		t.Errorf("request id = %s, expected an empty string", recvdId)
	}
}

func TestStop(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	// Create a mock JR client that records the requestId it receives.
	var recvdId string
	mockJRc := &mock.JRClient{
		StopRequestFunc: func(reqId string) error {
			recvdId = reqId
			return nil
		},
	}

	reqId := "454ae2f98a05" // request is running
	m := request.NewManager(grf, dbc, mockJRc)
	err := m.Stop(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if recvdId != reqId {
		t.Errorf("request id = %s, expected %s", recvdId, reqId)
	}
}

func TestFinishNotRunning(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "0874a524aa1e" // request is pending
	params := proto.FinishRequestParams{
		State: proto.STATE_COMPLETE,
	}
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	err := m.Finish(reqId, params)
	if err != db.ErrNotUpdated {
		t.Errorf("error = %s, expected %s", err, db.ErrNotUpdated)
	}
}

func TestFinish(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "454ae2f98a05" // request is running
	params := proto.FinishRequestParams{
		State: proto.STATE_COMPLETE,
	}
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	err := m.Finish(reqId, params)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Get the request from the db and make sure its state was updated.
	req, err := m.Get(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if req.State != params.State {
		t.Errorf("request state = %d, expected %d", req.State, params.State)
	}
}

func TestStatusRunning(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "454ae2f98a05" // request is running and has JLs

	// Create a mock JR client that returns live status for some jobs.
	mockJRc := &mock.JRClient{
		RequestStatusFunc: func(reqId string) (proto.JobChainStatus, error) {
			return proto.JobChainStatus{
				RequestId: reqId,
				JobStatuses: proto.JobStatuses{
					proto.JobStatus{
						RequestId: reqId,
						JobId:     "ldfi",
						State:     proto.STATE_RUNNING,
						Status:    "in progress",
					},
					// This job is marked as COMPLETE in the database. Therefore,
					// the RM will disregard this status since it's out of date.
					proto.JobStatus{
						RequestId: reqId,
						JobId:     "590s",
						State:     proto.STATE_RUNNING,
						Status:    "will get disregarded",
					},
				},
			}, nil
		},
	}

	m := request.NewManager(grf, dbc, mockJRc)
	actual, err := m.Status(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expected := proto.RequestStatus{
		Request: testdb.SavedRequests[reqId],
		JobChainStatus: proto.JobChainStatus{
			RequestId: reqId,
			JobStatuses: proto.JobStatuses{
				proto.JobStatus{
					RequestId: reqId,
					JobId:     "di12",
					State:     proto.STATE_COMPLETE,
				},
				proto.JobStatus{
					RequestId: reqId,
					JobId:     "ldfi",
					State:     proto.STATE_RUNNING,
					Status:    "in progress",
				},
				proto.JobStatus{
					RequestId: reqId,
					JobId:     "590s",
					State:     proto.STATE_COMPLETE,
				},
				proto.JobStatus{
					RequestId: reqId,
					JobId:     "g012",
					State:     proto.STATE_PENDING,
				},
				proto.JobStatus{
					RequestId: reqId,
					JobId:     "9sa1",
					State:     proto.STATE_FAIL,
				},
				proto.JobStatus{
					RequestId: reqId,
					JobId:     "pzi8",
					State:     proto.STATE_PENDING,
				},
			},
		},
	}
	sort.Sort(actual.JobChainStatus.JobStatuses)
	sort.Sort(expected.JobChainStatus.JobStatuses)

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}

func TestIncrementFinishedJobs(t *testing.T) {
	dbName := setup(t, rmtest.DataPath+"/request-default.sql")
	defer teardown(t, dbName)

	reqId := "454ae2f98a05" // request is running
	m := request.NewManager(grf, dbc, &mock.JRClient{})
	err := m.IncrementFinishedJobs(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Get the request from the db and make sure its FinishedJobs counter was incremented.
	req, err := m.Get(reqId)
	if err != nil {
		t.Error(err)
	}

	expectedCount := testdb.SavedRequests[reqId].FinishedJobs + 1
	if req.FinishedJobs != expectedCount {
		t.Errorf("request FinishedJobs = %d, expected %d", req.FinishedJobs, expectedCount)
	}
}
