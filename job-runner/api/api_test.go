// Copyright 2017, Square, Inc.

package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/router"
	"github.com/square/spincycle/test/mock"
)

var noJobData = map[string]interface{}{}

var noRunnersFactory = &mock.RunnerFactory{}
var defaultTraverserFactory chain.TraverserFactory
var defaultTraverserRepo chain.TraverserRepo
var defaultAPI *API
var defaultServer *httptest.Server

func setup(rf runner.Factory) {
	defaultTraverserFactory = chain.NewTraverserFactory(chain.NewMemoryRepo(), rf, runner.NewRepo())
	defaultTraverserRepo = chain.NewTraverserRepo()
	defaultAPI = NewAPI(&router.Router{}, defaultTraverserFactory, defaultTraverserRepo)
	defaultServer = httptest.NewServer(defaultAPI.Router)
}

func teardown() {
	defaultServer.CloseClientConnections()
	defaultServer.Close()
}

// //////////////////////////////////////////////////////////////////////////
// Tests
// //////////////////////////////////////////////////////////////////////////

func TestNewJobChainInvalid(t *testing.T) {
	setup(noRunnersFactory)
	defer teardown()

	// The chain is cyclic.
	jobChain := &proto.JobChain{
		RequestId: "abc",
		Jobs:      mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job1"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Post(defaultServer.URL+API_ROOT+"job-chains", "application/json; charset=utf-8", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 400 {
		t.Errorf("response status = %d, expected 200", res.StatusCode)
	}
}

func TestStartJobChain(t *testing.T) {
	// Start default API with a job runner that can make these mock jobs:
	setup(&mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
			"job2": mock.NewRunner(true, "", nil, nil, noJobData),
			"job3": mock.NewRunner(true, "", nil, nil, noJobData),
		},
	})
	defer teardown()

	// First create job chain: job1 -> job2 -> job3
	jobChain := &proto.JobChain{
		RequestId: "def",
		Jobs:      mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(defaultServer.URL+API_ROOT+"job-chains", "application/json; charset=utf-8", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("response status = %d, expected 200", res.StatusCode)
	}

	// Then start the job chain we just created
	url, err := url.Parse(defaultServer.URL + API_ROOT + "job-chains/" + jobChain.RequestId + "/start")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: "PUT",
		URL:    url,
	}
	res, err = (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != 200 {
		t.Log(url)
		t.Errorf("response status = %d, expected 200", res.StatusCode)
	}
}

func TestStopJobChain(t *testing.T) {
	setup(&mock.RunnerFactory{RunnersToReturn: map[string]*mock.Runner{}})
	defer teardown()

	err := defaultAPI.traverserRepo.Add("4", &mock.Traverser{})
	if err != nil {
		t.Fatal(err)
	}

	url, err := url.Parse(defaultServer.URL + API_ROOT + "job-chains/4/stop")
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

	_, err = defaultAPI.traverserRepo.Get("4")
	if err == nil {
		t.Errorf("Traverser was not removed from the repo as expected.")
	}
}

func TestStopJobChainNotRunning(t *testing.T) {
	setup(&mock.RunnerFactory{RunnersToReturn: map[string]*mock.Runner{}})
	defer teardown()

	url, err := url.Parse(defaultServer.URL + API_ROOT + "job-chains/4/stop")
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
	setup(&mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
			"job2": mock.NewRunner(true, "", nil, nil, noJobData),
			"job3": mock.NewRunner(true, "", nil, nil, noJobData),
		},
	})
	defer teardown()

	chainStatus := proto.JobChainStatus{
		RequestId: "ghi",
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{"job2", "", proto.STATE_FAIL},
			proto.JobStatus{"job3", "95% complete", proto.STATE_RUNNING},
		},
	}

	err := defaultAPI.traverserRepo.Add("4", &mock.Traverser{
		StatusResp: chainStatus,
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(defaultServer.URL + API_ROOT + "job-chains/4/status")
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	var gotStatus proto.JobChainStatus
	if err := json.Unmarshal(bytes, &gotStatus); err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(gotStatus, chainStatus); diff != nil {
		t.Error(diff)
	}
}
