// Copyright 2017-2018, Square, Inc.

package chain_test

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

// Return an error when we try to create an invalid chain.
func TestRunErrorNoFirstJob(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{}
	rmc := &mock.RMClient{}
	f := chain.NewTraverserFactory(chainRepo, rf, rmc)

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
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
	}
	rmc := &mock.RMClient{}

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
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}

	_, err = chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Not all jobs in the chain complete successfully.
func TestRunNotComplete(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}},
		},
	}
	rmc := &mock.RMClient{}

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
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_FAIL)
	}
	if c.JobState("job4") != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobState("job4"), proto.STATE_PENDING)
	}

	_, err = chainRepo.Get("abc")
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

func TestRetrySequence(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	// Job in middle of sequence fails
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_PENDING}},
		},
	}
	rmc := &mock.RMClient{}

	jobs := testutil.InitJobsWithSequenceRetry(4, 2)

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_FAIL)
	}

	job := jobs["job3"]
	expect := uint(2)
	actual := c.SequenceRetryCount(job)
	if actual != expect {
		t.Errorf("sequence retried = %d, expected %d", actual, expect)

	}
	_, err = chainRepo.Get("abc")
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

func TestRetrySequenceFirstJobFailed(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	// Job at start of sequence fails
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_PENDING}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_PENDING}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_PENDING}},
		},
	}
	rmc := &mock.RMClient{}

	jobs := testutil.InitJobsWithSequenceRetry(4, 2)

	jc := &proto.JobChain{
		RequestId: "abc",
		Jobs:      jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_FAIL)
	}

	job := jobs["job3"]
	expect := uint(2)
	actual := c.SequenceRetryCount(job)
	if actual != expect {
		t.Errorf("sequence retried = %d, expected %d", actual, expect)

	}
	_, err = chainRepo.Get("abc")
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Unknown job state should not cause the traverser to panic when running.
func TestJobUnknownState(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
	}
	rmc := &mock.RMClient{}

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
	for _, j := range jc.Jobs {
		j.State = proto.STATE_UNKNOWN
	}
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	if err := traverser.Run(); err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}

	_, err := chainRepo.Get("abc")
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Make sure jobData gets updated as we expect.
func TestJobData(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, AddedJobData: map[string]interface{}{"k1": "v1", "k2": "v2"}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, AddedJobData: map[string]interface{}{}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, AddedJobData: map[string]interface{}{}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, AddedJobData: map[string]interface{}{"k1": "v9"}},
		},
	}
	rmc := &mock.RMClient{}

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
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	expectedJobData := map[string]interface{}{"k1": "v9", "k2": "v2"}

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if diff := deep.Equal(jc.Jobs["job4"].Data, expectedJobData); diff != nil {
		t.Error(diff)
	}
}

// Error creating a runner.
func TestRunJobsRunnerError(t *testing.T) {
	requestId := "abc"
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
		// This is what causes the error, even though the job returns STATE_COMPLETE
		MakeErr: mock.ErrRunner,
	}
	var recvdjl proto.JobLog // record the jl that gets sent to the RM
	rmc := &mock.RMClient{
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
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if jc.State != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", jc.State, proto.STATE_FAIL)
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
	if recvdjl.StartedAt != 0 {
		t.Errorf("jobLog.StartedAt = %d, expected 0", recvdjl.StartedAt)
	}
	if recvdjl.FinishedAt != 0 {
		t.Errorf("jobLog.Finished = %d, expected 0", recvdjl.FinishedAt)
	}

	_, err = chainRepo.Get("abc")
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Stop the traverser and all running jobs.
func TestStop(t *testing.T) {
	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(2)
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}, RunBlock: make(chan struct{}), RunWg: &runWg},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}, RunBlock: make(chan struct{}), RunWg: &runWg},
		},
	}
	rmc := &mock.RMClient{}

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
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will run
	// until Stop is called (which will close their RunBlock channels).
	runWg.Wait()

	err := traverser.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Wait for the traverser to finish.
	<-doneChan

	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}
	if c.JobState("job2") != proto.STATE_FAIL {
		t.Errorf("job2 state = %d, expected %d", c.JobState("job2"), proto.STATE_FAIL)
	}
	if c.JobState("job3") != proto.STATE_FAIL {
		t.Errorf("job3 state = %d, expected %d", c.JobState("job3"), proto.STATE_FAIL)
	}
	if c.JobState("job4") != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobState("job4"), proto.STATE_PENDING)
	}

	_, err = chainRepo.Get("abc")
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Get the status from all running jobs.
func TestStatus(t *testing.T) {
	now := time.Now().UnixNano()

	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(2)
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job1 running"},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job2 running", RunBlock: make(chan struct{}), RunWg: &runWg},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job3 running", RunBlock: make(chan struct{}), RunWg: &runWg},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job4 running"},
		},
	}
	rmc := &mock.RMClient{}

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
	traverser := chain.NewTraverser(c, chainRepo, rf, rmc)

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will run
	// until Status is called (which will close their RunBlock channels).
	runWg.Wait()

	expectedStatus := proto.JobChainStatus{
		RequestId: "abc",
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{
				RequestId: "abc",
				JobId:     "job2",
				State:     proto.STATE_RUNNING,
				Status:    "job2 running",
				N:         0, // see below
				Args:      map[string]interface{}{},
			},
			proto.JobStatus{
				RequestId: "abc",
				JobId:     "job3",
				State:     proto.STATE_RUNNING,
				Status:    "job3 running",
				N:         0, // see below
				Args:      map[string]interface{}{},
			},
		},
	}
	status, err := traverser.Status()
	sort.Sort(status.JobStatuses)

	for i, j := range status.JobStatuses {
		if j.StartedAt < now {
			t.Error("StartedAt <= 0: %+v", j)
		}
		status.JobStatuses[i].StartedAt = 0

		// Don't know if job2 or job3 will run first. They're enqueued at same
		// time, but no way to guarantee which makes in into queue and runs first.
		// But either way, N should be only 2 or 3 because they're are the 2nd
		// and 3rd jobs ran.
		if j.N != 2 && j.N != 3 {
			t.Error("got N = %d, expected 2 or 3", j.N)
		}
		status.JobStatuses[i].N = 0
	}

	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if diff := deep.Equal(status, expectedStatus); diff != nil {
		t.Error(diff)
	}

	// Wait for the traverser to finish.
	<-doneChan

	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}
}
