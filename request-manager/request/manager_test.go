// Copyright 2017, Square, Inc.

package request_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

var abcSpec *grapher.Config
var testJobFactory *mock.JobFactory
var testGrapher *grapher.Grapher

func setup() {
	testJobFactory = &mock.JobFactory{
		MockJobs: map[string]*mock.Job{},
	}
	for i, c := range []string{"a", "b", "c"} {
		jobType := c + "JobType"
		testJobFactory.MockJobs[jobType] = &mock.Job{
			NameResp: fmt.Sprintf("%s@%d", c, i),
			TypeResp: jobType,
		}
	}
	testJobFactory.MockJobs["aJobType"].SetJobArgs = map[string]interface{}{
		"aArg": "aValue",
	}
	testGrapher = grapher.NewGrapher(testJobFactory, abcSpec)
}

func init() {
	var err error
	abcSpec, err = grapher.ReadConfig("../test/specs/a-b-c.yaml")
	if err != nil {
		panic(err)
	}
	setup()
}

func TestCreateRequestMissingType(t *testing.T) {
	m := request.NewManager(testGrapher, &mock.RequestDBAccessor{}, &mock.JRClient{})

	_, err := m.CreateRequest(proto.CreateRequestParams{})
	if err != request.ErrInvalidParams {
		t.Errorf("err = %s, expected %s", err, request.ErrInvalidParams)
	}
}

func TestCreateRequestDBError(t *testing.T) {
	// Create a mock dbaccessor that will return an error.
	dbAccessor := &mock.RequestDBAccessor{
		SaveRequestFunc: func(proto.Request, proto.CreateRequestParams) error {
			return mock.ErrRequestDBAccessor
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})
	reqParams := proto.CreateRequestParams{Type: "three-nodes", Args: map[string]interface{}{"foo": 1}}

	_, err := m.CreateRequest(reqParams)
	if err != mock.ErrRequestDBAccessor {
		t.Errorf("err = %s, expected %s", err, mock.ErrRequestDBAccessor)
	}
}

func TestCreateRequestSuccess(t *testing.T) {
	setup()

	// testGrapher uses spec a-b-c.yaml which has reqest "three-nodes"
	reqParams := proto.CreateRequestParams{
		Type: "three-nodes",
		User: "john",
		Args: map[string]interface{}{
			"foo": "foo-value",
		},
	}

	// Create a mock dbaccessor that records the rawJc and rawParams it receives.
	var dbRequest proto.Request
	var dbReqParams proto.CreateRequestParams
	dbAccessor := &mock.RequestDBAccessor{
		SaveRequestFunc: func(req proto.Request, reqParams proto.CreateRequestParams) error {
			dbRequest = req
			dbReqParams = reqParams
			return nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	actualReq, err := m.CreateRequest(reqParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Make sure the returned request is what we expect.
	if actualReq.Id == "" {
		t.Errorf("request id is an empty string, expected it to be set")
	}
	if actualReq.CreatedAt.IsZero() {
		t.Errorf("request created at is a zero time, should not be")
	}

	// Job names in requests are non-deterministic because the nodes in a sequence
	// are built from a map (i.e. hash order randomness). So sometimes we get a@3
	// and other times a@4, etc. So we'll check some specific, deterministic stuff.
	// But an example of a job chain is shown in the comment block below.
	actualJobChain := actualReq.JobChain
	actualReq.JobChain = nil
	dbRequest.JobChain = nil

	/*
		expectedJc := proto.JobChain{
			RequestId: actualReq.Id, // no other way of getting this from outside the package
			State:     proto.STATE_PENDING,
			Jobs: map[string]proto.Job{
				"sequence_three-nodes_start@1": proto.Job{
					Id:   "sequence_three-nodes_start@1",
					Type: "no-op",
				},
				"a@3": proto.Job{
					Id:        "a@3",
					Type:      "aJobType",
					Retry:     1,
					RetryWait: 500,
				},
				"b@4": proto.Job{
					Id:    "b@4",
					Type:  "bJobType",
					Retry: 3,
				},
				"c@5": proto.Job{
					Id:   "c@5",
					Type: "cJobType",
				},
				"sequence_three-nodes_end@2": proto.Job{
					Id:   "sequence_three-nodes_end@2",
					Type: "no-op",
				},
			},
			AdjacencyList: map[string][]string{
				"sequence_three-nodes_start@1": []string{"a@3"},
				"a@3": []string{"b@4"},
				"b@4": []string{"c@5"},
				"c@5": []string{"sequence_three-nodes_end@2"},
			},
		}
	*/

	expectedReq := proto.Request{
		Id:        actualReq.Id, // no other way of getting this from outside the package
		Type:      reqParams.Type,
		CreatedAt: actualReq.CreatedAt, // same deal as request id
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
		JobChain:  nil,
		TotalJobs: 5,
	}
	if diff := deep.Equal(actualReq, expectedReq); diff != nil {
		test.Dump(actualReq)
		t.Error(diff)
	}

	// Make sure the request sent to the db is what we expect.
	if diff := deep.Equal(dbRequest, actualReq); diff != nil {
		t.Error(diff)
	}

	// Make sure the request params sent to the db is what we expect.
	if diff := deep.Equal(dbReqParams, reqParams); diff != nil {
		t.Error(diff)
	}

	// Check the job chain
	if actualJobChain.RequestId == "" {
		t.Error("job chain RequestId not set, expected it to be set")
	}
	if actualJobChain.State != proto.STATE_PENDING {
		t.Error("job chain state = %s, expected PENDING", proto.StateName[actualJobChain.State])
	}
	if len(actualJobChain.Jobs) != 5 {
		test.Dump(actualJobChain.Jobs)
		t.Error("job chain has %d jobs, expected 5", len(actualJobChain.Jobs))
	}
	if len(actualJobChain.AdjacencyList) != 4 {
		test.Dump(actualJobChain.Jobs)
		t.Error("job chain AdjacencyList len = %d, expected 4", len(actualJobChain.AdjacencyList))
	}

}

func TestGetRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{Id: r}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	actualReq, err := m.GetRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if actualReq.Id != reqId {
		t.Errorf("request id = %s, expected %s", actualReq.Id, reqId)
	}
}

func TestStartRequestInvalidState(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{State: proto.STATE_RUNNING}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	err := m.StartRequest(reqId)
	if err != nil {
		if _, ok := err.(request.ErrInvalidState); !ok {
			t.Errorf("err = %s, expected a type of request.ErrInvalidState")
		}
	} else {
		t.Errorf("expected an error but did not get one")
	}
}

func TestStartRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	jc := proto.JobChain{RequestId: reqId}
	// Create a mock dbaccessor that returns a request and a job chain, and
	// records the update params it receives.
	var dbReq proto.Request
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{State: proto.STATE_PENDING}, nil
		},
		GetJobChainFunc: func(r string) (proto.JobChain, error) {
			return jc, nil
		},
		UpdateRequestFunc: func(r proto.Request) error {
			dbReq = r
			return nil
		},
	}
	// Create a mock jr client that records the job chain it receives.
	var jrJc proto.JobChain
	jrClient := &mock.JRClient{
		NewJobChainFunc: func(j proto.JobChain) error {
			jrJc = j
			return nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, jrClient)

	err := m.StartRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if dbReq.State != proto.STATE_RUNNING {
		t.Errorf("request state = %d, expected %d", dbReq.State, proto.STATE_RUNNING)
	}
	if dbReq.StartedAt.IsZero() {
		t.Errorf("request started at is a zero time, should not be")
	}

	if diff := deep.Equal(jrJc, jc); diff != nil {
		t.Error(diff)
	}
}

func TestFinishRequestInvalidState(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{State: proto.STATE_PENDING}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})
	finishParams := proto.FinishRequestParams{State: proto.STATE_FAIL}

	err := m.FinishRequest(reqId, finishParams)
	if err != nil {
		if _, ok := err.(request.ErrInvalidState); !ok {
			t.Errorf("err = %s, expected a type of request.ErrInvalidState")
		}
	} else {
		t.Errorf("expected an error but did not get one")
	}
}

func TestFinishRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request and records the update
	// params it receives.
	var dbReq proto.Request
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{State: proto.STATE_RUNNING}, nil
		},
		UpdateRequestFunc: func(r proto.Request) error {
			dbReq = r
			return nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})
	finishParams := proto.FinishRequestParams{State: proto.STATE_FAIL}

	err := m.FinishRequest(reqId, finishParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if dbReq.State != proto.STATE_FAIL {
		t.Errorf("request state = %d, expected %d", dbReq.State, proto.STATE_FAIL)
	}
	if dbReq.FinishedAt.IsZero() {
		t.Errorf("request started at is a zero time, should not be")
	}
}

func TestStopRequestInvalidState(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{State: proto.STATE_PENDING}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	err := m.StopRequest(reqId)
	if err != nil {
		if _, ok := err.(request.ErrInvalidState); !ok {
			t.Errorf("err = %s, expected a type of request.ErrInvalidState")
		}
	} else {
		t.Errorf("expected an error but did not get one")
	}
}

func TestStopRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{State: proto.STATE_RUNNING}, nil
		},
	}
	// Create a mock jr client that records the request id it receives.
	var jrReqId string
	jrClient := &mock.JRClient{
		StopRequestFunc: func(r string) error {
			jrReqId = r
			return nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, jrClient)

	err := m.StopRequest(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if jrReqId != reqId {
		t.Errorf("request id sent to jr = %d, expected %d", jrReqId, reqId)
	}
}

func TestRequestStatusRunning(t *testing.T) {
	reqId := "abcd1234"
	jc := proto.JobChain{
		Jobs: test.InitJobs(9),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4", "job5", "job6"},
			"job3": []string{"job6"},
			"job4": []string{"job9"},
			"job5": []string{"job9"},
			"job6": []string{"job9"},
			"job7": []string{"job8"},
			"job8": []string{"job9"},
		},
	}
	req := proto.Request{State: proto.STATE_RUNNING}
	// Create a mock dbaccessor that returns a request and job statuses.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return req, nil
		},
		GetJobChainFunc: func(r string) (proto.JobChain, error) {
			return jc, nil
		},
		GetRequestJobStatusesFunc: func(req string) (proto.JobStatuses, error) {
			return proto.JobStatuses{
				proto.JobStatus{JobId: "job1", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job2", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job3", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job4", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job5", State: proto.STATE_FAIL},
				proto.JobStatus{JobId: "job6", State: proto.STATE_COMPLETE},
			}, nil
		},
	}
	// Create a mock jr client that returns a job chain status.
	jrClient := &mock.JRClient{
		RequestStatusFunc: func(r string) (proto.JobChainStatus, error) {
			return proto.JobChainStatus{
				RequestId: reqId,
				JobStatuses: proto.JobStatuses{
					// jobs 4 and 7 are running. notice above, however, that job4
					// is also "finished". This means that between the time we asked
					// the JR for status, and we got the finished jobs from the db,
					// job 4 moved from running to being finished.
					proto.JobStatus{JobId: "job4", State: proto.STATE_RUNNING, Status: "running"},
					proto.JobStatus{JobId: "job7", State: proto.STATE_RUNNING, Status: "running as well"},
				},
			}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, jrClient)

	actualReqStatus, err := m.RequestStatus(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedReqStatus := proto.RequestStatus{
		Request: req,
		JobChainStatus: proto.JobChainStatus{
			JobStatuses: proto.JobStatuses{
				proto.JobStatus{JobId: "job1", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job2", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job3", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job4", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job5", State: proto.STATE_FAIL},
				proto.JobStatus{JobId: "job6", State: proto.STATE_COMPLETE},
				proto.JobStatus{JobId: "job7", State: proto.STATE_RUNNING, Status: "running as well"},
				proto.JobStatus{JobId: "job8", State: proto.STATE_PENDING},
				proto.JobStatus{JobId: "job9", State: proto.STATE_PENDING},
			},
		},
	}

	sort.Sort(expectedReqStatus.JobChainStatus.JobStatuses)
	sort.Sort(actualReqStatus.JobChainStatus.JobStatuses)

	if diff := deep.Equal(actualReqStatus, expectedReqStatus); diff != nil {
		t.Error(diff)
	}
}

func TestGetJobChainSuccess(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a job chain.
	dbAccessor := &mock.RequestDBAccessor{
		GetJobChainFunc: func(r string) (proto.JobChain, error) {
			return proto.JobChain{RequestId: r}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	actualJc, err := m.GetJobChain(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if actualJc.RequestId != reqId {
		t.Errorf("request id = %s, expected %s", actualJc.RequestId, reqId)
	}
}

func TestGetJLSuccess(t *testing.T) {
	reqId := "abcd1234"
	jobId := "job1"
	// Create a mock dbaccessor that returns a jl.
	dbAccessor := &mock.RequestDBAccessor{
		GetLatestJLFunc: func(r, j string) (proto.JobLog, error) {
			return proto.JobLog{RequestId: r, JobId: j}, nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	actualjl, err := m.GetJL(reqId, jobId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if actualjl.RequestId != reqId {
		t.Errorf("request id = %s, expected %s", actualjl.RequestId, reqId)
	}
	if actualjl.JobId != jobId {
		t.Errorf("job id = %s, expected %s", actualjl.JobId, jobId)
	}
}

func TestCreateJLComplete(t *testing.T) {
	reqId := "abcd1234"
	jl := proto.JobLog{RequestId: reqId, State: proto.STATE_COMPLETE}
	// Create a mock dbaccessor that records the jl it receieves, and also
	// records if it increments the finished jobs count of a request.
	var dbjl proto.JobLog
	var dbReqId string
	dbAccessor := &mock.RequestDBAccessor{
		SaveJLFunc: func(j proto.JobLog) error {
			dbjl = j
			return nil
		},
		IncrementRequestFinishedJobsFunc: func(r string) error {
			dbReqId = r
			return nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	actualjl, err := m.CreateJL(reqId, jl)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(actualjl, jl); diff != nil {
		t.Error(diff)
	}

	if diff := deep.Equal(dbjl, jl); diff != nil {
		t.Error(diff)
	}
	if dbReqId != reqId {
		t.Errorf("request id = %s, expected %s", dbReqId, reqId)
	}
}

func TestCreateJLNotComplete(t *testing.T) {
	reqId := "abcd1234"
	jl := proto.JobLog{RequestId: reqId, State: proto.STATE_FAIL}
	// Create a mock dbaccessor that records the jl it receieves, and also
	// records if it increments the finished jobs count of a request.
	var dbjl proto.JobLog
	var incrementCalled bool
	dbAccessor := &mock.RequestDBAccessor{
		SaveJLFunc: func(j proto.JobLog) error {
			dbjl = j
			return nil
		},
		IncrementRequestFinishedJobsFunc: func(r string) error {
			incrementCalled = true
			return nil
		},
	}
	m := request.NewManager(testGrapher, dbAccessor, &mock.JRClient{})

	actualjl, err := m.CreateJL(reqId, jl)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(actualjl, jl); diff != nil {
		t.Error(diff)
	}

	if diff := deep.Equal(dbjl, jl); diff != nil {
		t.Error(diff)
	}
	if incrementCalled {
		t.Errorf("incrementCalled = true, expected false")
	}
}
