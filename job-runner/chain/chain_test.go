// Copyright 2017, Square, Inc.

package chain

import (
	"reflect"
	"sort"
	"testing"

	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
)

func TestFirstJobMultiple(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job3"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)

	_, err := c.FirstJob()
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestFirstJobOne(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)

	expectedFirstJob := c.JobChain.Jobs["job1"]
	firstJob, err := c.FirstJob()

	if !reflect.DeepEqual(firstJob, expectedFirstJob) {
		t.Errorf("firstJob = %v, expected %v", firstJob, expectedFirstJob)
	}
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

func TestLastJobMultiple(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
		},
	}
	c := NewChain(jc)

	_, err := c.LastJob()
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestLastJobOne(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)

	expectedLastJob := c.JobChain.Jobs["job4"]
	lastJob, err := c.LastJob()

	if !reflect.DeepEqual(lastJob, expectedLastJob) {
		t.Errorf("lastJob = %v, expected %v", lastJob, expectedLastJob)
	}
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

func TestNextJobs(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)

	expectedNextJobs := proto.Jobs{c.JobChain.Jobs["job2"], c.JobChain.Jobs["job3"]}
	sort.Sort(expectedNextJobs)
	nextJobs := c.NextJobs("job1")
	sort.Sort(nextJobs)

	if !reflect.DeepEqual(nextJobs, expectedNextJobs) {
		t.Errorf("nextJobs = %v, want %v", nextJobs, expectedNextJobs)
	}

	nextJobs = c.NextJobs("job4")

	if len(nextJobs) != 0 {
		t.Errorf("nextJobs count = %d, want 0", len(nextJobs))
	}
}

func TestPreviousJobs(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)

	expectedPreviousJobs := proto.Jobs{c.JobChain.Jobs["job2"], c.JobChain.Jobs["job3"]}
	sort.Sort(expectedPreviousJobs)
	previousJobs := c.PreviousJobs("job4")
	sort.Sort(previousJobs)

	if !reflect.DeepEqual(previousJobs, expectedPreviousJobs) {
		t.Errorf("previousJobs = %v, want %v", previousJobs, expectedPreviousJobs)
	}

	previousJobs = c.PreviousJobs("job1")

	if len(previousJobs) != 0 {
		t.Errorf("previousJobs count = %d, want 0", len(previousJobs))
	}
}

func TestJobIsReady(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_PENDING)

	expectedReady := false
	ready := c.JobIsReady("job4")

	if ready != expectedReady {
		t.Errorf("ready = %t, want %t", ready, expectedReady)
	}

	expectedReady = true
	ready = c.JobIsReady("job5")

	if ready != expectedReady {
		t.Errorf("ready = %t, want %t", ready, expectedReady)
	}
}

// When the chain is not done or complete.
func TestIsDoneJobRunning(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc)
	c.SetJobState("job1", proto.STATE_RUNNING)

	expectedDone := false
	expectedComplete := false
	done, complete := c.IsDone()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// When the chain is done but not complete.
func TestIsDoneNotComplete(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc)
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_FAIL)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_PENDING)

	expectedDone := true
	expectedComplete := false
	done, complete := c.IsDone()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}

	// Make sure we can handle unknown states
	c.SetJobState("job4", proto.STATE_UNKNOWN)

	done, complete = c.IsDone()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// When the chain is done and complete.
func TestIsDoneComplete(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc)
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)

	expectedDone := true
	expectedComplete := true
	done, complete := c.IsDone()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

func TestSetJobState(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(1),
	}
	c := NewChain(jc)

	c.SetJobState("job1", proto.STATE_COMPLETE)
	if c.JobChain.Jobs["job1"].State != proto.STATE_COMPLETE {
		t.Errorf("State = %d, want %d", c.JobChain.Jobs["job1"].State, proto.STATE_COMPLETE)
	}
}

func TestSetState(t *testing.T) {
	jc := &proto.JobChain{}
	c := NewChain(jc)

	c.SetState(proto.STATE_RUNNING)
	if c.JobChain.State != proto.STATE_RUNNING {
		t.Errorf("State = %d, want %d", c.JobChain.State, proto.STATE_RUNNING)
	}
}

func TestIndegreeCounts(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(9),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job5", "job6"},
			"job4": {"job6", "job7"},
			"job5": {"job6"},
			"job6": {"job8"},
			"job7": {"job8"},
		},
	}
	c := NewChain(jc)

	expectedCounts := map[string]int{
		"job1": 0,
		"job2": 1,
		"job3": 1,
		"job4": 1,
		"job5": 2,
		"job6": 3,
		"job7": 1,
		"job8": 2,
		"job9": 0,
	}
	counts := c.indegreeCounts()

	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Errorf("counts = %v, want %v", counts, expectedCounts)
	}
}

func TestOutdegreeCounts(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(7),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job5", "job6"},
			"job4": {"job5", "job6"},
			"job5": {"job6"},
			"job6": {"job7"},
		},
	}
	c := NewChain(jc)

	expectedCounts := map[string]int{
		"job1": 2,
		"job2": 3,
		"job3": 2,
		"job4": 2,
		"job5": 1,
		"job6": 1,
		"job7": 0,
	}
	counts := c.outdegreeCounts()

	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Errorf("counts = %v, want %v", counts, expectedCounts)
	}
}

func TestIsAcyclic(t *testing.T) {
	// No cycle in the chain.
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc)

	expectedIsAcyclic := true
	isAcyclic := c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}

	// Cycle from end to beginning of the chain (i.e., there is no first job).
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
			"job4": {"job1"},
		},
	}
	c = NewChain(jc)

	expectedIsAcyclic = false
	isAcyclic = c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}

	// Cycle in the middle of the chain.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job5"},
			"job4": {"job5"},
			"job5": {"job2", "job6"},
		},
	}
	c = NewChain(jc)

	expectedIsAcyclic = false
	isAcyclic = c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}

	// No cycle, but multiple first jobs and last jobs.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job3"},
			"job2": {"job3"},
			"job3": {"job4", "job5"},
		},
	}
	c = NewChain(jc)

	expectedIsAcyclic = true
	isAcyclic = c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}
}

func TestValidateAdjacencyList(t *testing.T) {
	// Invalid 1.
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
		},
	}
	c := NewChain(jc)

	expectedValid := false
	valid := c.adjacencyListIsValid()

	if valid != expectedValid {
		t.Errorf("valid = %t, expected %t", valid, expectedValid)
	}

	// Invalid 2.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job7": {},
		},
	}
	c = NewChain(jc)

	expectedValid = false
	valid = c.adjacencyListIsValid()

	if valid != expectedValid {
		t.Errorf("valid = %t, expected %t", valid, expectedValid)
	}

	// Valid.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c = NewChain(jc)

	expectedValid = true
	valid = c.adjacencyListIsValid()

	if valid != expectedValid {
		t.Errorf("valid = %t, expected %t", valid, expectedValid)
	}
}
