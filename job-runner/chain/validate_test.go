// Copyright 2017-2019, Square, Inc.

package chain

import (
	"reflect"
	"testing"

	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
)

func TestValidateJobChain(t *testing.T) {
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"}, // first job
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	if err := Validate(jc, true); err != nil {
		t.Error(err)
	}

	// Invalid adjacency list
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job7": {},
		},
	}
	if err := Validate(jc, true); err == nil {
		t.Error("no error, expected error on invalid chain")
	}

	// Invalid first job
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job3"}, // first job
			"job2": {"job3"}, // first job
		},
	}
	if err := Validate(jc, true); err == nil {
		t.Error("no error, expected error on invalid chain")
	}
}

func TestValidateFirstJobMultiple(t *testing.T) {
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job3"}, // first job
			"job2": {"job3"}, // first job
			"job3": {"job4"},
		},
	}
	if hasFirstJob(jc) {
		t.Errorf("hasFirstJob = true, expected false")
	}
}

func TestValidateFirstJobOne(t *testing.T) {
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"}, // first job
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	if !hasFirstJob(jc) {
		t.Errorf("hasFirstJob = false, expected true")
	}
}

func TestValidateLastJobMultiple(t *testing.T) {
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
		},
	}
	if hasLastJob(jc) {
		t.Errorf("hasLastJob = true, expected false")
	}
}

func TestValidateLastJobOne(t *testing.T) {
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	if !hasLastJob(jc) {
		t.Errorf("hasLastJob = false, expected true")
	}
}

func TestValidateIndegreeCounts(t *testing.T) {
	jc := proto.JobChain{
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
	counts := indegreeCounts(jc)
	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Errorf("counts = %v, want %v", counts, expectedCounts)
	}
}

func TestValidateOutdegreeCounts(t *testing.T) {
	jc := proto.JobChain{
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
	expectedCounts := map[string]int{
		"job1": 2,
		"job2": 3,
		"job3": 2,
		"job4": 2,
		"job5": 1,
		"job6": 1,
		"job7": 0,
	}
	counts := outdegreeCounts(jc)
	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Errorf("counts = %v, want %v", counts, expectedCounts)
	}
}

func TestValidateIsAcyclic(t *testing.T) {
	// No cycles in the chain.
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	if !isAcyclic(jc) {
		t.Error("isAcyclic = false, expected true")
	}

	// Cycle from end (job4) to start (job1). Also means no first job.
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
			"job4": {"job1"}, // creates cycle: 4 -> 1 -> 2, 3 -> 4
		},
	}
	if isAcyclic(jc) {
		t.Error("isAcyclic = true, expected false")
	}

	// Cycle in the middle of the chain.
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job5"},
			"job4": {"job5"},
			"job5": {"job2", "job6"}, // creates cycle: 5 -> 2 - 4 -> 5
		},
	}
	if isAcyclic(jc) {
		t.Error("isAcyclic = true, expected false")
	}

	// No cycle, but multiple first jobs and last jobs.
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job3"},
			"job2": {"job3"},
			"job3": {"job4", "job5"},
		},
	}
	if !isAcyclic(jc) {
		t.Error("isAcyclic = false, expected true")
	}
}

func TestValidateAdjacencyList(t *testing.T) {
	// Valid, no cycles
	jc := proto.JobChain{
		Jobs: testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	if !adjacencyListIsValid(jc) {
		t.Error("adjacencyListIsValid = false, expected true")
	}

	// Invalid: job2 and job3 have no edges, and job3 doesn't exist
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
		},
	}
	if adjacencyListIsValid(jc) {
		t.Error("adjacencyListIsValid = true, expected false")
	}

	// Invalid: job7 doesn't exist
	jc = proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job7": {},
		},
	}
	if adjacencyListIsValid(jc) {
		t.Error("adjacencyListIsValid = true, expected false")
	}
}
