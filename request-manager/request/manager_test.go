// Copyright 2017-2019, Square, Inc.

package request_test

import (
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/go-test/deep"

	serr "github.com/square/spincycle/errors"
	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/request-manager/request"
	rmtest "github.com/square/spincycle/request-manager/test"
	testdb "github.com/square/spincycle/request-manager/test/db"
	"github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

var dbm testdb.Manager
var dbc *sql.DB
var grf *grapher.MockGrapherFactory
var dbSuffix string
var shutdownChan chan struct{}
var req proto.Request

func setupManager(t *testing.T, dataFile string) string {
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

	// Create a mock grapher factory.
	if grf == nil {
		spec, err := grapher.ReadConfig(rmtest.SpecPath + "/a-b-c.yaml")
		if err != nil {
			t.Fatal(err)
		}
		testJobFactory := &mock.JobFactory{
			MockJobs: map[string]*mock.Job{},
		}
		req := proto.Request{
			Id:   "reqId1",
			Type: "reqType",
		}
		for i, c := range []string{"a", "b", "c"} {
			jobType := c + "JobType"
			testJobFactory.MockJobs[jobType] = &mock.Job{
				IdResp: job.NewIdWithRequestId(jobType, c, fmt.Sprintf("id%d", i), req.Id),
			}
		}
		testJobFactory.MockJobs["aJobType"].SetJobArgs = map[string]interface{}{
			"aArg": "aValue",
		}
		gr := grapher.NewGrapher(req, testJobFactory, spec, id.NewGenerator(4, 100))
		grf = &grapher.MockGrapherFactory{
			MakeFunc: func(req proto.Request) *grapher.Grapher {
				return gr
			},
		}
	}

	// Create a shutdown channel
	shutdownChan = make(chan struct{})

	return dbName
}

func teardownManager(t *testing.T, dbName string) {
	close(shutdownChan)

	if err := dbm.Destroy(dbName); err != nil {
		t.Fatal(err)
	}
	dbc.Close()
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestCreateMissingType(t *testing.T) {
	shutdownChan := make(chan struct{})
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	defer close(shutdownChan)

	_, err := m.Create(proto.CreateRequest{})
	switch err.(type) {
	case serr.ErrInvalidCreateRequest:
	default:
		t.Errorf("err = %s, expected request.ErrInvalidCreateRequest type", err)
	}
}

func TestCreate(t *testing.T) {
	dbName := setupManager(t, "")
	defer teardownManager(t, dbName)

	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)

	// gr uses spec a-b-c.yaml which has reqest "three-nodes"
	reqParams := proto.CreateRequest{
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
					SequenceId: "sequence_three-nodes_start@1",
					SequenceRetry: 2,
				},
				"a@3": proto.Job{
					Id:        "a@3",
					Type:      "aJobType",
					Retry:     1,
					RetryWait: 500,
					SequenceId: "sequence_three-nodes_start@1",
					SequenceRetry: 0,
				},
				"b@4": proto.Job{
					Id:    "b@4",
					Type:  "bJobType",
					Retry: 3,
					SequenceId: "sequence_three-nodes_start@1",
					SequenceRetry: 0,
				},
				"c@5": proto.Job{
					Id:   "c@5",
					Type: "cJobType",
					SequenceId: "sequence_three-nodes_start@1",
					SequenceRetry: 0,
				},
				"sequence_three-nodes_end@2": proto.Job{
					Id:   "sequence_three-nodes_end@2",
					Type: "no-op",
					SequenceId: "sequence_three-nodes_start@1",
					SequenceRetry: 0,
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

	for _, job := range actualJobChain.Jobs {
		if job.State != proto.STATE_PENDING {
			t.Errorf("job %s has state %s, expected all jobs to be STATE_PENDING", job.Id, proto.StateName[job.State])
		}
	}

	expectedReq := proto.Request{
		Id:        actualReq.Id, // no other way of getting this from outside the package
		Type:      reqParams.Type,
		CreatedAt: actualReq.CreatedAt, // same deal as request id
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
		JobChain:  nil,
		TotalJobs: 5,
		Args: []proto.RequestArg{
			{
				Name:  "foo",
				Type:  proto.ARG_TYPE_REQUIRED,
				Value: "foo-value",
				Given: true,
			},
			{
				Name:    "bar",
				Type:    proto.ARG_TYPE_OPTIONAL,
				Default: "175",
				Value:   "175",
				Given:   false,
			},
			//"aArg": "aValue", // job arg, not request arg
		},
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
		t.Errorf("job chain state = %s, expected PENDING", proto.StateName[actualJobChain.State])
	}
	if len(actualJobChain.Jobs) != 5 {
		test.Dump(actualJobChain.Jobs)
		t.Errorf("job chain has %d jobs, expected 5", len(actualJobChain.Jobs))
	}
	if len(actualJobChain.AdjacencyList) != 4 {
		test.Dump(actualJobChain.Jobs)
		t.Errorf("job chain AdjacencyList len = %d, expected 4", len(actualJobChain.AdjacencyList))
	}
}

func TestGetNotFound(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "invalid"
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	_, err := m.Get(reqId)
	if err != nil {
		switch v := err.(type) {
		case serr.RequestNotFound:
			break // this is what we expect
		default:
			t.Errorf("error is of type %s, expected serr.RequestNotFound", v)
		}
	} else {
		t.Error("expected an error, did not get one")
	}
}

func TestGet(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "0874a524aa1edn3ysp00"
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	actual, err := m.Get(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected := testdb.SavedRequests[reqId]
	expected.JobChain = nil // expect request without JC
	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}

func TestGetWithJC(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "0874a524aa1edn3ysp00"
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	actual, err := m.GetWithJC(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected := testdb.SavedRequests[reqId]
	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}

func TestStartNotPending(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "454ae2f98a05cv16sdwt" // request is running
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	err := m.Start(reqId)
	_, ok := err.(serr.ErrInvalidState)
	if !ok {
		t.Errorf("error = %s, expected %s", err, serr.NewErrInvalidState(proto.StateName[proto.STATE_PENDING], proto.StateName[proto.STATE_RUNNING]))
	}
}

func TestStart(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	// Create a mock JR client that records the JC it receives.
	var recvdJc proto.JobChain
	mockJRc := &mock.JRClient{
		NewJobChainFunc: func(baseURL string, jc proto.JobChain) (*url.URL, error) {
			recvdJc = jc
			url, _ := url.Parse("http://fake_host:1111/api/v1/job-chains/1")
			return url, nil
		},
	}

	reqId := "0874a524aa1edn3ysp00" // request is pending
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       mockJRc,
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
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
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "0874a524aa1edn3ysp00" // request is pending
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	err := m.Stop(reqId)
	if err != nil {
		switch v := err.(type) {
		case serr.ErrInvalidState:
			break // this is what we expect
		default:
			t.Errorf("error is of type %s, expected serr.ErrInvalidState", v)
		}
	} else {
		t.Error("expected an error, did not get one")
	}
}

func TestStopComplete(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	// Create a mock JR client that records the requestId it receives. This shouldn't
	// be hit.
	var recvdId string
	mockJRc := &mock.JRClient{
		StopRequestFunc: func(baseURL string, reqId string) error {
			recvdId = reqId
			return nil
		},
	}

	reqId := "93ec156e204ety45sgf0" // request is complete
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       mockJRc,
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	err := m.Stop(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if recvdId != "" {
		t.Errorf("request id = %s, expected an empty string", recvdId)
	}

}

func TestStop(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	// Create a mock JR client that records the requestId it receives.
	var recvdId string
	var recvdHost string
	mockJRc := &mock.JRClient{
		StopRequestFunc: func(baseURL string, reqId string) error {
			recvdId = reqId
			recvdHost = baseURL
			return nil
		},
	}

	reqId := "454ae2f98a05cv16sdwt" // request is running
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       mockJRc,
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	err := m.Stop(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if recvdId != reqId {
		t.Errorf("request id = %s, expected %s", recvdId, reqId)
	}
	req := testdb.SavedRequests[reqId]
	if recvdHost != req.JobRunnerURL {
		t.Errorf("JR url = %s, expected %s", recvdHost, req.JobRunnerURL)
	}
}

func TestFinishNotRunning(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "0874a524aa1edn3ysp00" // request is pending
	params := proto.FinishRequest{
		State: proto.STATE_COMPLETE,
	}
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	err := m.Finish(reqId, params)
	switch err.(type) {
	case serr.ErrInvalidState:
	default:
		t.Errorf("error = %s, expected %s", err, serr.NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[proto.STATE_PENDING]))
	}
}

func TestFinish(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "454ae2f98a05cv16sdwt" // request is running
	params := proto.FinishRequest{
		State: proto.STATE_COMPLETE,
	}
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
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
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "454ae2f98a05cv16sdwt" // request is running and has JLs

	// Create a mock JR client that returns live status for some jobs.
	var recvdHost string
	mockJRc := &mock.JRClient{
		RequestStatusFunc: func(baseURL string, reqId string) (proto.JobChainStatus, error) {
			recvdHost = baseURL
			return proto.JobChainStatus{
				RequestId: reqId,
				JobStatuses: proto.JobStatuses{
					proto.JobStatus{
						RequestId: reqId,
						JobId:     "ldfi",
						State:     proto.STATE_RUNNING,
						Status:    "in progress",
					},
				},
			}, nil
		},
	}

	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       mockJRc,
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
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

	req := testdb.SavedRequests[reqId]
	if recvdHost != req.JobRunnerURL {
		t.Errorf("JR url = %s, expected %s", recvdHost, req.JobRunnerURL)
	}
}

func TestStatusJobRetried(t *testing.T) {
	// Bug fix: RM does not report live status for retried jobs. Problem is:
	// RM uses job log and a retried job will have a JLE which masks its
	// realtime status from the JR. So for this test, we need a JLE where the
	// job failed + JR still reporting job on 2nd+ try.
	reqId := "aaabbbcccdddeeefff00" // used in this data file:
	dbName := setupManager(t, rmtest.DataPath+"/retry-job-live-status.sql")
	defer teardownManager(t, dbName)

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

	mockJRc := &mock.JRClient{
		RequestStatusFunc: func(baseURL string, reqId string) (proto.JobChainStatus, error) {
			return proto.JobChainStatus{
				RequestId:   reqId,
				JobStatuses: proto.JobStatuses{job1Status},
			}, nil
		},
	}

	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       mockJRc,
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	actual, err := m.Status(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	actual.JobChain = nil // don't need this
	reqTs, _ := time.Parse("2006-01-02 15:04:05", "2019-03-04 00:00:00")
	expected := proto.RequestStatus{
		Request: proto.Request{
			Id:           reqId,
			Type:         "req-name",
			State:        proto.STATE_RUNNING,
			User:         "finch",
			CreatedAt:    reqTs,
			StartedAt:    &reqTs,
			TotalJobs:    3,
			FinishedJobs: 0,
		},
		JobChainStatus: proto.JobChainStatus{
			RequestId:   reqId,
			JobStatuses: proto.JobStatuses{job1Status, job2Status, job3Status},
		},
	}
	sort.Sort(actual.JobChainStatus.JobStatuses)
	// The bug causes diff: [JobChainStatus.JobStatuses.slice[0].State: 4 != 2 JobChainStatus.JobStatuses.slice[0].Status:  != 2nd try...]
	// Statue 4 is from the job_log when it should be state 2 from the mock JR reporting it's running
	if diff := deep.Equal(actual, expected); diff != nil {
		t.Logf("-- got job statuses: %+v", actual.JobChainStatus.JobStatuses)
		t.Error(diff)
	}
}

func TestIncrementFinishedJobs(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "454ae2f98a05cv16sdwt" // request is running
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
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

func TestJobChainNotFound(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/jc-default.sql")
	defer teardownManager(t, dbName)

	reqId := "invalid"
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	_, err := m.JobChain(reqId)
	if err != nil {
		switch v := err.(type) {
		case serr.RequestNotFound:
			break // this is what we expect
		default:
			t.Errorf("error is of type %s, expected serr.RequestNotFound", v)
		}
	} else {
		t.Error("expected an error, did not get one")
	}
}

func TestJobChainInvalid(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/jc-bad.sql")
	defer teardownManager(t, dbName)

	reqId := "cd724fd12092"
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	_, err := m.JobChain(reqId)
	if err == nil {
		t.Errorf("expected an error unmarshaling the job chain, did not get one")
	}
}

func TestJobChain(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/jc-default.sql")
	defer teardownManager(t, dbName)

	reqId := "8bff5def4f3fvh78skjy"
	cfg := request.ManagerConfig{
		GrapherFactory: grf,
		DBConnector:    dbc,
		JRClient:       &mock.JRClient{},
		ShutdownChan:   shutdownChan,
		DefaultJRURL:   "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	actual, err := m.JobChain(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	if diff := deep.Equal(actual, testdb.SavedJCs[reqId]); diff != nil {
		t.Error(diff)
	}
}
