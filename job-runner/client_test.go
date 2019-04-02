// Copyright 2017-2019, Square, Inc.

package jr_test

import (
	"encoding/json"
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
	c := jr.NewClient(&http.Client{})

	_, err := c.NewJobChain(ts.URL, jc)
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
	c = jr.NewClient(&http.Client{})

	_, err = c.NewJobChain(ts.URL, jc)
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
	c := jr.NewClient(&http.Client{})

	_, err := c.ResumeJobChain(ts.URL, sjc)
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
	c = jr.NewClient(&http.Client{})

	_, err = c.ResumeJobChain(ts.URL, sjc)
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
	c := jr.NewClient(&http.Client{})

	err := c.StopRequest(ts.URL, "2")
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
	c = jr.NewClient(&http.Client{})

	err = c.StopRequest(ts.URL, "2")
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

func TestRunning(t *testing.T) {
	var path string
	var method string
	jobs := []proto.JobStatus{
		{
			RequestId: "req1",
			JobId:     "job1",
			Type:      "reqType",
			Name:      "reqName",
			StartedAt: 100,
			State:     proto.STATE_RUNNING,
			Status:    "job status",
			Try:       2,
		},
	}
	bytes, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}))
	c := jr.NewClient(&http.Client{})

	gotJobs, err := c.Running(ts.URL, proto.StatusFilter{})
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/status/running"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}
	if method != "GET" {
		t.Errorf("request method = %s, expected POST", method)
	}
	if diff := deep.Equal(gotJobs, jobs); diff != nil {
		t.Error(diff)
	}
}
