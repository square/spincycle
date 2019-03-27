// Copyright 2017-2019, Square, Inc.

package status_test

import (
	"sort"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/job-runner/status"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

func TestRunning(t *testing.T) {
	now := time.Now().UnixNano()

	jc1 := &proto.JobChain{
		RequestId: "chain1",
		AdjacencyList: map[string][]string{
			"job1": []string{"job2", "job3"},
		},
		Jobs: map[string]proto.Job{
			"job1": proto.Job{
				Id:            "job1",
				Type:          "type1",
				State:         proto.STATE_PENDING,
				SequenceId:    "job1",
				SequenceRetry: 0,
			},
		},
	}
	c1 := chain.NewChain(jc1, make(map[string]uint), make(map[string]uint), make(map[string]uint))
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
				Id:            "job2",
				Type:          "type2",
				State:         proto.STATE_PENDING,
				SequenceId:    "job2",
				SequenceRetry: 0,
			},
		},
	}
	c2 := chain.NewChain(jc2, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c2.SetJobState("job2", proto.STATE_RUNNING) // sets runtime start ts

	repo := chain.NewMemoryRepo()
	if err := repo.Add(c1); err != nil {
		t.Fatalf("error in Add: %v", err)
	}
	if err := repo.Add(c2); err != nil {
		t.Fatalf("error in Add: %v", err)
	}

	rr := mock.RunnerRepo{
		GetFunc: func(jobId string) runner.Runner {
			switch jobId {
			case "job1":
				return &mock.Runner{StatusResp: "job1 status"}
			case "job2":
				return &mock.Runner{StatusResp: "job2 status"}
			default:
				return nil
			}
		},
	}

	m := status.NewManager(repo, rr)

	got, err := m.Running(proto.StatusFilter{})
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
			Type:      "type1",
			State:     proto.STATE_RUNNING,
			Try:       1,
			Status:    "job1 status",
		},
		{
			RequestId: "chain2",
			JobId:     "job2",
			Type:      "type2",
			State:     proto.STATE_RUNNING,
			Try:       1,
			Status:    "job2 status",
		},
	}

	if got[0].StartedAt < now {
		t.Errorf("job1 started %d < %d", got[0].StartedAt, now)
	}
	if got[1].StartedAt <= 0 {
		t.Errorf("job2 runtime %d, expected > 0", got[1].StartedAt)
	}

	// StartedAt is nondeterministic
	expect[0].StartedAt = got[0].StartedAt
	expect[1].StartedAt = got[1].StartedAt

	if diff := deep.Equal(got, expect); diff != nil {
		test.Dump(got)
		t.Error(diff)
	}

	// chain1 should before chain2
	if got[0].StartedAt >= got[1].StartedAt {
		t.Errorf("started chain1 %d >= chain2 %d", got[0].StartedAt, got[1].StartedAt)
	}
}
