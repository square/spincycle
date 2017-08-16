// Copyright 2017, Square, Inc.

package request_test

import (
	"sort"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/request"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

func TestCreateRequestMissingType(t *testing.T) {
	m := request.NewManager(&mock.Grapher{}, &mock.RequestDBAccessor{}, &mock.JRClient{})

	_, err := m.CreateRequest(proto.CreateRequestParams{})
	if err != request.ErrInvalidParams {
		t.Errorf("err = %s, expected %s", err, request.ErrInvalidParams)
	}
}

func TestCreateRequestResolveError(t *testing.T) {
	// Create a mock request resolver that will return an error.
	grapher := &mock.Grapher{
		CreateGraphFunc: func(reqType string, args map[string]interface{}) (*grapher.Graph, error) {
			return nil, mock.ErrGrapher
		},
	}
	m := request.NewManager(grapher, &mock.RequestDBAccessor{}, &mock.JRClient{})
	reqParams := proto.CreateRequestParams{Type: "foo"}

	_, err := m.CreateRequest(reqParams)
	if err != mock.ErrGrapher {
		t.Errorf("err = %s, expected %s", err, mock.ErrGrapher)
	}
}

func TestCreateRequestDBError(t *testing.T) {
	// Create a mock dbaccessor that will return an error.
	dbAccessor := &mock.RequestDBAccessor{
		SaveRequestFunc: func(proto.Request, proto.CreateRequestParams) error {
			return mock.ErrRequestDBAccessor
		},
	}
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})
	reqParams := proto.CreateRequestParams{Type: "foo"}

	_, err := m.CreateRequest(reqParams)
	if err != mock.ErrRequestDBAccessor {
		t.Errorf("err = %s, expected %s", err, mock.ErrRequestDBAccessor)
	}
}

func TestCreateRequestSuccess(t *testing.T) {
	reqParams := proto.CreateRequestParams{Type: "foo", User: "john"}
	// Create a mock request resolver that will return a graph.
	grapher := &mock.Grapher{
		CreateGraphFunc: func(reqType string, args map[string]interface{}) (*grapher.Graph, error) {
			fNode := &grapher.Node{
				Datum: &mock.Payload{
					NameResp: "job1",
				},
			}
			lNode := &grapher.Node{
				Datum: &mock.Payload{
					NameResp: "job2",
				},
			}
			return &grapher.Graph{
				First: fNode,
				Last:  lNode,
				Vertices: map[string]*grapher.Node{
					"job1": fNode,
					"job2": lNode,
				},
				Edges: map[string][]string{
					"job1": []string{"job2"},
				},
			}, nil
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
	m := request.NewManager(grapher, dbAccessor, &mock.JRClient{})

	actualReq, err := m.CreateRequest(reqParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Make sure the returned request is what we expect.
	if actualReq.Id == "" {
		t.Errorf("request id is an empty string, shoudl not be")
	}
	if actualReq.CreatedAt.IsZero() {
		t.Errorf("request created at is a zero time, should not be")
	}
	expectedJc := proto.JobChain{
		RequestId: actualReq.Id, // no other way of getting this from outside the package
		State:     proto.STATE_PENDING,
		Jobs: map[string]proto.Job{
			"job1": proto.Job{Id: "job1"},
			"job2": proto.Job{Id: "job2"},
		},
		AdjacencyList: map[string][]string{"job1": []string{"job2"}},
	}
	expectedReq := proto.Request{
		Id:        actualReq.Id, // no other way of getting this from outside the package
		Type:      reqParams.Type,
		CreatedAt: actualReq.CreatedAt, // same deal as request id
		State:     proto.STATE_PENDING,
		User:      reqParams.User,
		JobChain:  &expectedJc,
		TotalJobs: 2,
	}
	if diff := deep.Equal(actualReq, expectedReq); diff != nil {
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
}

func TestGetRequestSuccess(t *testing.T) {
	reqId := "abcd1234"
	// Create a mock dbaccessor that returns a request.
	dbAccessor := &mock.RequestDBAccessor{
		GetRequestFunc: func(r string) (proto.Request, error) {
			return proto.Request{Id: r}, nil
		},
	}
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, jrClient)

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})
	finishParams := proto.FinishRequestParams{State: proto.STATE_INCOMPLETE}

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})
	finishParams := proto.FinishRequestParams{State: proto.STATE_INCOMPLETE}

	err := m.FinishRequest(reqId, finishParams)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if dbReq.State != proto.STATE_INCOMPLETE {
		t.Errorf("request state = %d, expected %d", dbReq.State, proto.STATE_INCOMPLETE)
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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, jrClient)

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
		Jobs: testutil.InitJobs(9),
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
				proto.JobStatus{Id: "job1", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job2", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job3", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job4", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job5", State: proto.STATE_FAIL},
				proto.JobStatus{Id: "job6", State: proto.STATE_COMPLETE},
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
					proto.JobStatus{Id: "job4", State: proto.STATE_RUNNING, Status: "running"},
					proto.JobStatus{Id: "job7", State: proto.STATE_RUNNING, Status: "running as well"},
				},
			}, nil
		},
	}
	m := request.NewManager(&mock.Grapher{}, dbAccessor, jrClient)

	actualReqStatus, err := m.RequestStatus(reqId)
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	expectedReqStatus := proto.RequestStatus{
		Request: req,
		JobChainStatus: proto.JobChainStatus{
			JobStatuses: proto.JobStatuses{
				proto.JobStatus{Id: "job1", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job2", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job3", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job4", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job5", State: proto.STATE_FAIL},
				proto.JobStatus{Id: "job6", State: proto.STATE_COMPLETE},
				proto.JobStatus{Id: "job7", State: proto.STATE_RUNNING, Status: "running as well"},
				proto.JobStatus{Id: "job8", State: proto.STATE_PENDING},
				proto.JobStatus{Id: "job9", State: proto.STATE_PENDING},
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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
	jl := proto.JobLog{RequestId: reqId, State: proto.STATE_INCOMPLETE}
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
	m := request.NewManager(&mock.Grapher{}, dbAccessor, &mock.JRClient{})

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
