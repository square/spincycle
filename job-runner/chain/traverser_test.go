// Copyright 2017, Square, Inc.

package chain_test

import (
	"sort"
	"sync"
	"testing"

	"github.com/go-test/deep"
	job "github.com/square/spincycle/job"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

// Return an error when we try to create an invalid chain.
func TestRunErrorNoFirstJob(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	jf := &mock.JobFactory{}
	rmClient := &mock.RMClient{}
	f := chain.NewTraverserFactory(chainRepo, jf, rmClient)

	jc := proto.JobChain{
		RequestId:     "abc",
		Jobs:          testutil.InitJobs(2),
		AdjacencyList: map[string][]string{},
	}
	tr, err := f.Make(jc)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
	if tr != nil {
		t.Errorf("got non-nil Traverser, expected nil on error")
	}
}

// All jobs in the chain complete successfully.
func TestRunComplete(t *testing.T) {
	requestId := "abc"
	chainRepo := chain.NewMemoryRepo()
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job2": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job3": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job4": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
		},
	}
	var recvdjl proto.JobLog // record the last jl that gets sent to the RM
	rmClient := &mock.RMClient{
		CreateJLFunc: func(reqId string, jl proto.JobLog) error {
			if reqId == requestId && jl.JobId == "job4" { // only check one of the jobs
				recvdjl = jl
				return nil
			}
			return mock.ErrRMClient
		},
	}

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.JobChain.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}

	// Make sure the JL sent to the RM matches what we expect.
	if recvdjl.RequestId != requestId {
		t.Errorf("jl request id = %d, expected %d", recvdjl.RequestId, requestId)
	}
	if recvdjl.JobId != "job4" {
		t.Errorf("jl job id = %d, expected %d", recvdjl.JobId, "job4")
	}
	if recvdjl.State != proto.STATE_COMPLETE {
		t.Errorf("jl state = %d, expected %d", recvdjl.State, proto.STATE_COMPLETE)
	}
	if recvdjl.Error != "" {
		t.Errorf("jl error = %d, expected an empty string", recvdjl.Error)
	}
	if recvdjl.StartedAt == nil || recvdjl.StartedAt.IsZero() {
		t.Errorf("jl started at value is not a non-zero time")
	}
	if recvdjl.FinishedAt == nil || recvdjl.FinishedAt.IsZero() {
		t.Errorf("jl finished at value is not a non-zero time")
	}
}

// Not all jobs in the chain complete successfully.
func TestRunNotComplete(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job2": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job3": &mock.Job{RunReturn: job.Return{State: proto.STATE_FAIL}},
			"job4": &mock.Job{RunReturn: job.Return{State: proto.STATE_FAIL}},
		},
	}
	rmClient := &mock.RMClient{}

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.JobChain.State != proto.STATE_INCOMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_INCOMPLETE)
	}
	if c.JobChain.Jobs["job4"].State != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobChain.Jobs["job4"].State, proto.STATE_PENDING)
	}
}

// Unknown job state should not cause the traverser to panic when running.
func TestJobUnknownState(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job2": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job3": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job4": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
		},
	}
	rmClient := &mock.RMClient{}

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {},
		},
	}
	c := chain.NewChain(jc)
	for _, j := range c.JobChain.Jobs {
		j.State = proto.STATE_UNKNOWN
	}
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	if err := traverser.Run(); err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.JobChain.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}
}

// Make sure jobData gets updated as we expect.
func TestJobData(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE},
				AddedJobData: map[string]interface{}{"k1": "v1", "k2": "v2"}},
			"job2": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE},
				AddedJobData: map[string]interface{}{}},
			"job3": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE},
				AddedJobData: map[string]interface{}{}},
			"job4": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE},
				AddedJobData: map[string]interface{}{"k1": "v9"}},
		},
	}
	rmClient := &mock.RMClient{}

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	expectedJobData := map[string]interface{}{"k1": "v9", "k2": "v2"}

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(jc.Jobs["job4"].Data, expectedJobData); diff != nil {
		t.Error(diff)
	}
}

// Error creating a job.
func TestRunJobsRunnerError(t *testing.T) {
	requestId := "abc"
	chainRepo := chain.NewMemoryRepo()
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
		},
		// This is what causes the error, even though the job returns STATE_COMPLETE
		MakeErr: mock.ErrJob,
	}
	var recvdjl proto.JobLog // record the jl that gets sent to the RM
	rmClient := &mock.RMClient{
		CreateJLFunc: func(reqId string, jl proto.JobLog) error {
			if reqId == requestId {
				recvdjl = jl
				return nil
			}
			return mock.ErrRMClient
		},
	}

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(1),
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if jc.State != proto.STATE_INCOMPLETE {
		t.Errorf("chain state = %d, expected %d", jc.State, proto.STATE_INCOMPLETE)
	}

	// Make sure the JL sent to the RM matches what we expect.
	if recvdjl.RequestId != requestId {
		t.Errorf("jl request id = %d, expected %d", recvdjl.RequestId, requestId)
	}
	if recvdjl.JobId != "job1" {
		t.Errorf("jl job id = %d, expected %d", recvdjl.JobId, "job1")
	}
	if recvdjl.State != proto.STATE_FAIL {
		t.Errorf("jl state = %d, expected %d", recvdjl.State, proto.STATE_FAIL)
	}
	if recvdjl.Error == "" {
		t.Errorf("jl error is empty, expected something")
	}
	if recvdjl.StartedAt == nil || recvdjl.StartedAt.IsZero() {
		t.Errorf("jl started at value is not a non-zero time")
	}
	if recvdjl.FinishedAt == nil || recvdjl.FinishedAt.IsZero() {
		t.Errorf("jl finished at value is not a non-zero time")
	}
}

// Stop the traverser and all running jobs.
func TestStop(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	runBlock := make(chan struct{})
	var runWg sync.WaitGroup
	runWg.Add(2)
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}},
			"job2": &mock.Job{RunReturn: job.Return{State: proto.STATE_FAIL}, RunBlock: runBlock, RunWg: &runWg},
			"job3": &mock.Job{RunReturn: job.Return{State: proto.STATE_FAIL}, RunBlock: runBlock, RunWg: &runWg},
			"job4": &mock.Job{},
		},
	}
	rmClient := &mock.RMClient{}

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will run
	// until we close the runBlock chan.
	runWg.Wait()

	traverser.Stop()

	// Let jobs 2 and 3 finish.
	close(runBlock)
	// Wait for the traverser to finish.
	<-doneChan

	if c.JobChain.State != proto.STATE_INCOMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}
	if c.JobChain.Jobs["job2"].State != proto.STATE_FAIL {
		t.Errorf("job2 state = %d, expected %d", c.JobChain.Jobs["job2"].State, proto.STATE_FAIL)
	}
	if c.JobChain.Jobs["job3"].State != proto.STATE_FAIL {
		t.Errorf("job3 state = %d, expected %d", c.JobChain.Jobs["job3"].State, proto.STATE_FAIL)
	}
	if c.JobChain.Jobs["job4"].State != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobChain.Jobs["job4"].State, proto.STATE_PENDING)
	}
}

// Get the status from all running jobs.
func TestStatus(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	runBlock := make(chan struct{})
	var runWg sync.WaitGroup
	runWg.Add(2)
	jf := &mock.JobFactory{
		JobsToReturn: map[string]*mock.Job{
			"job1": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}, StatusResp: "job1 running"},
			"job2": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE},
				RunBlock: runBlock, RunWg: &runWg, StatusResp: "job2 running"},
			"job3": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE},
				RunBlock: runBlock, RunWg: &runWg, StatusResp: "job3 running"},
			"job4": &mock.Job{RunReturn: job.Return{State: proto.STATE_COMPLETE}, StatusResp: "job4 running"},
		},
	}
	rmClient := &mock.RMClient{}

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, jf, rmClient)

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will run
	// until we close the runBlock chan.
	runWg.Wait()

	expectedStatus := proto.JobChainStatus{
		RequestId: "abc",
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{"job2", "job2 running", proto.STATE_RUNNING},
			proto.JobStatus{"job3", "job3 running", proto.STATE_RUNNING},
		},
	}
	status, err := traverser.Status()
	sort.Sort(status.JobStatuses)

	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if diff := deep.Equal(status, expectedStatus); diff != nil {
		t.Error(diff)
	}

	// Let jobs 2 and 3 finish.
	close(runBlock)
	// Wait for the traverser to finish.
	<-doneChan

	if c.JobChain.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}
}
