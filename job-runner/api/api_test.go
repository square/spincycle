// Copyright 2017, Square, Inc.

package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/router"
	"github.com/square/spincycle/test/mock"
)

func TestNewJobChainValid(t *testing.T) {
	api := NewAPI(&router.Router{}, chain.NewMemoryRepo(), &mock.RunnerFactory{})
	jobChain := &proto.JobChain{
		RequestId: uint(4),
		Jobs:      mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2"},
			"job2": []string{"job3"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	h := httptest.NewServer(api.Router)
	defer h.Close()

	res, err := http.Post(h.URL+API_ROOT+"job-chains", "application/json; charset=utf-8", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 200 {
		t.Errorf("response status = %d, expected 200", res.StatusCode)
	}
}

func TestNewJobChainInvalid(t *testing.T) {
	api := NewAPI(&router.Router{}, chain.NewMemoryRepo(), &mock.RunnerFactory{})
	// The chain is cyclic.
	jobChain := &proto.JobChain{
		RequestId: uint(4),
		Jobs:      mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2"},
			"job2": []string{"job3"},
			"job3": []string{"job1"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	h := httptest.NewServer(api.Router)
	defer h.Close()

	res, err := http.Post(h.URL+API_ROOT+"job-chains", "application/json; charset=utf-8", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 400 {
		t.Errorf("response status = %d, expected 200", res.StatusCode)
	}
}

func TestStartJobChain(t *testing.T) {
	api := NewAPI(&router.Router{}, chain.NewMemoryRepo(), &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job3": mock.NewRunner(true, "", nil, nil, map[string]string{}),
		},
	})
	jobChain := &proto.JobChain{
		RequestId: uint(4),
		Jobs:      mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2"},
			"job2": []string{"job3"},
		},
	}
	c := chain.NewChain(jobChain)

	traverser, err := chain.NewTraverser(api.chainRepo, api.runnerFactory, c)
	if err != nil {
		t.Fatal(err)
	}
	err = api.traverserRepo.Add("4", traverser)
	if err != nil {
		t.Fatal(err)
	}

	h := httptest.NewServer(api.Router)
	defer h.Close()

	url, err := url.Parse(h.URL + API_ROOT + "job-chains/4/start")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: "PUT",
		URL:    url,
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 200 {
		t.Errorf("response status = %d, expected 200", res.StatusCode)
	}
}

func TestStopJobChain(t *testing.T) {
	api := NewAPI(&router.Router{}, chain.NewMemoryRepo(), &mock.RunnerFactory{RunnersToReturn: map[string]*mock.Runner{}})

	err := api.traverserRepo.Add("4", &mock.Traverser{})
	if err != nil {
		t.Fatal(err)
	}

	h := httptest.NewServer(api.Router)
	defer h.Close()

	url, err := url.Parse(h.URL + API_ROOT + "job-chains/4/stop")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: "PUT",
		URL:    url,
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 200 {
		t.Errorf("response status = %d, expected 200", res.StatusCode)
	}

	_, err = api.traverserRepo.Get("4")
	if err == nil {
		t.Errorf("Traverser was not removed from the repo as expected.")
	}
}

func TestStopJobChainNotRunning(t *testing.T) {
	api := NewAPI(&router.Router{}, chain.NewMemoryRepo(), &mock.RunnerFactory{RunnersToReturn: map[string]*mock.Runner{}})

	h := httptest.NewServer(api.Router)
	defer h.Close()

	url, err := url.Parse(h.URL + API_ROOT + "job-chains/4/stop")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: "PUT",
		URL:    url,
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 404 {
		t.Errorf("response status = %d, expected 404", res.StatusCode)
	}
}

func TestStatusJobChain(t *testing.T) {
	api := NewAPI(&router.Router{}, chain.NewMemoryRepo(), &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job3": mock.NewRunner(true, "", nil, nil, map[string]string{}),
		},
	})
	chainStatus := proto.JobChainStatus{
		RequestId: uint(4),
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{"job2", "", proto.STATE_FAIL},
			proto.JobStatus{"job3", "95% complete", proto.STATE_RUNNING},
		},
	}

	err := api.traverserRepo.Add("4", &mock.Traverser{
		StatusResp: chainStatus,
	})
	if err != nil {
		t.Fatal(err)
	}

	h := httptest.NewServer(api.Router)
	defer h.Close()

	res, err := http.Get(h.URL + API_ROOT + "job-chains/4/status")
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	var actualResponse proto.JobChainStatus
	if err := json.Unmarshal(bytes, &actualResponse); err != nil {
		t.Fatal(err)
	}

	expectedResponse := chainStatus

	if !reflect.DeepEqual(actualResponse, expectedResponse) {
		t.Errorf("actual response = %v, expected %v", actualResponse, expectedResponse)
	}
}
