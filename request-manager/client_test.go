package rm_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

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
	var payload proto.CreateRequestParams

	setup(t, &payload, http.StatusCreated, "{\"id\":\""+reqId+"\"}")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	actualReqId, err := c.CreateRequest(reqType, args)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedPayload := proto.CreateRequestParams{
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

	err := c.FinishRequest(reqId, proto.STATE_COMPLETE)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestFinishRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	var payload proto.FinishRequestParams

	setup(t, &payload, http.StatusOK, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	err := c.FinishRequest(reqId, proto.STATE_COMPLETE)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedPayload := proto.FinishRequestParams{
		State: proto.STATE_COMPLETE,
	}
	if diff := deep.Equal(payload, expectedPayload); diff != nil {
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

func TestRequestStatusError(t *testing.T) {
	reqId := "abcd1234"

	setup(t, nil, http.StatusBadRequest, "")
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	_, err := c.RequestStatus(reqId)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

func TestRequestStatusSuccess(t *testing.T) {
	reqId := "abcd1234"
	respBody := `{"request":{"id":"` + reqId + `"},"jobChainStatus":{"jobStatuses":[{"jobId":"job1"}]}}`

	setup(t, nil, http.StatusOK, respBody)
	defer cleanup()
	c := rm.NewClient(&http.Client{}, ts.URL)

	status, err := c.RequestStatus(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedStatus := proto.RequestStatus{
		Request: proto.Request{
			Id: reqId,
		},
		JobChainStatus: proto.JobChainStatus{
			JobStatuses: proto.JobStatuses{
				proto.JobStatus{JobId: "job1"},
			},
		},
	}
	if diff := deep.Equal(status, expectedStatus); diff != nil {
		t.Error(diff)
	}

	expectedPath := "/api/v1/requests/" + reqId + "/status"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "GET" {
		t.Errorf("request method = %s, expected GET", method)
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

func TestGetjlrror(t *testing.T) {
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

func TestCreatejlrror(t *testing.T) {
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
