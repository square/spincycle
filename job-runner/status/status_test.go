// Copyright 2017-2019, Square, Inc.

package status_test

import (
	"sort"
	"testing"
	"time"

	"github.com/orcaman/concurrent-map"

	"github.com/go-test/deep"
	"github.com/square/spincycle/v2/job-runner/status"
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/test"
	"github.com/square/spincycle/v2/test/mock"
)

func TestRunning(t *testing.T) {
	// The status manager uses traversers to get running job status. All traversers
	// are added to the global traverser repo (in api). So we just create some mock
	// traverers that return mock job status info. Request-traverser-chain is 1:1:1.
	trRepo := cmap.New()

	t1 := time.Now().Add(-2 * time.Second).UnixNano()
	t2 := time.Now().Add(-1 * time.Second).UnixNano()

	tr1 := &mock.Traverser{
		JobStatus: []proto.JobStatus{
			{
				RequestId: "req1",
				JobId:     "job1",
				Type:      "job1Type",
				Name:      "job1Name",
				StartedAt: t1,
				State:     proto.STATE_RUNNING,
				Status:    "job1 status",
				Try:       1,
			},
		},
	}
	trRepo.Set("req1", tr1)

	tr2 := &mock.Traverser{
		JobStatus: []proto.JobStatus{
			{
				RequestId: "req2",
				JobId:     "job2",
				Type:      "job2Type",
				Name:      "job2Name",
				StartedAt: t2,
				State:     proto.STATE_RUNNING,
				Status:    "job2 status",
				Try:       2,
			},
		},
	}
	trRepo.Set("req2", tr2)

	m := status.NewManager(trRepo)

	got, err := m.Running(proto.StatusFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d proto.JobStatus, expected 2", len(got))
	}
	sort.Sort(proto.JobStatusByStartTime(got))
	expect := []proto.JobStatus{
		{
			RequestId: "req1",
			JobId:     "job1",
			Type:      "job1Type",
			Name:      "job1Name",
			State:     proto.STATE_RUNNING,
			Try:       1,
			Status:    "job1 status",
			StartedAt: t1,
		},
		{
			RequestId: "req2",
			JobId:     "job2",
			Type:      "job2Type",
			Name:      "job2Name",
			State:     proto.STATE_RUNNING,
			Try:       2,
			Status:    "job2 status",
			StartedAt: t2,
		},
	}
	if diff := deep.Equal(got, expect); diff != nil {
		test.Dump(got)
		t.Error(diff)
	}

	// Filter by request ID
	got, err = m.Running(proto.StatusFilter{RequestId: "req2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d proto.JobStatus, expected 1", len(got))
	}
	expect = []proto.JobStatus{
		{
			RequestId: "req2",
			JobId:     "job2",
			Type:      "job2Type",
			Name:      "job2Name",
			State:     proto.STATE_RUNNING,
			Try:       2,
			Status:    "job2 status",
			StartedAt: t2,
		},
	}
	if diff := deep.Equal(got, expect); diff != nil {
		test.Dump(got)
		t.Error(diff)
	}
}
