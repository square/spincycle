// Copyright 2017, Square, Inc.

package chain

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/square/spincycle/proto"
)

func initJobs(count int, emptyBytes bool) map[string]*SerializedJob {
	jobs := make(map[string]*SerializedJob)
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("job%d", i)
		if emptyBytes {
			jobs[name] = &SerializedJob{
				Name:  name,
				State: proto.STATE_PENDING,
			}
		} else {
			bytes := make([]byte, 10)
			rand.Read(bytes)
			jobs[name] = &SerializedJob{
				Name:  name,
				Bytes: bytes,
				State: proto.STATE_PENDING,
			}
		}
	}
	return jobs
}

func TestRootJobMultipleRoots(t *testing.T) {
	c := &Chain{
		Jobs: initJobs(4, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job3"},
			"job2": []string{"job3"},
			"job3": []string{"job4"},
		},
	}

	rootJob, err := c.RootJob()
	if rootJob != nil {
		t.Errorf("rootJob = %s, expected nil", rootJob)
	}
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestRootJobCyclic(t *testing.T) {
	c := &Chain{
		Jobs: initJobs(4, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
			"job4": []string{"job1"},
		},
	}

	rootJob, err := c.RootJob()
	if rootJob != nil {
		t.Errorf("rootJob = %s, expected nil", rootJob)
	}
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestRootJobOneRoot(t *testing.T) {
	jobs := initJobs(4, false)
	c := &Chain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}

	expectedRootJob := jobs["job1"]
	rootJob, err := c.RootJob()

	if !reflect.DeepEqual(rootJob, expectedRootJob) {
		t.Errorf("root job = %v, expected %v", rootJob, expectedRootJob)
	}
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

func TestNextJobs(t *testing.T) {
	jobs := initJobs(4, false)
	c := &Chain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}

	expectedNextJobs := SerializedJobs{jobs["job2"], jobs["job3"]}
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
	jobs := initJobs(4, false)
	c := &Chain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}

	expectedPreviousJobs := SerializedJobs{jobs["job2"], jobs["job3"]}
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
	jobs := initJobs(5, false)
	jobs["job2"].State = proto.STATE_COMPLETE
	jobs["job3"].State = proto.STATE_PENDING
	c := &Chain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}

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

func TestComplete(t *testing.T) {
	jobs := initJobs(4, false)
	jobs["job1"].State = proto.STATE_COMPLETE
	jobs["job2"].State = proto.STATE_COMPLETE
	jobs["job3"].State = proto.STATE_COMPLETE
	jobs["job4"].State = proto.STATE_COMPLETE
	c := &Chain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
		},
	}

	expectedComplete := true
	complete := c.Complete()

	if complete != expectedComplete {
		t.Errorf("complete = %t, want %t", complete, expectedComplete)
	}

	c.Jobs["job3"].State = proto.STATE_RUNNING
	expectedComplete = false
	complete = c.Complete()

	if complete != expectedComplete {
		t.Errorf("complete = %t, want %t", complete, expectedComplete)
	}
}

func TestIncomplete(t *testing.T) {
	jobs := initJobs(4, false)
	jobs["job1"].State = proto.STATE_COMPLETE
	jobs["job2"].State = proto.STATE_RUNNING
	jobs["job3"].State = proto.STATE_FAIL
	jobs["job4"].State = proto.STATE_PENDING
	c := &Chain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4"},
			"job3": []string{"job4"},
		},
	}

	expectedIncomplete := false
	incomplete := c.Incomplete()

	if incomplete != expectedIncomplete {
		t.Errorf("incomplete = %t, want %t", incomplete, expectedIncomplete)
	}

	c.Jobs["job2"].State = proto.STATE_COMPLETE
	expectedIncomplete = true
	incomplete = c.Incomplete()

	if incomplete != expectedIncomplete {
		t.Errorf("incomplete = %t, want %t", incomplete, expectedIncomplete)
	}

	c.Jobs["job3"].State = proto.STATE_COMPLETE
	expectedIncomplete = false
	incomplete = c.Incomplete()

	if incomplete != expectedIncomplete {
		t.Errorf("incomplete = %t, want %t", incomplete, expectedIncomplete)
	}
}

func TestIndegreeCounts(t *testing.T) {
	c := &Chain{
		Jobs: initJobs(9, true),
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
			"job2": []string{"job4", "job5"},
			"job3": []string{"job5", "job6"},
			"job4": []string{"job6", "job7"},
			"job5": []string{"job6"},
			"job6": []string{"job8"},
			"job7": []string{"job8"},
		},
	}

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
