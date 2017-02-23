// Copyright 2017, Square, Inc.

package chain

import (
	"reflect"
	"sort"
	"testing"

	"github.com/square/spincycle/job-runner/db"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test/mock"
)

// Return an error when we try to get the first job of the chain.
func TestRunErrorNoFirstJob(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{},
			"job2": &mock.Runner{},
		},
	}
	jc := &proto.JobChain{
		Jobs:          mock.InitJobs(2),
		AdjacencyList: map[string][]string{},
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())

	err := traverser.Run()
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

// All jobs in the chain complete successfully.
func TestRunComplete(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job3": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job4": mock.NewRunner(true, "", nil, nil, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())

	err := traverser.Run()
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
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job3": mock.NewRunner(false, "", nil, nil, map[string]string{}),
			"job4": mock.NewRunner(false, "", nil, nil, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())

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
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job3": mock.NewRunner(true, "", nil, nil, map[string]string{}),
			"job4": mock.NewRunner(true, "", nil, nil, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2"},
			"job2": []string{"job3"},
			"job3": []string{},
		},
	}
	c := NewChain(jc)
	for _, job := range c.JobChain.Jobs {
		job.State = proto.STATE_UNKNOWN
	}
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())

	if err := traverser.Run(); err != nil {
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
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{"k1": "v1", "k2": "v2"}),
			"job2": mock.NewRunner(true, "", nil, nil, map[string]string{"k1": "v8"}),
			"job3": mock.NewRunner(true, "", nil, nil, map[string]string{"k3": "v3"}),
			"job4": mock.NewRunner(true, "", nil, nil, map[string]string{"k1": "v9"}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())
	expectedJobData := map[string]string{"k1": "v9", "k2": "v2", "k3": "v3"}

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if !reflect.DeepEqual(c.JobData, expectedJobData) {
		t.Errorf("jobData = %v, expected %v", c.JobData, expectedJobData)
	}
}

// Stop the traverser and all running jobs.
func TestStop(t *testing.T) {
	chainRepo := NewMemoryRepo()
	runBlock := make(chan struct{})
	stopChan := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, stopChan, map[string]string{}),
			"job2": mock.NewRunner(true, "", runBlock, stopChan, map[string]string{}),
			"job3": mock.NewRunner(true, "", runBlock, stopChan, map[string]string{}),
			"job4": mock.NewRunner(true, "", nil, stopChan, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())
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

// Error getting a runner from the cache when calling Stop.
func TestStopCacheError(t *testing.T) {
	chainRepo := NewMemoryRepo()
	runBlock := make(chan struct{})
	stopChan := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", runBlock, stopChan, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(1),
	}
	c := NewChain(jc)
	cache := &mock.Datastore{
		GetAllResp: map[string]interface{}{"not a": "runner"},
	}
	traverser := NewTraverser(chainRepo, rf, c, cache)
	traverser.stopChan = stopChan

	// Start the traverser.
	go func() { traverser.Run() }()

	// Wait until job1 is running. It will run until we close the runBlock chan.
	for {
		if rf.RunnersToReturn["job1"].Running() == true {
			break
		}
	}

	err := traverser.Stop()

	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

// Get the status from all running jobs.
func TestStatus(t *testing.T) {
	chainRepo := NewMemoryRepo()
	runBlock := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "job1 running", nil, nil, map[string]string{}),
			"job2": mock.NewRunner(true, "job2 running", runBlock, nil, map[string]string{}),
			"job3": mock.NewRunner(true, "job3 running", runBlock, nil, map[string]string{}),
			"job4": mock.NewRunner(true, "job4 running", nil, nil, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())

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
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
		},
		MakeErr: mock.ErrRunner,
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(1),
	}
	c := NewChain(jc)
	traverser := NewTraverser(chainRepo, rf, c, db.NewMemory())

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

// Error adding a runner to the cache.
func TestRunJobsCacheAddError(t *testing.T) {
	chainRepo := NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": mock.NewRunner(true, "", nil, nil, map[string]string{}),
		},
	}
	jc := &proto.JobChain{
		Jobs: mock.InitJobs(1),
	}
	c := NewChain(jc)
	cache := &mock.Datastore{
		AddErr: mock.ErrDatastore,
	}
	traverser := NewTraverser(chainRepo, rf, c, cache)

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
