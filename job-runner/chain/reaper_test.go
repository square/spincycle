// Copyright 2017-2018, Square, Inc.

package chain_test

import (
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

// Helper func to run reap on a job with a timeout. Returns true if
// reaper.Reap(job) returns within 100 ms; else false.
func reapWithTimeout(reaper chain.JobReaper, job proto.Job) bool {
	doneChan := make(chan struct{})
	go func() {
		reaper.Reap(job)
		close(doneChan)
	}()

	select {
	case <-doneChan:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

// runningChainReaper on a completed job
func TestRunningReapComplete(t *testing.T) {
	// Job Chain:
	//         6
	//        /
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 completes (Job 3 not complete)

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_reap_complete",
		Jobs:      testutil.InitJobs(6),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job4"},
		},
	}
	job5 := jc.Jobs["job5"]
	job5.SequenceId = "job5" // make job5 its own sequence
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan. We only expect 1 value but in
	// case of a bug, make the buffer larger than needed so Reap can always
	// return. We check later that only 1 value was sent to the chan.
	runJobChan := make(chan proto.Job, 5)
	reaper := factory.MakeRunning(runJobChan)

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just completed
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_COMPLETE,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job5 + job6 should be sent to runJobChan
	select {
	case gotJob := <-runJobChan:
		if gotJob.Id != "job5" {
			t.Errorf("got job %s from runJobChan, expected job %s", gotJob.Id, "job5")
		}
	default:
		t.Errorf("no job sent to runJobChan - expected to get job5")
	}
	select {
	case gotJob := <-runJobChan:
		if gotJob.Id != "job6" {
			t.Errorf("got job %s from runJobChan, expected job %s", gotJob.Id, "job6")
		}
	default:
		t.Errorf("no job sent to runJobChan - expected to get job6")
	}

	// no other jobs should be sent to runJobChan
	select {
	case <-runJobChan:
		t.Errorf("more than one job sent to runJobChan - expected only job5")
	default:
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_COMPLETE {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_COMPLETE)
	}

	// Job data should have been copied to jobs 4 + 5
	if diff := deep.Equal(jc.Jobs["job5"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
	if diff := deep.Equal(jc.Jobs["job4"].Data, job.Data); diff != nil {
		t.Error(diff)
	}

	// Job5's sequence try count should have been incremented from 0 to 1
	if c.SequenceTries("job5") != 1 {
		t.Errorf("got job5 sequence tries = %d, expected %d", c.SequenceTries("job5"), 1)
	}

	// Job6's sequence try count should stay 1 (same sequence as job2)
	if c.SequenceTries("job6") != 1 {
		t.Errorf("got job6 sequence tries = %d, expected %d", c.SequenceTries("job5"), 1)
	}
}

// runningChainReaper on a failed job (no sequence retry)
func TestRunningReapFail(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	//  \_ 4
	// Testing when job 2 fails

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_reap_fail",
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job4"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan (though we don't expect any values
	// to be sent after a job fails).
	runJobChan := make(chan proto.Job, 5)
	reaper := factory.MakeRunning(runJobChan)

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just failed.
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_FAIL,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// no jobs should be sent to runJobChan
	select {
	case gotJob := <-runJobChan:
		t.Errorf("got job %s from runJobChan, expected no job", gotJob.Id)
	default:
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_FAIL {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_FAIL)
	}

	// Job data should not have been copied to job3
	if len(jc.Jobs["job3"].Data) != 0 {
		t.Errorf("job3 has job data - no data should have been copied from job2")
	}
}

// runningChainReaper on a failed job whose sequence can be retried
func TestRunningReapFailRetry(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	//  \_ 4
	// Testing when job 2 fails

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_reap_fail_retry",
		Jobs:      testutil.InitJobsWithSequenceRetry(4, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job4"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan. We only expect 1 value to
	// be sent, but make the buffer larger just in case.
	runJobChan := make(chan proto.Job, 5)
	reaper := factory.MakeRunning(runJobChan)

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just failed.
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_FAIL,
		Data: map[string]interface{}{
			"key1": "val1",
		},
		SequenceId: "job1",
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job1 should be (re)sent to runJobChan
	select {
	case gotJob := <-runJobChan:
		if gotJob.Id != "job1" {
			t.Errorf("got job %s from runJobChan, expected job %s", gotJob.Id, "job1")
		}
	default:
		t.Errorf("no job sent to runJobChan - expected to get job1")
	}

	// no other jobs should be sent to runJobChan
	select {
	case <-runJobChan:
		t.Errorf("more than one job sent to runJobChan - expected only job1")
	default:
	}

	// all job states should have been set back to PENDING
	for id := range jc.Jobs {
		gotState := c.JobState(id)
		if gotState != proto.STATE_PENDING {
			t.Errorf("%s state in chain = %d, expected state = %d", id, gotState, proto.STATE_PENDING)
		}
	}

	// sequence tries in chain should have been incremented
	tryCount := c.SequenceTries(job.Id)
	if tryCount != 2 {
		t.Errorf("got sequence try count %d, expected %d", tryCount, 1)
	}
}

// runningChainReaper on a completed job - chain is failed
func TestRunningReapChainFail(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 5 completes (Job 3 previously failed)

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_reap_chain_fail",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan (though we don't expect any values
	// to be sent after a job fails).
	runJobChan := make(chan proto.Job, 5)
	reaper := factory.MakeRunning(runJobChan)

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_FAIL)
	c.SetJobState("job5", proto.STATE_RUNNING)

	// Job 5 has just completed
	job := proto.Job{
		Id:    "job5",
		State: proto.STATE_COMPLETE,
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// runJobChan should have been closed (+ no jobs sent)
	select {
	case gotJob, ok := <-runJobChan:
		if ok {
			t.Errorf("got job %s from runJobChan, expected no jobs and runJobChan to be closed", gotJob.Id)
		}
	default:
		t.Errorf("runJobChan open - expected runJobChan to be closed")
	}

	// job5's state should have been set in the chain
	gotState := c.JobState("job5")
	if gotState != proto.STATE_COMPLETE {
		t.Errorf("job5 state in chain = %d, expected state = %d", gotState, proto.STATE_COMPLETE)
	}

	// chain's final state should have been set (failed b/c of job3)
	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected state = %d", c.State(), proto.STATE_FAIL)
	}
}

// runningChainReaper on a completed job - chain is complete
func TestRunningReapChainComplete(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 5 completes (all other jobs complete)

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_reap_chain_complete",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan (though we don't expect any values
	// to be sent after a job fails).
	runJobChan := make(chan proto.Job, 5)
	reaper := factory.MakeRunning(runJobChan)

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_RUNNING)

	// Job 5 has just completed
	job := proto.Job{
		Id:    "job5",
		State: proto.STATE_COMPLETE,
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// runJobChan should have been closed (+ no jobs sent)
	select {
	case gotJob, ok := <-runJobChan:
		if ok {
			t.Errorf("got job %s from runJobChan, expected no jobs and runJobChan to be closed", gotJob.Id)
		}
	default:
		t.Errorf("runJobChan open - expected runJobChan to be closed")
	}

	// job5's state should have been set in the chain
	gotState := c.JobState("job5")
	if gotState != proto.STATE_COMPLETE {
		t.Errorf("job5 state in chain = %d, expected state = %d", gotState, proto.STATE_COMPLETE)
	}

	// chain's final state should have been set (complete)
	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected state = %d", c.State(), proto.STATE_COMPLETE)
	}
}

// runningChainReaper where another job (not the one being reaped) has state
// = stopped. This may occur soon after resuming a suspended chain, and it
// shouldn't register as the chain being done.
func TestRunningReapStoppedJob(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 completes (Job 3 is stopped)

	reqId := "test_running_reap_stopped_job"
	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	sjc := &proto.SuspendedJobChain{
		RequestId:       reqId,
		JobChain:        jc,
		TotalJobTries:   make(map[string]uint),
		LastRunJobTries: make(map[string]uint),
		SequenceTries:   make(map[string]uint),
		NumJobsRun:      1,
	}
	c := chain.ResumeChain(sjc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan. We only expect 1 value but in
	// case of a bug, make the buffer larger than needed so Reap can always
	// return. We check later that only 1 value was sent to the chan.
	runJobChan := make(chan proto.Job, 5)
	reaper := factory.MakeRunning(runJobChan)

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_STOPPED)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just completed
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_COMPLETE,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job5 should be sent to runJobChan
	select {
	case gotJob, ok := <-runJobChan:
		if !ok {
			t.Errorf("runJobChan closed unexpectedly")
			return
		}
		if gotJob.Id != "job5" {
			t.Errorf("got job %s from runJobChan, expected job %s", gotJob.Id, "job5")
		}
	default:
		t.Errorf("no job sent to runJobChan - expected to get job5")
	}

	// no other jobs should be sent to runJobChan
	select {
	case <-runJobChan:
		t.Errorf("more than one job sent to runJobChan - expected only job5")
	default:
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_COMPLETE {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_COMPLETE)
	}

	// Job data should have been copied to jobs 4 + 5
	if diff := deep.Equal(jc.Jobs["job5"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
	if diff := deep.Equal(jc.Jobs["job4"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
}

// runningChainReaper finalizing a chain
func TestRunningFinalize(t *testing.T) {
	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_finalize",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactory(c, cr, rmc)
	runJobChan := make(chan proto.Job)
	reaper := factory.MakeRunning(runJobChan)

	// all jobs and chain completed
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)
	c.SetState(proto.STATE_COMPLETE)

	reaper.Finalize()

	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_COMPLETE {
		t.Errorf("chain state %d sent to RM client, expected state %d", receivedState, proto.STATE_COMPLETE)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// runningChainReaper error when finalizing chain
func TestRunningFinalizeError(t *testing.T) {
	count := 0
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			count++
			return mock.ErrRMClient
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_finalize_error",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactoryWithRetries(c, cr, rmc, 5, time.Millisecond)
	runJobChan := make(chan proto.Job)
	reaper := factory.MakeRunning(runJobChan)

	// all jobs and chain completed
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)
	c.SetState(proto.STATE_COMPLETE)

	reaper.Finalize()

	if count != 5 {
		t.Errorf("tried to finish request %d times, expected %d tries", count, 5)
	}

	_, err := cr.Get(jc.RequestId)
	if err != nil {
		t.Errorf("chain removed from chain repo, but should not have been removed")
	}
}

// runningChainReaper error when finalizing chain but success after retry
func TestRunningFinalizeRetry(t *testing.T) {
	count := 0
	expectCount := 4
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			count++
			if count < expectCount {
				return mock.ErrRMClient
			}
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_running_finalize_retry",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactoryWithRetries(c, cr, rmc, 5, time.Millisecond)
	runJobChan := make(chan proto.Job)
	reaper := factory.MakeRunning(runJobChan)

	// all jobs and chain completed
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)
	c.SetState(proto.STATE_COMPLETE)

	reaper.Finalize()

	if count != expectCount {
		t.Errorf("tried to finish request %d times, expected %d tries", count, expectCount)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// suspendedChainReaper on a completed job
func TestSuspendedReapComplete(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 completes

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_reap_complete",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just completed
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_COMPLETE,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_COMPLETE {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_COMPLETE)
	}

	// Job data should have been copied to jobs 4 + 5
	if diff := deep.Equal(jc.Jobs["job5"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
	if diff := deep.Equal(jc.Jobs["job4"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
}

// suspendedChainReaper on a failed job (no sequence retry)
func TestSuspendedReapFail(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	// Testing when Job 2 fails

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_reap_fail",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just failed.
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_FAIL,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_FAIL {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_FAIL)
	}

	// Job data should not have been copied to job3
	if len(jc.Jobs["job3"].Data) != 0 {
		t.Errorf("job3 has job data - no data should have been copied from job2")
	}
}

// suspendedChainReaper on a failed job (with sequence retry)
func TestSuspendedReapFailRetry(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	// Testing when Job 2 fails

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_reap_fail_retry",
		Jobs:      testutil.InitJobsWithSequenceRetry(3, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just failed.
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_FAIL,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// all job states should have been set back to PENDING
	for id := range jc.Jobs {
		gotState := c.JobState(id)
		if gotState != proto.STATE_PENDING {
			t.Errorf("%s state in chain = %d, expected state = %d", id, gotState, proto.STATE_PENDING)
		}
	}

	// sequence retry count in chain should NOT have been incremented
	tryCount := c.SequenceTries(job.Id)
	if tryCount != 1 {
		t.Errorf("got sequence try count %d, expected %d", tryCount, 1)
	}
}

// suspendedChainReaper on a stopped job
func TestSuspendedReapStopped(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	// Testing when Job 2 stops

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_reap_stopped",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just been stopped
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_STOPPED,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_STOPPED {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_STOPPED)
	}

	// Job data should not have been copied to job3
	if len(jc.Jobs["job3"].Data) != 0 {
		t.Errorf("job3 has job data - no data should have been copied from job2")
	}
}

// suspendedChainReaper finalizing a complete chain
func TestSuspendedFinalizeComplete(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// All jobs are complete

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			t.Error("unexpected SJC sent to RM - expected final chain state to be sent")
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_finalize_complete",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactory(c, cr, rmc)
	reaper := factory.MakeSuspended()

	// all jobs complete
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)

	reaper.Finalize()

	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_COMPLETE {
		t.Errorf("chain state %d sent to RM client, expected state %d", receivedState, proto.STATE_COMPLETE)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// suspendedChainReaper finalizing a failed chain
func TestSuspendedFinalizeFail(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Job 5 fails, job 3 fails

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			t.Error("unexpected SJC sent to RM - expected final chain state to be sent")
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_finalize_fail",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactory(c, cr, rmc)
	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_FAIL)
	c.SetJobState("job5", proto.STATE_FAIL)

	reaper.Finalize()

	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %d sent to RM client, expected state %d", receivedState, proto.STATE_FAIL)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// suspendedChainReaper finalizing a suspended chain
func TestSuspendedFinalizeStopped(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Jobs 3 stopped, job 5 fails

	sent := false
	var receivedSJC proto.SuspendedJobChain
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			t.Errorf("unexpected final chain state (%d) sent to RM - expected SJC to be sent", state)
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			sent = true
			receivedSJC = sjc
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_finalize_stopped",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactory(c, cr, rmc)
	reaper := factory.MakeSuspended()

	// all jobs and chain completed
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_STOPPED)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_FAIL)

	reaper.Finalize()

	if !sent {
		t.Errorf("sjc not sent to RM client")
		return
	}

	if diff := deep.Equal(receivedSJC, c.ToSuspended()); diff != nil {
		t.Error(diff)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// suspendedChainReaper error when finalizing a done chain
func TestSuspendedFinalizeFinishError(t *testing.T) {
	count := 0
	expectCount := 4
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			count++
			if count < expectCount {
				return mock.ErrRMClient
			}
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_finalize_finish_error",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactoryWithRetries(c, cr, rmc, 5, time.Millisecond)
	reaper := factory.MakeSuspended()

	// all jobs completed
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)

	reaper.Finalize()

	if count != expectCount {
		t.Errorf("tried to finish request %d times, expected %d tries", count, expectCount)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// suspendedChainReaper error when finalizing a suspended chain
func TestSuspendedFinalizeSuspendError(t *testing.T) {
	count := 0
	expectCount := 4
	rmc := &mock.RMClient{
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			count++
			if count < expectCount {
				return mock.ErrRMClient
			}
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_suspended_finalize_suspend_error",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactoryWithRetries(c, cr, rmc, 5, time.Millisecond)
	reaper := factory.MakeSuspended()

	// stopped
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_STOPPED)

	reaper.Finalize()

	if count != expectCount {
		t.Errorf("tried to finish request %d times, expected %d tries", count, expectCount)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// stoppedChainReaper reaping a job
func TestStoppedReap(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	// Testing when Job 2 stops

	rmc := &mock.RMClient{}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_stopped_reap",
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc)
	factory := chain.NewReaperFactory(c, cr, rmc)

	reaper := factory.MakeStopped()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just returned
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_STOPPED,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	if !reapWithTimeout(reaper, job) {
		t.Error("reaper did not finish running within 1 second")
		return
	}

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_STOPPED {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_STOPPED)
	}
}

// stoppedChainReaper finalizing a complete chain
func TestStoppedFinalizeComplete(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// All jobs are complete

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			t.Error("unexpected SJC sent to RM - expected final chain state to be sent")
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_stopped_finalize_complete",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactory(c, cr, rmc)
	reaper := factory.MakeStopped()

	// all jobs complete
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)

	reaper.Finalize()

	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_COMPLETE {
		t.Errorf("chain state %d sent to RM client, expected state %d", receivedState, proto.STATE_COMPLETE)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// stoppedChainReaper finalizing a failed chain
func TestStoppedFinalizeFail(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Job 5 stopped

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			t.Error("unexpected SJC sent to RM - expected final chain state to be sent")
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_stopped_finalize_fail",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactory(c, cr, rmc)
	reaper := factory.MakeStopped()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_STOPPED)

	reaper.Finalize()

	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %d sent to RM client, expected state %d", receivedState, proto.STATE_FAIL)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// stopepdChainReaper error when finalizing chain
func TestStoppedFinalizeError(t *testing.T) {
	count := 0
	expectCount := 4
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			count++
			if count < expectCount {
				return mock.ErrRMClient
			}
			return nil
		},
	}
	cr := chain.NewMemoryRepo()
	jc := &proto.JobChain{
		RequestId: "test_stopped_finalize_error",
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc)
	cr.Add(c)
	factory := chain.NewReaperFactoryWithRetries(c, cr, rmc, 5, time.Millisecond)
	reaper := factory.MakeStopped()

	// all jobs completed
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
	c.SetJobState("job5", proto.STATE_COMPLETE)

	reaper.Finalize()

	if count != expectCount {
		t.Errorf("tried to finish request %d times, expected %d tries", count, expectCount)
	}

	_, err := cr.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}
