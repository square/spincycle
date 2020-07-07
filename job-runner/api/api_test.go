// Copyright 2017-2019, Square, Inc.

package api_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/orcaman/concurrent-map"

	"github.com/square/spincycle/v2/job-runner/api"
	"github.com/square/spincycle/v2/job-runner/app"
	"github.com/square/spincycle/v2/job-runner/chain"
	"github.com/square/spincycle/v2/proto"
	testutil "github.com/square/spincycle/v2/test"
	"github.com/square/spincycle/v2/test/mock"
	v "github.com/square/spincycle/v2/version"
)

var (
	server        *httptest.Server
	traverserRepo cmap.ConcurrentMap
	shutdownChan  chan struct{}
)

func setup(traverserFactory *mock.TraverserFactory) {
	traverserRepo = cmap.New()
	shutdownChan = make(chan struct{})
	appCtx := app.Defaults()
	baseURL, _ := appCtx.Hooks.ServerURL(appCtx)
	apiCfg := api.Config{
		AppCtx:           appCtx,
		TraverserFactory: traverserFactory,
		TraverserRepo:    traverserRepo,
		StatusManager:    &mock.JRStatus{},
		ShutdownChan:     shutdownChan,
		BaseURL:          baseURL,
	}
	api := api.NewAPI(apiCfg)
	server = httptest.NewServer(api)
}

func setupWithCtx(traverserFactory *mock.TraverserFactory, ctx app.Context) {
	traverserRepo = cmap.New()
	shutdownChan = make(chan struct{})
	baseURL, _ := ctx.Hooks.ServerURL(ctx)
	cfg := api.Config{
		AppCtx:           ctx,
		TraverserFactory: traverserFactory,
		TraverserRepo:    traverserRepo,
		StatusManager:    &mock.JRStatus{},
		ShutdownChan:     shutdownChan,
		BaseURL:          baseURL,
	}
	api := api.NewAPI(cfg)
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
		MakeFunc: func(jc *proto.JobChain) (chain.Traverser, error) {
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
	ctx := app.Defaults()
	ctx.Config.Server.Addr = "host:port"
	setupWithCtx(&mock.TraverserFactory{}, ctx)
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

	expectedLocation := "http://" + ctx.Config.Server.Addr + "/api/v1/job-chains/" + requestId
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
	ctx := app.Defaults()
	ctx.Config.Server.Addr = "host:port"
	setupWithCtx(&mock.TraverserFactory{}, ctx)
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

	expectedLocation := "http://" + ctx.Config.Server.Addr + "/api/v1/job-chains/" + requestId
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
		MakeFromSJCFunc: func(sjc *proto.SuspendedJobChain) (chain.Traverser, error) {
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

func TestGetVersion(t *testing.T) {
	setup(&mock.TraverserFactory{})
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
