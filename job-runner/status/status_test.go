// Copyright 2017, Square, Inc.

package status_test

import (
	"sort"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/status"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test"
)

func TestRunning(t *testing.T) {

	jc1 := &proto.JobChain{
		RequestId: "chain1",
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
		Jobs: map[string]proto.Job{
			"job1": proto.Job{
				Id:   "job1",
				Type: "type1",
			},
		},
	}
	c1 := chain.NewChain(jc1)
	c1.SetJobState("job1", proto.STATE_RUNNING) // sets runtime start ts

	// Any short delay to make Runtime differ. chain1/job1 will have a runtime
	// roughly equal to this delay, and chain2/job2 will have a runtime closer
	// to zero.
	time.Sleep(250 * time.Millisecond)

	jc2 := &proto.JobChain{
		RequestId: "chain2",
		AdjacencyList: map[string][]string{
			"job2": []string{"job2"},
		},
		Jobs: map[string]proto.Job{
			"job2": proto.Job{
				Id:   "job2",
				Type: "type2",
			},
		},
	}
	c2 := chain.NewChain(jc2)
	c2.SetJobState("job2", proto.STATE_RUNNING) // sets runtime start ts

	repo := chain.NewMemoryRepo()
	if err := repo.Add(c1); err != nil {
		t.Fatalf("error in Add: %v", err)
	}
	if err := repo.Add(c2); err != nil {
		t.Fatalf("error in Add: %v", err)
	}

	m := status.NewManager(repo)

	got, err := m.Running()
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d proto.JobStatus, expected 2", len(got))
	}

	sort.Sort(proto.JobStatuses(got))

	expect := []proto.JobStatus{
		{
			RequestId: "chain1", // longer runtime because of delay ^
			JobId:     "job1",
			State:     proto.STATE_RUNNING,
			N:         1,
		},
		{
			RequestId: "chain2",
			JobId:     "job2",
			State:     proto.STATE_RUNNING,
			N:         1,
		},
	}

	if got[0].Runtime <= 0 {
		t.Errorf("job1 runtime %f, expected > 0", got[0].Runtime)
	}
	if got[1].Runtime <= 0 {
		t.Errorf("job2 runtime %f, expected > 0", got[1].Runtime)
	}

	// Runtime is nondeterministic
	expect[0].Runtime = got[0].Runtime
	expect[1].Runtime = got[1].Runtime

	if diff := deep.Equal(got, expect); diff != nil {
		test.Dump(got)
		t.Error(diff)
	}

	// chain1 runtime should be > chain2
	if got[0].Runtime <= got[1].Runtime {
		t.Errorf("runtime chain1 %f <= chain2 %f", got[0].Runtime, got[1].Runtime)
	}
}
