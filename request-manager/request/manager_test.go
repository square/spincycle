// Copyright 2017-2019, Square, Inc.

package request_test

import (
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/go-test/deep"

	serr "github.com/square/spincycle/v2/errors"
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/request"
	rmtest "github.com/square/spincycle/v2/request-manager/test"
	testdb "github.com/square/spincycle/v2/request-manager/test/db"
	"github.com/square/spincycle/v2/test"
	"github.com/square/spincycle/v2/test/mock"
)

var dbm testdb.Manager
var dbc *sql.DB
var jccf *graph.MockResolverFactory
var dbSuffix string
var shutdownChan chan struct{}
var req proto.Request

var testReqArgs = []proto.RequestArg{
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
}
var testJobChain = &proto.JobChain{
	Jobs: map[string]proto.Job{
		"job1": proto.Job{},
		"job2": proto.Job{},
		"job3": proto.Job{},
		"job4": proto.Job{},
	},
}

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

	// Create a mock creator factory.
	if jccf == nil {
		jccf = &graph.MockResolverFactory{
			MakeFunc: func(req proto.Request) graph.Resolver {
				return &graph.MockResolver{
					RequestArgsFunc: func(jobArgs map[string]interface{}) ([]proto.RequestArg, error) {
						return testReqArgs, nil
					},
					BuildJobChainFunc: func(jobArgs map[string]interface{}) (*proto.JobChain, error) {
						return testJobChain, nil
					},
				}
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)

	// gr uses spec a-b-c.yaml which has request "three-nodes"
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
	actualReq.JobChain = nil

	expectedReq := proto.Request{
		Id:        actualReq.Id, // no other way of getting this from outside the package
		Type:      reqParams.Type,
		CreatedAt: actualReq.CreatedAt, // same deal as request id
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
		JobChain:  nil,
		TotalJobs: uint(len(testJobChain.Jobs)),
		Args:      testReqArgs,
	}
	if diff := deep.Equal(actualReq, expectedReq); diff != nil {
		test.Dump(actualReq)
		t.Error(diff)
	}
}

func TestGetNotFound(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "invalid"
	cfg := request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            mockJRc,
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            mockJRc,
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            mockJRc,
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
	reqId := "454ae2f98a05cv16sdwt"

	cfg := request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)

	// Verify initial request state
	req, err := m.Get(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if req.State != proto.STATE_RUNNING {
		t.Errorf("request state = %s, expected RUNNING", proto.StateName[req.State])
	}
	if req.FinishedJobs != 1 {
		t.Errorf("got FinishedJobs = %d, expected 1", req.FinishedJobs)
	}
	if req.FinishedAt != nil && !req.FinishedAt.IsZero() {
		t.Errorf("got FinishedAt = %s, expected nil/NULL", req.FinishedAt)
	}

	// Send a proto.FinishRequest to finish the request
	now := time.Now()
	params := proto.FinishRequest{
		State:        proto.STATE_COMPLETE,
		FinishedJobs: 3,
		FinishedAt:   now,
	}
	err = m.Finish(reqId, params)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Verify post-finish request state
	req, err = m.Get(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if req.State != params.State {
		t.Errorf("request state = %d, expected %d", req.State, params.State)
	}
	if req.FinishedJobs != params.FinishedJobs {
		t.Errorf("got FinishedJobs = %d, expected %d", req.FinishedJobs, params.FinishedJobs)
	}
	if req.FinishedAt.IsZero() {
		t.Errorf("got FinishedAt = nil/NULL, expected a value")
	}
}

func TestFailNotPending(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	reqId := "454ae2f98a05cv16sdwt" // request is running
	cfg := request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	err := m.FailPending(reqId)
	switch err.(type) {
	case serr.ErrInvalidState:
	default:
		t.Errorf("error = %s, expected %s", err, serr.NewErrInvalidState(proto.StateName[proto.STATE_PENDING], proto.StateName[proto.STATE_RUNNING]))
	}
}

func TestFailPending(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)
	reqId := "0874a524aa1edn3ysp00"

	cfg := request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)

	// Verify initial request state
	req, err := m.Get(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if req.State != proto.STATE_PENDING {
		t.Errorf("request state = %s, expected PENDING", proto.StateName[req.State])
	}
	if req.FinishedJobs != 0 {
		t.Errorf("got FinishedJobs = %d, expected 0", req.FinishedJobs)
	}
	if req.FinishedAt != nil && !req.FinishedAt.IsZero() {
		t.Errorf("got FinishedAt = %s, expected nil/NULL", req.FinishedAt)
	}

	// Fail pending request
	err = m.FailPending(reqId)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	// Verify post-finish request state
	req, err = m.Get(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if req.State != proto.STATE_FAIL {
		t.Errorf("request state = %d, expected %d", req.State, proto.STATE_FAIL)
	}
	if req.FinishedJobs != 0 {
		t.Errorf("got FinishedJobs = %d, expected 0", req.FinishedJobs)
	}
	if req.FinishedAt.IsZero() {
		t.Errorf("got FinishedAt = nil/NULL, expected a value")
	}
}

func TestJobChainNotFound(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/jc-default.sql")
	defer teardownManager(t, dbName)

	reqId := "invalid"
	cfg := request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
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

func TestFind(t *testing.T) {
	dbName := setupManager(t, rmtest.DataPath+"/request-default.sql")
	defer teardownManager(t, dbName)

	// 1. Filter States
	filter := proto.RequestFilter{
		States: []byte{
			proto.STATE_PENDING,
			proto.STATE_COMPLETE,
		},
	}
	cfg := request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m := request.NewManager(cfg)
	actual, err := m.Find(filter)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected := []proto.Request{
		// ordered by descending create time (most recent first)
		testdb.SavedRequests["93ec156e204ety45sgf0"],
		testdb.SavedRequests["0874a524aa1edn3ysp00"],
	}
	// Expect requests without job chain + args.
	for i, _ := range expected {
		expected[i].JobChain = nil
		expected[i].Args = nil
	}

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}

	// 2. Filter user
	filter = proto.RequestFilter{
		User: "finch",
	}
	cfg = request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m = request.NewManager(cfg)
	actual, err = m.Find(filter)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected = []proto.Request{
		testdb.SavedRequests["454ae2f98a05cv16sdwt"],
	}
	// Expect requests without job chain + args.
	for i, _ := range expected {
		expected[i].JobChain = nil
		expected[i].Args = nil
	}

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}

	// 3. Filter type
	filter = proto.RequestFilter{
		Type: "do-another-thing",
	}
	cfg = request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m = request.NewManager(cfg)
	actual, err = m.Find(filter)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected = []proto.Request{
		// create time is same, so ordered by id (alphabetically)
		testdb.SavedRequests["abandoned_old_sjc___"],
		testdb.SavedRequests["abandoned_sjc_______"],
		testdb.SavedRequests["old_sjc_____________"],
		testdb.SavedRequests["running_abandoned___"],
		testdb.SavedRequests["running_with_old_sjc"],
		testdb.SavedRequests["suspended___________"],
	}
	// Expect requests without job chain + args.
	for i, _ := range expected {
		expected[i].JobChain = nil
		expected[i].Args = nil
	}

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}

	// 4. Filter time
	filter = proto.RequestFilter{
		Since: time.Date(2017, 9, 13, 2, 15, 00, 00, time.UTC),
		Until: time.Date(2017, 9, 13, 2, 45, 00, 00, time.UTC),
	}
	cfg = request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m = request.NewManager(cfg)
	actual, err = m.Find(filter)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected = []proto.Request{
		// ordered by descending create time
		testdb.SavedRequests["93ec156e204ety45sgf0"],
		testdb.SavedRequests["454ae2f98a05cv16sdwt"],
		testdb.SavedRequests["0874a524aa1edn3ysp00"],
	}
	// Expect requests without job chain + args.
	for i, _ := range expected {
		expected[i].JobChain = nil
		expected[i].Args = nil
	}

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}

	// 5. Limit + Offset. Otherwise filter is same as test 3
	filter = proto.RequestFilter{
		Type:   "do-another-thing",
		Limit:  2,
		Offset: 2,
	}
	cfg = request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m = request.NewManager(cfg)
	actual, err = m.Find(filter)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected = []proto.Request{
		// Requests commented out are those removed by the limit + offset:
		// testdb.SavedRequests["abandoned_old_sjc___"],
		// testdb.SavedRequests["abandoned_sjc_______"],
		testdb.SavedRequests["old_sjc_____________"],
		testdb.SavedRequests["running_abandoned___"],
		// testdb.SavedRequests["running_with_old_sjc"],
		// testdb.SavedRequests["suspended___________"],
	}
	// Expect requests without job chain + args.
	for i, _ := range expected {
		expected[i].JobChain = nil
		expected[i].Args = nil
	}

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}

	// 6. Empty filter
	filter = proto.RequestFilter{}
	cfg = request.ManagerConfig{
		ChainCreatorFactory: jccf,
		DBConnector:         dbc,
		JRClient:            &mock.JRClient{},
		ShutdownChan:        shutdownChan,
		DefaultJRURL:        "http://defaulturl:1111",
	}
	m = request.NewManager(cfg)
	actual, err = m.Find(filter)
	if err != nil {
		t.Errorf("error = %s, expected nil", err)
	}

	expected = []proto.Request{
		testdb.SavedRequests["abandoned_old_sjc___"],
		testdb.SavedRequests["abandoned_sjc_______"],
		testdb.SavedRequests["old_sjc_____________"],
		testdb.SavedRequests["running_abandoned___"],
		testdb.SavedRequests["running_with_old_sjc"],
		testdb.SavedRequests["suspended___________"],
		testdb.SavedRequests["93ec156e204ety45sgf0"],
		testdb.SavedRequests["454ae2f98a05cv16sdwt"],
		testdb.SavedRequests["0874a524aa1edn3ysp00"],
	}
	// Expect requests without job chain + args.
	for i, _ := range expected {
		expected[i].JobChain = nil
		expected[i].Args = nil
	}

	if diff := deep.Equal(actual, expected); diff != nil {
		t.Error(diff)
	}
}
