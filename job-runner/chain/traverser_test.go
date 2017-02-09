// Copyright 2017, Square, Inc.

package chain

import (
	"reflect"
	"sort"
	"testing"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test/mock"
)

// Return an error when we try to get the root job of the chain.
func TestRunErrorNoRoot(t *testing.T) {
	chainRepo := &FakeRepo{}
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{},
			"job2": &mock.Runner{},
		},
	}
	c := &Chain{
		Jobs:          initJobs(2, true),
		AdjacencyList: map[string][]string{},
	}
	traverser := NewTraverser(chainRepo, rf, c)

	err := traverser.Run()
	if err == nil {
		t.Errorf("expected an error but did not get one")
	}
}

// Return an error when we try to create a job runner.
func TestRunErrorMakeJR(t *testing.T) {
	chainRepo := &FakeRepo{}
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{},
		MakeErr:         mock.ErrRunner,
	}
	c := &Chain{
		Jobs: initJobs(2, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2"},
		},
	}
	traverser := NewTraverser(chainRepo, rf, c)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.Jobs["job1"].State != proto.STATE_FAIL {
		t.Errorf("job1 state = %d, expected %d", c.Jobs["job1"].State, proto.STATE_FAIL)
	}
}

// All jobs in the chain complete successfully.
func TestRunCompleted(t *testing.T) {
	chainRepo := &FakeRepo{}
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunCompleted: true},
			"job2": &mock.Runner{RunCompleted: true},
			"job3": &mock.Runner{RunCompleted: true},
			"job4": &mock.Runner{RunCompleted: true},
		},
	}
	c := &Chain{
		Jobs: initJobs(4, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	traverser := NewTraverser(chainRepo, rf, c)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State, proto.STATE_COMPLETE)
	}
}

// Not all jobs in the chain complete successfully.
func TestRunUncompleted(t *testing.T) {
	chainRepo := &FakeRepo{}
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunCompleted: true},
			"job2": &mock.Runner{RunCompleted: true},
			"job3": &mock.Runner{RunCompleted: false},
			"job4": &mock.Runner{},
		},
	}
	c := &Chain{
		Jobs: initJobs(4, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	traverser := NewTraverser(chainRepo, rf, c)

	err := traverser.Run()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if c.State != proto.STATE_INCOMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State, proto.STATE_INCOMPLETE)
	}
	if c.Jobs["job4"].State != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.Jobs["job4"].State, proto.STATE_PENDING)
	}
}

// Get the status from all running jobs.
func TestStatus(t *testing.T) {
	chainRepo := &FakeRepo{}
	runBlock := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job1 running",
			},
			"job2": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job2 running",
				RunBlock:     runBlock,
			},
			"job3": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job3 running",
				RunBlock:     runBlock,
			},
			"job4": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job4 running",
			},
		},
	}
	c := &Chain{
		Jobs: initJobs(4, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	traverser := NewTraverser(chainRepo, rf, c)

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running. They will run until we close the runBlock chan.
	for {
		traverser.chainMutex.RLock()
		if c.Jobs["job2"].State == proto.STATE_RUNNING && c.Jobs["job3"].State == proto.STATE_RUNNING {
			traverser.chainMutex.RUnlock()
			break
		}
		traverser.chainMutex.RUnlock()
	}

	expectedStatus := proto.JobChainStatus{
		ChainState: proto.STATE_RUNNING,
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{"job1", "", proto.STATE_COMPLETE},
			proto.JobStatus{"job2", "job2 running", proto.STATE_RUNNING},
			proto.JobStatus{"job3", "job3 running", proto.STATE_RUNNING},
			proto.JobStatus{"job4", "", proto.STATE_PENDING},
		},
	}
	status := traverser.Status()
	sort.Sort(status.JobStatuses)

	if !reflect.DeepEqual(status, expectedStatus) {
		t.Errorf("status = %v, expected %v", status, expectedStatus)
	}

	// Let jobs 2 and 3 finish.
	close(runBlock)
	// Wait for the traverser to finish.
	<-doneChan

	if c.State != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State, proto.STATE_COMPLETE)
	}
}

// Stop the traverser and all running jobs.
func TestStop(t *testing.T) {
	chainRepo := &FakeRepo{}
	runBlock := make(chan struct{})
	stopChan := make(chan struct{})
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job1 running",
				StopChan:     stopChan,
			},
			"job2": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job2 running",
				RunBlock:     runBlock,
				StopChan:     stopChan,
			},
			"job3": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job3 running",
				RunBlock:     runBlock,
				StopChan:     stopChan,
			},
			"job4": &mock.Runner{
				RunCompleted: true,
				StatusResp:   "job4 running",
				StopChan:     stopChan,
			},
		},
	}
	c := &Chain{
		Jobs: initJobs(4, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}
	traverser := NewTraverser(chainRepo, rf, c)
	traverser.stopChan = stopChan

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running. They will run until we close the runBlock chan.
	for {
		traverser.chainMutex.RLock()
		if c.Jobs["job2"].State == proto.STATE_RUNNING && c.Jobs["job3"].State == proto.STATE_RUNNING {
			traverser.chainMutex.RUnlock()
			break
		}
		traverser.chainMutex.RUnlock()
	}

	traverser.Stop()

	// Wait for the traverser to finish.
	<-doneChan

	if c.State != proto.STATE_INCOMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State, proto.STATE_COMPLETE)
	}
	if c.Jobs["job2"].State != proto.STATE_FAIL {
		t.Errorf("job2 state = %d, expected %d", c.Jobs["job2"].State, proto.STATE_FAIL)
	}
	if c.Jobs["job3"].State != proto.STATE_FAIL {
		t.Errorf("job3 state = %d, expected %d", c.Jobs["job3"].State, proto.STATE_FAIL)
	}
	if c.Jobs["job4"].State != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.Jobs["job4"].State, proto.STATE_PENDING)
	}
}
