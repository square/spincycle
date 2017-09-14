// Copyright 2017, Square, Inc.

package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-test/deep"
	"github.com/labstack/echo"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/api"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

var server *httptest.Server

func setup(rm *mock.RequestManager, middleware ...echo.MiddlewareFunc) {
	a := api.NewAPI(rm, &mock.RMStatus{})
	a.Use(middleware...)
	server = httptest.NewServer(a)
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
	setup(&mock.RequestManager{})
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
	var rmReqParams proto.CreateRequestParams
	rm := &mock.RequestManager{
		CreateRequestFunc: func(reqParams proto.CreateRequestParams) (proto.Request, error) {
			rmReqParams = reqParams
			return proto.Request{}, mock.ErrRequestManager
		},
	}
	setup(rm)
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
	expectedReqParams := proto.CreateRequestParams{
		Type: "something",
		Args: map[string]interface{}{
			"first": "arg1",
		},
		User: "?", // the value from the payload is overwritten
	}
	if diff := deep.Equal(rmReqParams, expectedReqParams); diff != nil {
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
	var rmReqParams proto.CreateRequestParams
	rm := &mock.RequestManager{
		CreateRequestFunc: func(reqParams proto.CreateRequestParams) (proto.Request, error) {
			rmReqParams = reqParams
			return req, nil
		},
	}
	// Add middleware to the API to hardcode the caller's username to "john".
	middlewareFunc := func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("username", "john")
			return h(c)
		}
	}
	setup(rm, middlewareFunc)
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
	expectedReqParams := proto.CreateRequestParams{
		Type: "something",
		Args: map[string]interface{}{
			"first":  "arg1",
			"second": "arg2",
		},
		User: "john",
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
		GetRequestFunc: func(r string) (proto.Request, error) {
			return req, nil
		},
	}
	setup(rm)
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

func TestStartRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	setup(&mock.RequestManager{})
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
	var rmFinishParams proto.FinishRequestParams
	rm := &mock.RequestManager{
		FinishRequestFunc: func(r string, f proto.FinishRequestParams) error {
			rmFinishParams = f
			return nil
		},
	}
	setup(rm)
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
	expectedFinishParams := proto.FinishRequestParams{
		State: proto.STATE_COMPLETE,
	}
	if diff := deep.Equal(rmFinishParams, expectedFinishParams); diff != nil {
		t.Error(diff)
	}
}

func TestStopRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	setup(&mock.RequestManager{})
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

func TestStatusRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	reqStatus := proto.RequestStatus{
		Request: proto.Request{
			Id: reqId,
		},
		JobChainStatus: proto.JobChainStatus{
			JobStatuses: proto.JobStatuses{
				proto.JobStatus{JobId: "j1", Status: "status1", State: proto.STATE_RUNNING},
				proto.JobStatus{JobId: "j2", Status: "status2", State: proto.STATE_FAIL},
			},
		},
	}
	// Create a mock request manager that will return a request status.
	rm := &mock.RequestManager{
		RequestStatusFunc: func(r string) (proto.RequestStatus, error) {
			return reqStatus, nil
		},
	}
	setup(rm)
	defer cleanup()

	// Make the HTTP request.
	var actualReqStatus proto.RequestStatus
	statusCode, _, err := testutil.MakeHTTPRequest("GET", baseURL()+"requests/"+reqId+"/status", []byte{}, &actualReqStatus)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the status code is what we expect.
	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	// Check that the request status is what we expect.
	if diff := deep.Equal(actualReqStatus, reqStatus); diff != nil {
		t.Error(diff)
	}
}

func TestGetJobChainRequestHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	jc := proto.JobChain{
		RequestId: reqId,
		State:     proto.STATE_RUNNING,
	}
	// Create a mock request manager that will return a job chain.
	rm := &mock.RequestManager{
		GetJobChainFunc: func(r string) (proto.JobChain, error) {
			return jc, nil
		},
	}
	setup(rm)
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
	// Create a mock request manager that will return a jl.
	rm := &mock.RequestManager{
		GetJLFunc: func(r string, j string) (proto.JobLog, error) {
			return jl, nil
		},
	}
	setup(rm)
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

func TestCreateJLHandlerSuccess(t *testing.T) {
	reqId := "abcd1234"
	payload := []byte(fmt.Sprintf("{\"requestId\":\"%s\",\"state\":%d}", reqId, proto.STATE_COMPLETE))
	jl := proto.JobLog{
		RequestId: reqId,
		State:     proto.STATE_COMPLETE,
	}
	// Create a mock request manager that will return a jl and record the jl it receives.
	var rmjl proto.JobLog
	rm := &mock.RequestManager{
		CreateJLFunc: func(r string, j proto.JobLog) (proto.JobLog, error) {
			rmjl = j
			return jl, nil
		},
	}
	setup(rm)
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
