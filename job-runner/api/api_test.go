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

func TestNewJobChain(t *testing.T) {
	api := NewAPI(&router.Router{}, &chain.FakeRepo{}, &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{},
			"job2": &mock.Runner{},
			"job3": &mock.Runner{},
		},
	})
	jobChain := &proto.JobChain{
		RequestId: uint(4),
		Jobs: proto.Jobs{
			proto.Job{"job1", "shell-command", []byte{}},
			proto.Job{"job2", "shell-command", []byte{}},
			proto.Job{"job3", "shell-command", []byte{}},
		},
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

func TestStopJobChain(t *testing.T) {
	api := NewAPI(&router.Router{}, &chain.FakeRepo{}, &mock.RunnerFactory{RunnersToReturn: map[string]*mock.Runner{}})
	mockTraverser := &mock.Traverser{}
	api.activeT = map[uint]chain.Traverser{
		uint(4): mockTraverser,
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

	if len(api.activeT) != 0 {
		t.Errorf("Traverser was not removed from activeT as expected.")
	}
}

func TestStopJobChainNotRunning(t *testing.T) {
	api := NewAPI(&router.Router{}, &chain.FakeRepo{}, &mock.RunnerFactory{RunnersToReturn: map[string]*mock.Runner{}})
	api.activeT = map[uint]chain.Traverser{}

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
	api := NewAPI(&router.Router{}, &chain.FakeRepo{}, &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{},
			"job2": &mock.Runner{},
			"job3": &mock.Runner{},
		},
	})
	chainStatus := proto.JobChainStatus{
		RequestId:  uint(4),
		ChainState: proto.STATE_RUNNING,
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{"job1", "", proto.STATE_COMPLETE},
			proto.JobStatus{"job2", "", proto.STATE_FAIL},
			proto.JobStatus{"job3", "95% complete", proto.STATE_RUNNING},
		},
	}
	mockTraverser := &mock.Traverser{
		StatusResp: chainStatus,
	}
	api.activeT = map[uint]chain.Traverser{
		uint(4): mockTraverser,
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
