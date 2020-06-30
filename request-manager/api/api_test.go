// Copyright 2017-2019, Square, Inc.

package api_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/labstack/echo/v4"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/api"
	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/auth"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
	v "github.com/square/spincycle/version"
)

var server *httptest.Server

var mockAuth = mock.AuthPlugin{
	AuthenticateFunc: func(*http.Request) (auth.Caller, error) {
		return auth.Caller{
			Name:  "test",
			Roles: []string{"test"},
		}, nil
	},
}

func setup(rm *mock.RequestManager, rr *mock.RequestResumer, jls *mock.JLStore, shutdownChan chan struct{}) {
	appCtx := app.Defaults()
	appCtx.RM = rm
	appCtx.JLS = jls
	appCtx.RR = rr
	appCtx.Status = &mock.RMStatus{}
	appCtx.ShutdownChan = shutdownChan
	appCtx.Hooks.SetUsername = func(*http.Request) (string, error) {
		return "admin", nil
	}
	appCtx.Plugins.Auth = mockAuth
	appCtx.Auth = auth.NewManager(mockAuth, map[string][]auth.ACL{}, []string{"test"}, false)
	server = httptest.NewServer(api.NewAPI(appCtx))
}

func cleanup() {
	server.CloseClientConnections()
	server.Close()
}

func baseURL() string {
	if server != nil {
		return server.URL + api.API_ROOT
	}
	return api.API_ROOT
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestNewRequestHandlerInvalidPayload(t *testing.T) {
	payload := `"bad":"json"}` // Bad payload.
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"requests", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusBadRequest {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusBadRequest)
	}
}

func TestNewRequestHandlerRMError(t *testing.T) {
	payload := `{"type":"something","args":{"first":"arg1"},"user":"mike"}`
	// Create a mock request manager that will return an error and record the
	// request params it receives.
	var rmReqParams proto.CreateRequest
	rm := &mock.RequestManager{
		CreateFunc: func(reqParams proto.CreateRequest) (proto.Request, error) {
			rmReqParams = reqParams
			return proto.Request{}, mock.ErrRequestManager
		},
	}
	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"requests", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}

	// Check the request params sent to the request manager.
	expectedReqParams := proto.CreateRequest{
		Type: "something",
		Args: map[string]interface{}{
			"first": "arg1",
		},
		User: "admin", // the value from the payload is overwritten
	}
	if diff := deep.Equal(rmReqParams, expectedReqParams); diff != nil {
		t.Error(diff)
	}
}

func TestNewRequestHandlerBadStart(t *testing.T) {
	payload := `{"type":"something","args":{"first":"arg1","second":"arg2"}}`
	// Create a mock request manager that will fail on Start, so that status will be set to FAIL.
	var rmFinishParams proto.FinishRequest
	rm := &mock.RequestManager{
		StartFunc: func(string) error {
			return mock.ErrRequestManager
		},
		FailPendingFunc: func(string) error {
			rmFinishParams.State = proto.STATE_FAIL
			return nil
		},
	}
	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"requests", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}

	// Check that the request failed.
	expectedFinishParams := proto.FinishRequest{
		State: proto.STATE_FAIL,
	}
	if diff := deep.Equal(rmFinishParams, expectedFinishParams); diff != nil {
		t.Error(diff)
	}
}

func TestNewRequestHandlerSuccess(t *testing.T) {
	payload := `{"type":"something","args":{"first":"arg1","second":"arg2"}}`
	reqId := "abcd1234"
	req := proto.Request{
		Id:    reqId,
		State: proto.STATE_PENDING,
	}
	// Create a mock request manager that will return a request and record the
	// request params it receives.
	var rmReqParams proto.CreateRequest
	rm := &mock.RequestManager{
		CreateFunc: func(reqParams proto.CreateRequest) (proto.Request, error) {
			rmReqParams = reqParams
			return req, nil
		},
	}

	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualReq proto.Request
	statusCode, headers, err := testutil.MakeHTTPRequest("POST", baseURL()+"requests", []byte(payload), &actualReq)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusCreated {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusCreated)
	}

	// Check that the response body is what we expect.
	if diff := deep.Equal(actualReq, req); diff != nil {
		t.Error(diff)
	}

	// Check the response location header.
	expectedLocation := api.API_ROOT + "requests/" + req.Id
	if len(headers["Location"]) < 1 {
		t.Errorf("location header not set at all")
	} else {
		if headers["Location"][0] != expectedLocation {
			t.Errorf("location header = %s, expected %s", headers["Location"][0], expectedLocation)
		}
	}

	// Check the request params sent to the request manager.
	expectedReqParams := proto.CreateRequest{
		Type: "something",
		Args: map[string]interface{}{
			"first":  "arg1",
			"second": "arg2",
		},
		User: "admin",
	}
	if diff := deep.Equal(rmReqParams, expectedReqParams); diff != nil {
		t.Error(diff)
	}
}

func TestGetRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	req := proto.Request{
		Id:    reqId,
		State: proto.STATE_PENDING,
	}
	// Create a mock request manager that will return a request.
	rm := &mock.RequestManager{
		GetWithJCFunc: func(r string) (proto.Request, error) {
			return req, nil
		},
	}
	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualReq proto.Request
	statusCode, _, err := testutil.MakeHTTPRequest("GET", baseURL()+"requests/"+reqId, []byte{}, &actualReq)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the response body is what we expect.
	if diff := deep.Equal(actualReq, req); diff != nil {
		t.Error(diff)
	}
}

func TestFindRequestsHandler(t *testing.T) {
	reqs := []proto.Request{
		proto.Request{
			Id:    "abcd1234",
			State: proto.STATE_PENDING,
		},
	}
	// Create a mock request manager to record the filter the API sets.
	var gotFilter proto.RequestFilter
	rm := &mock.RequestManager{
		FindFunc: func(filter proto.RequestFilter) ([]proto.Request, error) {
			gotFilter = filter
			return reqs, nil
		},
	}
	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	sentFilter := proto.RequestFilter{
		Type: "request-type",
		States: []byte{
			proto.STATE_PENDING,
			proto.STATE_RUNNING,
			proto.STATE_SUSPENDED,
		},
		User:   "felixp",
		Since:  time.Date(2020, 01, 01, 12, 34, 56, 789000000, time.UTC),
		Until:  time.Date(2020, 01, 02, 12, 34, 56, 789000000, time.UTC),
		Limit:  5,
		Offset: 10,
	}

	var actualReqs []proto.Request
	statusCode, _, err := testutil.MakeHTTPRequest("GET", baseURL()+"requests?"+sentFilter.String(), []byte{}, &actualReqs)
	if err != nil {
		t.Error(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Fatalf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the response body is what we expect.
	if diff := deep.Equal(actualReqs, reqs); diff != nil {
		t.Error(diff)
	}

	// Check that parsed filter is what we expect.
	expectFilter := proto.RequestFilter{
		Type: "request-type",
		States: []byte{
			proto.STATE_PENDING,
			proto.STATE_RUNNING,
			proto.STATE_SUSPENDED,
		},
		User:   "felixp",
		Since:  time.Date(2020, 01, 01, 12, 34, 56, 789000000, time.UTC),
		Until:  time.Date(2020, 01, 02, 12, 34, 56, 789000000, time.UTC),
		Limit:  5,
		Offset: 10,
	}
	if diff := deep.Equal(gotFilter, expectFilter); diff != nil {
		t.Error(diff)
	}
}

func TestStartRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"requests/"+reqId+"/start", []byte{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}
}

func TestFinishRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	payload := []byte(fmt.Sprintf("{\"state\":%d}", proto.STATE_COMPLETE))
	// Create a mock request manager that will record the finish params it receives.
	var rmFinishParams proto.FinishRequest
	rm := &mock.RequestManager{
		FinishFunc: func(r string, f proto.FinishRequest) error {
			rmFinishParams = f
			return nil
		},
	}
	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"requests/"+reqId+"/finish", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the finish params sent to the request manager are what we expect.
	expectedFinishParams := proto.FinishRequest{
		State: proto.STATE_COMPLETE,
	}
	if diff := deep.Equal(rmFinishParams, expectedFinishParams); diff != nil {
		t.Error(diff)
	}
}

func TestStopRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"requests/"+reqId+"/stop", []byte{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}
}

func TestSuspendRequestHandlerSuccess(t *testing.T) {
	reqId := "729ghskd329dhj3sbjnr"
	payload := []byte("{\"requestId\":\"729ghskd329dhj3sbjnr\",\"jobChain\":{\"requestId\":\"729ghskd329dhj3sbjnr\",\"jobs\":{\"hw48\":{\"id\":\"hw48\",\"type\":\"test\",\"bytes\":null,\"state\":6,\"args\":null,\"data\":null,\"retry\":5,\"retryWait\":\"1s\",\"sequenceId\":\"hw48\",\"sequenceRetry\":1}},\"adjacencyList\":null,\"state\":7},\"totalJobTries\":{\"hw48\":5},\"latestRunJobTries\":{\"hw48\":2},\"sequenceTries\":{\"hw48\":1}}")

	// Create a mock request manager that will record the finish params it receives.
	var rrSJC proto.SuspendedJobChain
	rr := &mock.RequestResumer{
		SuspendFunc: func(sjc proto.SuspendedJobChain) error {
			rrSJC = sjc
			return nil
		},
	}
	setup(&mock.RequestManager{}, rr, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"requests/"+reqId+"/suspend", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the finish params sent to the request manager are what we expect.
	expectedSJC := proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					RetryWait:     "1s",
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}
	if diff := deep.Equal(rrSJC, expectedSJC); diff != nil {
		t.Error(diff)
	}
}

func TestSuspendRequestHandlerInvalidPayload(t *testing.T) {
	payload := `"bad":"json"}` // Bad payload.
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"requests/4/suspend", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusBadRequest {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusBadRequest)
	}
}

func TestSuspendRequestHandlerRMError(t *testing.T) {
	// reqId := "4"
	// payload := `{"requestId":"4","jobChain":{"requestId":"4","jobs":{"hw48":{"id":"hw48","type":"test","state":6,"retry":5,"sequenceId":"hw48","sequenceRetry":1}},"state":7},"jobTries":{"hw48":5},"stoppedJobTries":{"hw48":2},"sequenceRetries":{"hw48":1}}`
	reqId := "729ghskd329dhj3sbjnr"
	payload := []byte("{\"requestId\":\"729ghskd329dhj3sbjnr\",\"jobChain\":{\"requestId\":\"729ghskd329dhj3sbjnr\",\"jobs\":{\"hw48\":{\"id\":\"hw48\",\"type\":\"test\",\"bytes\":null,\"state\":6,\"args\":null,\"data\":null,\"retry\":5,\"retryWait\":\"1s\",\"sequenceId\":\"hw48\",\"sequenceRetry\":1}},\"adjacencyList\":null,\"state\":7},\"totalJobTries\":{\"hw48\":5},\"latestRunJobTries\":{\"hw48\":2},\"sequenceTries\":{\"hw48\":1}}")

	// Create a mock request manager that will return an error and record the
	// sjc it receives.
	var rrSJC proto.SuspendedJobChain
	rr := &mock.RequestResumer{
		SuspendFunc: func(sjc proto.SuspendedJobChain) error {
			rrSJC = sjc
			return mock.ErrRequestResumer
		},
	}
	setup(&mock.RequestManager{}, rr, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"requests/"+reqId+"/suspend", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}

	// Check the request params sent to the request manager.
	expectedSJC := proto.SuspendedJobChain{
		RequestId:         reqId,
		TotalJobTries:     map[string]uint{"hw48": 5},
		LatestRunJobTries: map[string]uint{"hw48": 2},
		SequenceTries:     map[string]uint{"hw48": 1},
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"hw48": proto.Job{
					Id:            "hw48",
					Type:          "test",
					State:         proto.STATE_STOPPED,
					Retry:         5,
					RetryWait:     "1s",
					SequenceId:    "hw48",
					SequenceRetry: 1,
				},
			},
			State: proto.STATE_SUSPENDED,
		},
	}
	if diff := deep.Equal(rrSJC, expectedSJC); diff != nil {
		t.Error(diff)
	}
}

func TestGetJobChainRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	jc := proto.JobChain{
		RequestId: reqId,
		State:     proto.STATE_RUNNING,
	}
	// Create a mock jobchain store that will return a job chain.
	rm := &mock.RequestManager{
		JobChainFunc: func(r string) (proto.JobChain, error) {
			return jc, nil
		},
	}
	setup(rm, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualJc proto.JobChain
	statusCode, _, err := testutil.MakeHTTPRequest("GET", baseURL()+"requests/"+reqId+"/job-chain", []byte{}, &actualJc)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the job chain is what we expect.
	if diff := deep.Equal(actualJc, jc); diff != nil {
		t.Error(diff)
	}
}

func TestGetJLHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	jobId := "job1"
	jl := proto.JobLog{
		RequestId: reqId,
		State:     proto.STATE_COMPLETE,
	}
	// Create a mock joblog store that will return a jl.
	jls := &mock.JLStore{
		GetFunc: func(r string, j string) (proto.JobLog, error) {
			return jl, nil
		},
	}
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, jls, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualjl proto.JobLog
	statusCode, _, err := testutil.MakeHTTPRequest("GET",
		baseURL()+"requests/"+reqId+"/log/"+jobId, []byte{}, &actualjl)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the job chain is what we expect.
	if diff := deep.Equal(actualjl, jl); diff != nil {
		t.Error(diff)
	}
}

func TestGetFullJLHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	jlList := []proto.JobLog{
		proto.JobLog{
			RequestId: reqId,
			State:     proto.STATE_COMPLETE,
		},
	}
	// Create a mock joblog store that will return a list of JLs.
	jls := &mock.JLStore{
		GetFullFunc: func(r string) ([]proto.JobLog, error) {
			return jlList, nil
		},
	}
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, jls, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualjlList []proto.JobLog
	statusCode, _, err := testutil.MakeHTTPRequest("GET",
		baseURL()+"requests/"+reqId+"/log", []byte{}, &actualjlList)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the job chain is what we expect.
	if diff := deep.Equal(actualjlList, jlList); diff != nil {
		t.Error(diff)
	}
}

func TestCreateJLHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	payload := []byte(fmt.Sprintf("{\"requestId\":\"%s\",\"state\":%d}", reqId, proto.STATE_COMPLETE))
	jl := proto.JobLog{
		RequestId: reqId,
		State:     proto.STATE_COMPLETE,
	}
	// Create a mock joblog store that will return a jl and record the jl it receives.
	var rmjl proto.JobLog
	jls := &mock.JLStore{
		CreateFunc: func(r string, j proto.JobLog) (proto.JobLog, error) {
			rmjl = j
			return jl, nil
		},
	}

	setup(&mock.RequestManager{}, &mock.RequestResumer{}, jls, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualjl proto.JobLog
	statusCode, _, err := testutil.MakeHTTPRequest("POST",
		baseURL()+"requests/"+reqId+"/log", payload, &actualjl)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusCreated {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusCreated)
	}

	// Check that the response body is what we expect.
	if diff := deep.Equal(actualjl, jl); diff != nil {
		t.Error(diff)
	}

	// Check the jl sent to the request manager is what we expect.
	if diff := deep.Equal(rmjl, jl); diff != nil {
		t.Error(diff)
	}
}

func TestCreateJLHandlerJobFailed(t *testing.T) {
	reqId := "abcd1234"
	payload := []byte(fmt.Sprintf("{\"requestId\":\"%s\",\"state\":%d}", reqId, proto.STATE_FAIL))
	jl := proto.JobLog{
		RequestId: reqId,
		State:     proto.STATE_FAIL,
	}
	// Create a mock joblog store that will return a jl.
	jls := &mock.JLStore{
		CreateFunc: func(r string, j proto.JobLog) (proto.JobLog, error) {
			return jl, nil
		},
	}

	setup(&mock.RequestManager{}, &mock.RequestResumer{}, jls, make(chan struct{}))
	defer cleanup()

	// Make the HTTP request.
	var actualjl proto.JobLog
	statusCode, _, err := testutil.MakeHTTPRequest("POST",
		baseURL()+"requests/"+reqId+"/log", payload, &actualjl)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusCreated {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusCreated)
	}
}

func TestAuth(t *testing.T) {
	// Test authentication and authorizaiton with an auth plugin we control.
	// The app default auth allows everything, so we have to override the plugin.
	var caller auth.Caller
	var authenErr, authorErr error
	var authenticateCalled, authorizeCalled, createCalled, startCalled bool
	var authOp string
	reset := func() {
		authenticateCalled = false
		authorizeCalled = false
		createCalled = false
		startCalled = false
		authenErr = nil
		authorErr = nil
		authOp = ""
		caller = auth.Caller{
			Name:  "dn",
			Roles: []string{"role1", "role2"}, // matches roles in auth-001.yaml (see below)
		}
	}
	ctx := app.Defaults()
	ctx.Plugins.Auth = mock.AuthPlugin{
		AuthenticateFunc: func(*http.Request) (auth.Caller, error) {
			authenticateCalled = true
			return caller, authenErr
		},
		AuthorizeFunc: func(c auth.Caller, op string, req proto.Request) error {
			authorizeCalled = true
			authOp = op
			return authorErr
		},
	}

	req := proto.Request{
		Id:    "xyz",
		Type:  "req1",
		State: proto.STATE_PENDING,
	}
	ctx.RM = &mock.RequestManager{
		CreateFunc: func(proto.CreateRequest) (proto.Request, error) {
			createCalled = true
			return req, nil
		},
		StartFunc: func(string) error {
			startCalled = true
			return nil
		},
	}

	acls := map[string][]auth.ACL{
		"req1": []auth.ACL{
			{
				Role:  "role1",
				Admin: true,
			},
			{
				Role: "role2",
				Ops:  []string{"start", "stop"},
			},
		},
	}
	ctx.Auth = auth.NewManager(ctx.Plugins.Auth, acls, nil, true)

	server := httptest.NewServer(api.NewAPI(ctx))
	defer func() {
		server.CloseClientConnections()
		server.Close()
	}()
	baseURL := server.URL + api.API_ROOT

	payload := `{"type":"req1","args":{"arg1":"hello","second":"arg2"}}`

	// Authentication fails
	// ----------------------------------------------------------------------

	reset()
	authenErr = fmt.Errorf("forced authenticated error")

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL+"requests", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != http.StatusUnauthorized {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusUnauthorized)
	}
	if authenticateCalled == false {
		t.Errorf("Authenticate not called, expected it to be called")
	}
	if authorizeCalled == true {
		t.Errorf("Authorize called, expected it NOT to be called")
	}
	if createCalled == true {
		t.Errorf("request.Manager.Create called, expected it NOT to be called")
	}

	// Authentication ok, then authorize fails in the controller
	// ----------------------------------------------------------------------

	reset()
	authorErr = fmt.Errorf("forced authorization error")

	var resp echo.HTTPError
	statusCode, _, err = testutil.MakeHTTPRequest("POST", baseURL+"requests", []byte(payload), &resp)
	t.Logf("resp: %+v", resp)
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != http.StatusUnauthorized {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusUnauthorized)
	}
	if authenticateCalled == false {
		t.Errorf("Authenticate not called, expected it to be called")
	}
	if authorizeCalled == false {
		t.Errorf("Authorize not called, expected it to be called")
	}
	if authOp != proto.REQUEST_OP_START {
		t.Errorf("got op %s, expected %s", authOp, proto.REQUEST_OP_START)
	}
	if createCalled == false { // have to create it to authorize it
		t.Errorf("request.Manager.Create not called, expected it to be called")
	}
	if startCalled == true { // but auth fails, so don't start it
		t.Errorf("request.Manager.Start called, expected it NOT to be called")
	}

	// All auth OK
	// ----------------------------------------------------------------------

	reset()

	statusCode, _, err = testutil.MakeHTTPRequest("POST", baseURL+"requests", []byte(payload), nil)
	if err != nil {
		t.Fatal(err)
	}
	if statusCode != http.StatusCreated {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusCreated)
	}
	if authenticateCalled == false {
		t.Errorf("Authenticate not called, expected it to be called")
	}
	if authorizeCalled == false {
		t.Errorf("Authorize not called, expected it to be called")
	}
	if createCalled == false {
		t.Errorf("request.Manager.Create not called, expected it to be called")
	}
	if startCalled == false {
		t.Errorf("request.Manager.Start not called, expected it to be called")
	}
}

func TestGetVersion(t *testing.T) {
	setup(&mock.RequestManager{}, &mock.RequestResumer{}, &mock.JLStore{}, make(chan struct{}))
	defer cleanup()
	resp, err := http.Get(server.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", resp.StatusCode, http.StatusOK)
	}
	expectVersion := v.Version()
	gotVersion := string(body)
	if gotVersion != expectVersion {
		t.Errorf("got version '%s', expected '%s'", gotVersion, expectVersion)
	}
}
