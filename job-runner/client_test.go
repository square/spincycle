// Copyright 2017, Square, Inc.

package jr_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-test/deep"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
)

func TestNewJobChain(t *testing.T) {
	// Make a job chain.
	jc := proto.JobChain{
		RequestId: "4",
		Jobs: map[string]proto.Job{
			"job1": {
				Id:    "job1",
				Type:  "type1",
				Bytes: []byte{1, 2, 3, 4, 5},
				State: 3,
			},
		},
		AdjacencyList: map[string][]string{
			"job1": {},
		},
		State: 4,
	}

	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := jr.NewClient(&http.Client{}, ts.URL)

	_, err := c.NewJobChain(jc)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()

	// Successful response status code.
	var path string
	var method string
	var payload proto.JobChain
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method

		// Get the request payload.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal(body, &payload)
		if err != nil {
			t.Fatal(err)
		}

		w.Header().Add("Location", "location")
		w.WriteHeader(http.StatusOK)
	}))
	c = jr.NewClient(&http.Client{}, ts.URL)

	_, err = c.NewJobChain(jc)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/job-chains"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "POST" {
		t.Errorf("request method = %s, expected POST", method)
	}

	if diff := deep.Equal(payload, jc); diff != nil {
		t.Error(diff)
	}
}

func TestResumeJobChain(t *testing.T) {
	// Make a job chain.
	jc := proto.JobChain{
		RequestId: "4",
		Jobs: map[string]proto.Job{
			"job1": {
				Id:    "job1",
				Type:  "type1",
				Bytes: []byte{1, 2, 3, 4, 5},
				State: 3,
			},
		},
		AdjacencyList: map[string][]string{
			"job1": {},
		},
		State: 4,
	}
	sjc := proto.SuspendedJobChain{
		RequestId: "4",
		JobChain:  &jc,
		SequenceTries: map[string]uint{
			"job1": 1,
		},
		TotalJobTries: map[string]uint{
			"job1": 2,
		},
		LatestRunJobTries: map[string]uint{
			"job1": 1,
		},
	}
	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := jr.NewClient(&http.Client{}, ts.URL)

	_, err := c.ResumeJobChain(sjc)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()

	// Successful response status code.
	var path string
	var method string
	var payload proto.SuspendedJobChain
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method

		// Get the request payload.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal(body, &payload)
		if err != nil {
			t.Fatal(err)
		}

		w.Header().Add("Location", "location")
		w.WriteHeader(http.StatusOK)
	}))
	c = jr.NewClient(&http.Client{}, ts.URL)

	_, err = c.ResumeJobChain(sjc)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/job-chains/resume"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "POST" {
		t.Errorf("request method = %s, expected POST", method)
	}

	if diff := deep.Equal(payload, sjc); diff != nil {
		t.Error(diff)
	}
}

func TestStopRequest(t *testing.T) {
	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := jr.NewClient(&http.Client{}, ts.URL)

	err := c.StopRequest("2", ts.URL)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()

	// Successful response status code.
	var path string
	var method string
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	c = jr.NewClient(&http.Client{}, ts.URL)

	err = c.StopRequest("2", ts.URL)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/job-chains/2/stop"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "PUT" {
		t.Errorf("request method = %s, expected POST", method)
	}
}

func TestRequestStatus(t *testing.T) {
	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := jr.NewClient(&http.Client{}, ts.URL)

	_, err := c.RequestStatus("3", ts.URL)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()

	// Successful response status code, but bad payload.
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "baD{json")
	}))
	c = jr.NewClient(&http.Client{}, ts.URL)

	_, err = c.RequestStatus("3", ts.URL)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	ts.Close()

	// Successful response status code.
	var path string
	var method string
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "{\"requestId\":\"3\",\"jobStatuses\":[{\"JobId\":\"job1\",\"status\":\"job is running...\",\"state\":5}]}")
	}))
	c = jr.NewClient(&http.Client{}, ts.URL)

	status, err := c.RequestStatus("3", ts.URL)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/job-chains/3/status"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "GET" {
		t.Errorf("request method = %s, expected POST", method)
	}

	expectedStatus := proto.JobChainStatus{
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{
				JobId:  "job1",
				Status: "job is running...",
				State:  5,
			},
		},
		RequestId: "3",
	}
	if diff := deep.Equal(status, expectedStatus); diff != nil {
		t.Error(diff)
	}
}
