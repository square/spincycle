// Copyright 2017, Square, Inc.

package chain

import (
	"reflect"
	"sort"
	"testing"

	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test/mock"
)

var noJobData = map[string]interface{}{}

// Return an error when we try to create an invalid chain.
func TestRunErrorNoFirstJob(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": {},
			"job2": {},
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs:          mock.InitJobs(2),
		AdjacencyList: map[string][]string{},
	}
	c := NewChain(jc)
	_, err := NewTraverser(chainRepo, rf, rr, c)
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

// All jobs in the chain complete successfully.
func TestRunComplete(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
			"job2": mock.NewRunner(true, "", nil, nil, noJobData),
			"job3": mock.NewRunner(true, "", nil, nil, noJobData),
			"job4": mock.NewRunner(true, "", nil, nil, noJobData),
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}

	err = traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.JobChain.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}
}

// Not all jobs in the chain complete successfully.
func TestRunNotComplete(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
			"job2": mock.NewRunner(true, "", nil, nil, noJobData),
			"job3": mock.NewRunner(false, "", nil, nil, noJobData),
			"job4": mock.NewRunner(false, "", nil, nil, noJobData),
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}

	err = traverser.Run()
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
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
			"job2": mock.NewRunner(true, "", nil, nil, noJobData),
			"job3": mock.NewRunner(true, "", nil, nil, noJobData),
			"job4": mock.NewRunner(true, "", nil, nil, noJobData),
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {},
		},
	}
	c := NewChain(jc)
	for _, job := range c.JobChain.Jobs {
		job.State = proto.STATE_UNKNOWN
	}
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}

	if err = traverser.Run(); err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.JobChain.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}
}

// Make sure jobData gets updated as we expect.
func TestJobData(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, map[string]interface{}{"k1": "v1", "k2": "v2"}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]interface{}{}),
			"job3": mock.NewRunner(true, "", nil, nil, map[string]interface{}{}),
			"job4": mock.NewRunner(true, "", nil, nil, map[string]interface{}{"k1": "v9"}),
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}
	expectedJobData := map[string]interface{}{"k1": "v9", "k2": "v2"}

	err = traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if !reflect.DeepEqual(jc.Jobs["job4"].Data, expectedJobData) {
		t.Errorf("job4 data = %v, expected %v", jc.Jobs["job4"].Data, expectedJobData)
	}
}

// Stop the traverser and all running jobs.
func TestStop(t *testing.T) {
	chainRepo := NewMemoryRepo()
	runBlock := make(chan struct{})
	stopChan := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, stopChan, noJobData),
			"job2": mock.NewRunner(true, "", runBlock, stopChan, noJobData),
			"job3": mock.NewRunner(true, "", runBlock, stopChan, noJobData),
			"job4": mock.NewRunner(true, "", nil, stopChan, noJobData),
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}
	traverser.stopChan = stopChan

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running. They will run until we close the runBlock chan.
	for {
		if rf.RunnersToReturn["job2"].Running() == true && rf.RunnersToReturn["job3"].Running() == true {
			break
		}
	}

	traverser.Stop()

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

// Error getting a runner from the repo when calling Stop.
func TestStopRepoError(t *testing.T) {
	chainRepo := NewMemoryRepo()
	runBlock := make(chan struct{})
	stopChan := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", runBlock, stopChan, noJobData),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(1),
	}
	c := NewChain(jc)
	runnerRepo := runner.NewRepo()
	runnerRepo.Store = &mock.KVStore{
		GetAllResp: map[string]interface{}{"not a": "runner"},
	}
	traverser, err := NewTraverser(chainRepo, rf, runnerRepo, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}
	traverser.stopChan = stopChan

	// Start the traverser.
	go func() { traverser.Run() }()

	// Wait until job1 is running. It will run until we close the runBlock chan.
	for {
		if rf.RunnersToReturn["job1"].Running() == true {
			break
		}
	}

	err = traverser.Stop()

	if err == nil {
		t.Errorf("err = nil, expected %s", runner.ErrInvalidRunner)
	}
}

// Get the status from all running jobs.
func TestStatus(t *testing.T) {
	chainRepo := NewMemoryRepo()
	runBlock := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "job1 running", nil, nil, noJobData),
			"job2": mock.NewRunner(true, "job2 running", runBlock, nil, noJobData),
			"job3": mock.NewRunner(true, "job3 running", runBlock, nil, noJobData),
			"job4": mock.NewRunner(true, "job4 running", nil, nil, noJobData),
		},
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running. They will run until we close the runBlock chan.
	for {
		if rf.RunnersToReturn["job2"].Running() == true && rf.RunnersToReturn["job3"].Running() == true {
			break
		}
	}

	expectedStatus := proto.JobChainStatus{
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
	if !reflect.DeepEqual(status, expectedStatus) {
		t.Errorf("status = %v, expected %v", status, expectedStatus)
	}

	// Let jobs 2 and 3 finish.
	close(runBlock)
	// Wait for the traverser to finish.
	<-doneChan

	if c.JobChain.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.JobChain.State, proto.STATE_COMPLETE)
	}
}

// Error creating a job runner.
func TestRunJobsRunnerError(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
		},
		MakeErr: mock.ErrRunner,
	}
	rr := runner.NewRepo()
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(1),
	}
	c := NewChain(jc)
	traverser, err := NewTraverser(chainRepo, rf, rr, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}

	// Start consuming from the runJobChan
	go traverser.runJobs()

	// Pass a job to the run channel
	traverser.runJobChan <- c.JobChain.Jobs["job1"]

	// Wait to get a response on the done channel
	resp := <-traverser.doneJobChan

	if resp.State != proto.STATE_FAIL {
		t.Errorf("job state = %d, expected %d", resp.State, proto.STATE_FAIL)
	}
}

// Error adding a runner to the repo.
func TestRunJobsRepoAddError(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, noJobData),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(1),
	}
	c := NewChain(jc)
	runnerRepo := runner.NewRepo()
	runnerRepo.Store = &mock.KVStore{
		AddErr: mock.ErrKVStore,
	}
	traverser, err := NewTraverser(chainRepo, rf, runnerRepo, c)
	if err != nil {
		t.Fatalf("err = %s, expected nil", err)
	}

	// Start consuming from the runJobChan
	go traverser.runJobs()

	// Pass a job to the run channel
	traverser.runJobChan <- c.JobChain.Jobs["job1"]

	// Wait to get a response on the done channel
	resp := <-traverser.doneJobChan

	if resp.State != proto.STATE_FAIL {
		t.Errorf("job state = %d, expected %d", resp.State, proto.STATE_FAIL)
	}
}
