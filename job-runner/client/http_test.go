package client_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/client"
	"github.com/square/spincycle/proto"
)

func TestStartRequest(t *testing.T) {
	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := client.NewJRClient(&http.Client{}, ts.URL)

	err := c.StartRequest(3)
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
	c = client.NewJRClient(&http.Client{}, ts.URL)

	err = c.StartRequest(3)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/job-chains/3/start"
	if path != expectedPath {
		t.Errorf("url path = %s, expected %s", path, expectedPath)
	}

	if method != "PUT" {
		t.Errorf("request method = %s, expected POST", method)
	}
}

func TestStopRequest(t *testing.T) {
	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := client.NewJRClient(&http.Client{}, ts.URL)

	err := c.StopRequest(3)
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
	c = client.NewJRClient(&http.Client{}, ts.URL)

	err = c.StopRequest(3)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	ts.Close()

	expectedPath := "/api/v1/job-chains/3/stop"
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
	c := client.NewJRClient(&http.Client{}, ts.URL)

	_, err := c.RequestStatus(3)
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
	c = client.NewJRClient(&http.Client{}, ts.URL)

	_, err = c.RequestStatus(3)
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
		fmt.Fprintln(w, "{\"requestId\":3,\"jobStatuses\":[{\"name\":\"job1\",\"status\":\"job is running...\",\"state\":5}]}")
	}))
	c = client.NewJRClient(&http.Client{}, ts.URL)

	status, err := c.RequestStatus(3)
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

	expectedStatus := &proto.JobChainStatus{
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{
				Name:   "job1",
				Status: "job is running...",
				State:  5,
			},
		},
		RequestId: 3,
	}
	if diff := deep.Equal(status, expectedStatus); diff != nil {
		t.Error(diff)
	}
}

func TestNewJobChain(t *testing.T) {
	// Make a job chain.
	jc := proto.JobChain{
		RequestId: 3,
		Jobs: map[string]proto.Job{
			"job1": proto.Job{
				Name:  "job1",
				Type:  "type1",
				Bytes: []byte{1, 2, 3, 4, 5},
				State: 3,
			},
		},
		AdjacencyList: map[string][]string{
			"job1": []string{},
		},
		State: 4,
	}

	// Unsuccessful response status code.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	c := client.NewJRClient(&http.Client{}, ts.URL)

	err := c.NewJobChain(jc)
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

		w.WriteHeader(http.StatusOK)
	}))
	c = client.NewJRClient(&http.Client{}, ts.URL)

	err = c.NewJobChain(jc)
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
