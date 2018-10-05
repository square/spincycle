// Copyright 2017, Square, Inc.

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-test/deep"
	"github.com/orcaman/concurrent-map"

	"github.com/square/spincycle/job-runner/api"
	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

var (
	server        *httptest.Server
	traverserRepo cmap.ConcurrentMap
	shutdownChan  chan struct{}
)

func setup(traverserFactory *mock.TraverserFactory) {
	traverserRepo = cmap.New()
	shutdownChan = make(chan struct{})
	api := api.NewAPI(app.Defaults(), traverserFactory, traverserRepo, &mock.JRStatus{}, shutdownChan)
	server = httptest.NewServer(api)
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

func TestNewJobChainTraverserError(t *testing.T) {
	// Force an error in creating a traverser.
	tf := &mock.TraverserFactory{
		MakeFunc: func(jc proto.JobChain) (chain.Traverser, error) {
			return nil, mock.ErrTraverser
		},
	}
	setup(tf)
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}
}

func TestNewJobChainMultipleTraverser(t *testing.T) {
	setup(&mock.TraverserFactory{})
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a key into the repo that will cause an "already exists" error.
	traverserRepo.Set(jobChain.RequestId, "blah")

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusBadRequest {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusBadRequest)
	}
}

// Test new job chain endpoint when Job Runner is shutting down.
func TestNewJobChainShutdown(t *testing.T) {
	setup(&mock.TraverserFactory{})
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	// Close shutdownChan as if Job Runner is shutting down -
	// should cause "no new job chains are being started" error
	close(shutdownChan)

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusServiceUnavailable {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusBadRequest)
	}
}

func TestNewJobChainSuccess(t *testing.T) {
	requestId := "abc"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	payload, err := json.Marshal(jobChain)
	if err != nil {
		t.Fatal(err)
	}

	statusCode, headers, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	h, _ := os.Hostname()
	expectedLocation := h + api.API_ROOT + "job-chains/" + requestId
	if len(headers["Location"]) < 1 {
		t.Errorf("location header not set at all")
	} else {
		if headers["Location"][0] != expectedLocation {
			t.Errorf("location header = %s, expected %s", headers["Location"][0], expectedLocation)
		}
	}
}

// Test successfully resuming a job chain.
func TestResumeJobChainSuccess(t *testing.T) {
	requestId := "abc"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	sjc := proto.SuspendedJobChain{
		RequestId:         requestId,
		JobChain:          &jobChain,
		TotalJobTries:     map[string]uint{},
		LatestRunJobTries: map[string]uint{},
		SequenceTries:     map[string]uint{},
	}
	payload, err := json.Marshal(sjc)
	if err != nil {
		t.Fatal(err)
	}

	statusCode, headers, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains/resume", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	h, _ := os.Hostname()
	expectedLocation := h + api.API_ROOT + "job-chains/" + requestId
	if len(headers["Location"]) < 1 {
		t.Errorf("location header not set at all")
	} else {
		if headers["Location"][0] != expectedLocation {
			t.Errorf("location header = %s, expected %s", headers["Location"][0], expectedLocation)
		}
	}
}

// Test resume job chain endpoint when Job Runner is shutting down.
func TestResumeJobChainShutdown(t *testing.T) {
	setup(&mock.TraverserFactory{})
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	sjc := proto.SuspendedJobChain{
		RequestId:         "abc",
		JobChain:          &jobChain,
		TotalJobTries:     map[string]uint{},
		LatestRunJobTries: map[string]uint{},
		SequenceTries:     map[string]uint{},
	}
	payload, err := json.Marshal(sjc)
	if err != nil {
		t.Fatal(err)
	}

	// Close shutdownChan as if Job Runner is shutting down -
	// should cause "no new job chains are being started" error
	close(shutdownChan)

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains/resume", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusServiceUnavailable {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusBadRequest)
	}
}

func TestResumeJobChainTraverserError(t *testing.T) {
	// Force an error in creating a traverser.
	tf := &mock.TraverserFactory{
		MakeFromSJCFunc: func(sjc proto.SuspendedJobChain) (chain.Traverser, error) {
			return nil, mock.ErrTraverser
		},
	}
	setup(tf)
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	sjc := proto.SuspendedJobChain{
		RequestId:         "abc",
		JobChain:          &jobChain,
		TotalJobTries:     map[string]uint{},
		LatestRunJobTries: map[string]uint{},
		SequenceTries:     map[string]uint{},
	}
	payload, err := json.Marshal(sjc)
	if err != nil {
		t.Fatal(err)
	}

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains/resume", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}
}

func TestResumeJobChainMultipleTraverser(t *testing.T) {
	setup(&mock.TraverserFactory{})
	defer cleanup()

	jobChain := proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	sjc := proto.SuspendedJobChain{
		RequestId:         "abc",
		JobChain:          &jobChain,
		TotalJobTries:     map[string]uint{},
		LatestRunJobTries: map[string]uint{},
		SequenceTries:     map[string]uint{},
	}
	payload, err := json.Marshal(sjc)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a key into the repo that will cause an "already exists" error.
	traverserRepo.Set(sjc.RequestId, "blah")

	statusCode, _, err := testutil.MakeHTTPRequest("POST", baseURL()+"job-chains/resume", payload, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusBadRequest {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusBadRequest)
	}
}

func TestStopJobChainHandlerNotFoundError(t *testing.T) {
	requestId := "abcd1234"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	// A key exists in the traverser repo, but the value isn't of type Traverser.
	traverserRepo.Set(requestId, "blah")

	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"job-chains/"+requestId+"/stop", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}
}

func TestStopJobChainHandlerStopError(t *testing.T) {
	requestId := "abcd1234"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	// Insert a mock traverser into the repo that will return an error on Stop.
	trav := &mock.Traverser{StopErr: mock.ErrTraverser}
	traverserRepo.Set(requestId, trav)

	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"job-chains/"+requestId+"/stop", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}
}

func TestStopJobChainHandlerSuccess(t *testing.T) {
	requestId := "abcd1234"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	// Insert a mock traverser into the repo that will not return an error on Stop.
	trav := &mock.Traverser{}
	traverserRepo.Set(requestId, trav)

	statusCode, _, err := testutil.MakeHTTPRequest("PUT", baseURL()+"job-chains/"+requestId+"/stop", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}
}

func TestStatusJobChainHandlerStatusError(t *testing.T) {
	requestId := "abcd1234"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	// Insert a mock traverser into the repo that will return an error on Stop.
	trav := &mock.Traverser{StatusErr: mock.ErrTraverser}
	traverserRepo.Set(requestId, trav)

	statusCode, _, err := testutil.MakeHTTPRequest("GET", baseURL()+"job-chains/"+requestId+"/status", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusInternalServerError {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusInternalServerError)
	}
}

func TestStatusJobChainHandlerSuccess(t *testing.T) {
	requestId := "abcd1234"
	setup(&mock.TraverserFactory{})
	defer cleanup()

	chainStatus := proto.JobChainStatus{
		RequestId: requestId,
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{JobId: "job2", Status: "", State: proto.STATE_FAIL},
			proto.JobStatus{JobId: "job3", Status: "95% complete", State: proto.STATE_RUNNING},
		},
	}

	// Insert a mock traverser into the repo that will return a status object on Status.
	trav := &mock.Traverser{
		StatusResp: chainStatus,
	}
	traverserRepo.Set(requestId, trav)

	var actualStatus proto.JobChainStatus
	statusCode, _, err := testutil.MakeHTTPRequest("GET", baseURL()+"job-chains/"+requestId+"/status", nil, &actualStatus)
	if err != nil {
		t.Fatal(err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("response status = %d, expected %d", statusCode, http.StatusOK)
	}

	if diff := deep.Equal(actualStatus, chainStatus); diff != nil {
		t.Error(diff)
	}
}
