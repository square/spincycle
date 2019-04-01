// Copyright 2017-2019, Square, Inc.

package rm_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
)

var (
	ts     *httptest.Server
	path   string
	method string
)

// setup creates a test http server that allows you to control the response to
// http calls via function arguments. It will record the path and the method for
// calls against it in global variables that can be accessed from tests. It will
// also unmarshal the payload it receives from a call into the struct that the
// "payloadStruct" argument points to.
func setup(t *testing.T, payloadStruct interface{}, responseStatus int, responseBody string) {
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method

		if payloadStruct != nil {
			// Get the request payload.
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			err = json.Unmarshal(body, &payloadStruct)
			if err != nil {
				t.Fatal(err)
			}
		}

		w.WriteHeader(responseStatus)
		if responseBody != "" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, responseBody)
		}
	}))
}

func cleanup() {
	ts.Close()
	ts = nil
	path = ""
	method = ""
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestCreateRequestError(t *testing.T) {
	reqType := "something"
	args := map[string]interface{}{"arg1": "val1"}

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	_, err := c.CreateRequest(reqType, args)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestCreateRequestSuccess(t *testing.T) {
	reqType := "something"
	args := map[string]interface{}{"arg1": "val1"}
	reqId := "abcd1234"
	var payload proto.CreateRequest

	setup(t, &payload, http.StatusCreated, "{\"id\":\""+reqId+"\"}")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	actualReqId, err := c.CreateRequest(reqType, args)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedPayload := proto.CreateRequest{
		Type: reqType,
		Args: args,
	}
	if diff := deep.Equal(payload, expectedPayload); diff != nil {
		t.Error(diff)
	}

	if actualReqId != reqId {
		t.Errorf("request id = %s, expected %s", actualReqId, reqId)
	}

	expectedPath := "/api/v1/requests"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "POST" {
		t.Errorf("request method = %s, expected POST", method)
	}
}

func TestGetRequestError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	_, err := c.GetRequest(reqId)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()
}

func TestGetRequestSuccess(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusOK, "{\"id\":\""+reqId+"\"}")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	actualReq, err := c.GetRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	if actualReq.Id != reqId {
		t.Errorf("request id = %s, expected %s", actualReq.Id, reqId)
	}

	expectedPath := "/api/v1/requests/" + reqId
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "GET" {
		t.Errorf("request method = %s, expected GET", method)
	}
}

func TestStartRequestError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.StartRequest(reqId)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()
}

func TestStartRequest(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusOK, "{\"id\":\""+reqId+"\"}")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.StartRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/requests/" + reqId + "/start"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "PUT" {
		t.Errorf("request method = %s, expected PUT", method)
	}
}

func TestFinishRequestError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	fr := proto.FinishRequest{
		RequestId:    reqId,
		State:        proto.STATE_COMPLETE,
		FinishedAt:   time.Now(),
		FinishedJobs: 10,
	}
	err := c.FinishRequest(fr)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestFinishRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	var payload proto.FinishRequest

	setup(t, &payload, http.StatusOK, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	finishTime := time.Now()
	fr := proto.FinishRequest{
		RequestId:    reqId,
		State:        proto.STATE_COMPLETE,
		FinishedAt:   finishTime,
		FinishedJobs: 10,
	}
	err := c.FinishRequest(fr)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(payload, fr); diff != nil {
		t.Error(diff)
	}

	expectedPath := "/api/v1/requests/" + reqId + "/finish"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "PUT" {
		t.Errorf("request method = %s, expected PUT", method)
	}
}

func TestStopRequestError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.StopRequest(reqId)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()
}

func TestStopRequest(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusOK, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.StopRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/requests/" + reqId + "/stop"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "PUT" {
		t.Errorf("request method = %s, expected PUT", method)
	}
}

func TestSuspendRequestError(t *testing.T) {
	reqId := "abcd1234"
	sjc := proto.SuspendedJobChain{
		RequestId:         reqId,
		JobChain:          &proto.JobChain{},
		TotalJobTries:     make(map[string]uint),
		LatestRunJobTries: make(map[string]uint),
		SequenceTries:     make(map[string]uint),
	}

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.SuspendRequest(reqId, sjc)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()
}

func TestSuspendRequest(t *testing.T) {
	reqId := "abcd1234"
	sjc := proto.SuspendedJobChain{
		RequestId: reqId,
		JobChain: &proto.JobChain{
			RequestId: reqId,
			Jobs: map[string]proto.Job{
				"job1": proto.Job{
					Id:   "job1",
					Data: map[string]interface{}{"data1": "val1"},
				},
			},
		},
		TotalJobTries:     map[string]uint{"job1": 1},
		LatestRunJobTries: map[string]uint{"job1": 1},
		SequenceTries:     map[string]uint{"job1": 1},
	}
	var payload proto.SuspendedJobChain

	setup(t, &payload, http.StatusOK, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.SuspendRequest(reqId, sjc)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedPayload := sjc
	if diff := deep.Equal(payload, expectedPayload); diff != nil {
		t.Error(diff)
	}

	expectedPath := "/api/v1/requests/" + reqId + "/suspend"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "PUT" {
		t.Errorf("request method = %s, expected PUT", method)
	}
}

func TestGetJobChainError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	_, err := c.GetJobChain(reqId)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestGetJobChainSuccess(t *testing.T) {
	reqId := "abcd1234"
	respBody := fmt.Sprintf("{\"requestId\":\"%s\",\"state\":%d}", reqId, proto.STATE_COMPLETE)

	setup(t, nil, http.StatusOK, respBody)
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	jc, err := c.GetJobChain(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedJc := proto.JobChain{
		RequestId: reqId,
		State:     proto.STATE_COMPLETE,
	}
	if diff := deep.Equal(jc, expectedJc); diff != nil {
		t.Error(diff)
	}

	expectedPath := "/api/v1/requests/" + reqId + "/job-chain"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "GET" {
		t.Errorf("request method = %s, expected GET", method)
	}
}

func TestGetJLError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	_, err := c.GetJL(reqId)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestGetJLSuccess(t *testing.T) {
	reqId := "abcd1234"
	jobId := "job1"
	respBody := fmt.Sprintf("[{\"requestId\":\"%s\",\"jobId\":\"%s\",\"state\":%d}]", reqId, jobId, proto.STATE_COMPLETE)

	setup(t, nil, http.StatusOK, respBody)
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	jl, err := c.GetJL(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedjl := []proto.JobLog{
		{
			RequestId: reqId,
			JobId:     jobId,
			State:     proto.STATE_COMPLETE,
		},
	}
	if diff := deep.Equal(jl, expectedjl); diff != nil {
		t.Error(diff)
	}

	expectedPath := "/api/v1/requests/" + reqId + "/log"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "GET" {
		t.Errorf("request method = %s, expected GET", method)
	}
}

func TestCreateJLError(t *testing.T) {
	reqId := "abcd1234"
	jobId := "job1"
	jl := proto.JobLog{
		RequestId: reqId,
		JobId:     jobId,
	}

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.CreateJL(reqId, jl)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestCreateJLSuccess(t *testing.T) {
	reqId := "abcd1234"
	jobId := "job1"
	jl := proto.JobLog{
		RequestId: reqId,
		JobId:     jobId,
	}
	var payload proto.JobLog

	setup(t, &payload, http.StatusCreated, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.CreateJL(reqId, jl)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedPayload := jl
	if diff := deep.Equal(payload, expectedPayload); diff != nil {
		t.Error(diff)
	}

	expectedPath := "/api/v1/requests/" + reqId + "/log"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "POST" {
		t.Errorf("request method = %s, expected POST", method)
	}
}
