// Copyright 2018, Square, Inc.

package chain_test

import (
	"fmt"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

var (
	deleteThis = fmt.Sprintf("blah")
)

// Get a default factory to use for testing. Tests probably need to re-set
// Chain, RMClient, and some of the channels to customize the reapers.
func defaultFactory(reqId string) *chain.ChainReaperFactory {
	return &chain.ChainReaperFactory{
		Chain:        chain.NewChain(&proto.JobChain{}, make(map[string]uint), make(map[string]uint), make(map[string]uint)),
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     &mock.RMClient{},
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  make(chan proto.Job),
		RunJobChan:   make(chan proto.Job),
		RunnerRepo:   runner.NewRepo(),
	}
}

// runningChainReaper.Reap on a completed job
func TestRunningReapComplete(t *testing.T) {
	// Job Chain:
	//         6 (same seq as job 2)
	//        /
	//       2 - 5 (diff seq as job 2)
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 completes (Job 3 not complete)

	reqId := "test_running_reap_complete"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(6),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job4"},
		},
	}
	job5 := jc.Jobs["job5"]
	job5.SequenceId = "job5" // make job5 its own sequence
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan.
	runJobChan := make(chan proto.Job, 5)
	factory.RunJobChan = runJobChan
	reaper := factory.MakeRunning()

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
	reaper.(*chain.RunningChainReaper).Reap(job)

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

	// Job data should have been copied to jobs 4, 5, + 6
	if diff := deep.Equal(jc.Jobs["job4"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
	if diff := deep.Equal(jc.Jobs["job5"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
	if diff := deep.Equal(jc.Jobs["job6"].Data, job.Data); diff != nil {
		t.Error(diff)
	}

	// Job5's sequence try count should have been incremented from 0 to 1 (diff seq from job2)
	if c.SequenceTries("job5") != 1 {
		t.Errorf("got job5 sequence tries = %d, expected %d", c.SequenceTries("job5"), 1)
	}

	// Job6's sequence try count should stay 1 (same sequence as job2)
	if c.SequenceTries("job6") != 1 {
		t.Errorf("got job6 sequence tries = %d, expected %d", c.SequenceTries("job5"), 1)
	}
}

// runningChainReaper.Reap on a failed job (no sequence retry)
func TestRunningReapFail(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	//  \_ 4
	// Testing when job 2 fails

	reqId := "test_running_reap_fail"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job4"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan (though we don't expect any values
	// to be sent after a job fails).
	runJobChan := make(chan proto.Job, 5)
	factory.RunJobChan = runJobChan
	reaper := factory.MakeRunning()

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
	reaper.(*chain.RunningChainReaper).Reap(job)

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

// runningChainReaper.Reap on a failed job whose sequence can be retried
func TestRunningReapFailRetry(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	//  \_ 4
	// Testing when job 2 fails (+ sequence is retryable)

	reqId := "test_running_reap_fail_retry"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobsWithSequenceRetry(4, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job4"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	// Normally runJobChan is unbuffered. Here we use a buffer so Reap
	// doesn't block on sending to the chan.
	runJobChan := make(chan proto.Job, 5)
	factory.RunJobChan = runJobChan
	reaper := factory.MakeRunning()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)
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
	reaper.(*chain.RunningChainReaper).Reap(job)

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

// test runningChainReaper.Run on a new chain (faking the runJobs loop)
func TestRunningReaper(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3

	reqId := "test_running_reaper"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobsWithSequenceRetry(5, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	job1 := jc.Jobs["job1"]
	job1.Data = map[string]interface{}{
		"key1": "val1",
	}
	jc.Jobs["job1"] = job1
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
	}

	// Normally these chans are unbuffered, but give them buffers here to make
	// testing easier (don't need to worry about sends blocking).
	runJobChan := make(chan proto.Job, 10)
	doneJobChan := make(chan proto.Job, 10)

	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   runJobChan,
		RunnerRepo:   runner.NewRepo(),
	}
	reaper := factory.MakeRunning()

	go func() {
		reaper.Run()
		close(runJobChan)
	}()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_RUNNING)

	job1 = jc.Jobs["job1"]
	job1.State = proto.STATE_COMPLETE

	doneJobChan <- job1

	// fail first job received (to retry sequence)
	job := <-runJobChan
	job.State = proto.STATE_FAIL
	doneJobChan <- job

	// complete all jobs except fail job 5
	for job := range runJobChan {
		if job.Id == "job5" {
			job.State = proto.STATE_FAIL
			doneJobChan <- job
		}
		job.State = proto.STATE_COMPLETE
		doneJobChan <- job
	}

	// check state and job data of every job
	expectedStates := map[string]byte{
		"job1": proto.STATE_COMPLETE,
		"job2": proto.STATE_COMPLETE,
		"job3": proto.STATE_COMPLETE,
		"job4": proto.STATE_COMPLETE,
		"job5": proto.STATE_FAIL,
	}
	expectedJobData := map[string]interface{}{
		"key1": "val1",
	}
	for _, job := range jc.Jobs {
		if job.State != expectedStates[job.Id] {
			t.Errorf("got state %s for job %s, expected state %s", proto.StateName[job.State], job.Id, proto.StateName[expectedStates[job.Id]])
		}

		if diff := deep.Equal(job.Data, expectedJobData); diff != nil {
			t.Errorf("job data for job %s not as expected: %s", job.Id, diff)
		}
	}

	// check that finalize was sent with correct state
	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_FAIL])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test runningChainReaper.Run on a resumed chain (faking the runJobs loop)
func TestRunningReaperResume(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3

	reqId := "test_running_reaper_resume"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobsWithSequenceRetry(5, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	job := jc.Jobs["job2"]
	job.Data = map[string]interface{}{
		"key1": "val1",
	}
	jc.Jobs["job2"] = job
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
	}

	// Normally these chans are unbuffered, but give them buffers here to make
	// testing easier (don't need to worry about sends blocking).
	runJobChan := make(chan proto.Job, 10)
	doneJobChan := make(chan proto.Job, 10)

	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   runJobChan,
		RunnerRepo:   runner.NewRepo(),
	}
	reaper := factory.MakeRunning()

	// Pretend job chain was suspended - jobs 2 + 3 were stopped and are now being
	// run. Keep job3 stopped while sending job2 to reaper, to make sure it can
	// handle stopped jobs.
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_STOPPED)
	c.SetJobState("job3", proto.STATE_STOPPED)

	job2 := jc.Jobs["job2"]
	job2.State = proto.STATE_COMPLETE
	job3 := jc.Jobs["job3"]
	job3.State = proto.STATE_COMPLETE

	go func() {
		reaper.Run()
		close(runJobChan)
	}()

	c.SetJobState("job2", proto.STATE_RUNNING)
	doneJobChan <- job2

	c.SetJobState("job3", proto.STATE_RUNNING)
	doneJobChan <- job3

	// complete all jobs
	for job := range runJobChan {
		job.State = proto.STATE_COMPLETE
		doneJobChan <- job
	}

	// check state and job data of every job
	expectedJobData := map[string]interface{}{
		"key1": "val1",
	}
	for _, job := range jc.Jobs {
		if job.State != proto.STATE_COMPLETE {
			t.Errorf("got state %s for job %s, expected state %s", proto.StateName[job.State], job.Id, proto.StateName[proto.STATE_COMPLETE])
		}

		// check that job2's job data was copied to its child jobs
		if job.Id != "job1" && job.Id != "job3" {
			if diff := deep.Equal(job.Data, expectedJobData); diff != nil {
				t.Errorf("job data for job %s not as expected: %s", job.Id, diff)
			}
		}
	}

	// check that finalize was sent with correct state
	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_COMPLETE {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_COMPLETE])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test stoppedChainReaper.Reap
func TestStoppedReap(t *testing.T) {
	// Job Chain:
	// 1 - 2 - 3
	// Testing when Job 2 stops

	reqId := "test_stopped_reap"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

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
	reaper.(*chain.StoppedChainReaper).Reap(job)

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_STOPPED {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_STOPPED)
	}
}

// test stopepdChainReaper.Run
func TestStoppedReaper(t *testing.T) {
	// Job Chain:
	//          6
	//         /
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// jobs 3, 5, 6 running

	reqId := "test_stopped_reaper"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(6),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
	}

	doneJobChan := make(chan proto.Job)
	runnerRepo := runner.NewRepo()
	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   make(chan proto.Job),
		RunnerRepo:   runnerRepo,
	}
	reaper := factory.MakeStopped()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_RUNNING)
	c.SetJobState("job5", proto.STATE_RUNNING)
	c.SetJobState("job6", proto.STATE_RUNNING)
	runnerRepo.Set("job3", &mock.Runner{})
	runnerRepo.Set("job5", &mock.Runner{})
	runnerRepo.Set("job6", &mock.Runner{})

	job3 := jc.Jobs["job3"]
	job3.State = proto.STATE_STOPPED
	job5 := jc.Jobs["job5"]
	job5.State = proto.STATE_COMPLETE
	job6 := jc.Jobs["job6"]
	job6.State = proto.STATE_STOPPED

	doneChan := make(chan struct{})
	go func() {
		reaper.Run()
		close(doneChan)
	}()

	doneJobChan <- job3
	runnerRepo.Remove("job3")

	doneJobChan <- job5
	runnerRepo.Remove("job5")

	doneJobChan <- job6
	runnerRepo.Remove("job6")

	<-doneChan // wait for reaper to finish

	if c.JobState("job3") != proto.STATE_STOPPED {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job3")], "job3", proto.StateName[proto.STATE_STOPPED])
	}
	if c.JobState("job5") != proto.STATE_COMPLETE {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job5")], "job5", proto.StateName[proto.STATE_COMPLETE])
	}
	if c.JobState("job6") != proto.STATE_STOPPED {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job6")], "job6", proto.StateName[proto.STATE_STOPPED])
	}

	// check that finalize was sent with correct state
	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_FAIL])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test stoppedChainReaper.Run + .Stop
func TestStoppedReaperStop(t *testing.T) {
	// Job Chain:
	//          6
	//         /
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// jobs 3, 5, 6 running

	reqId := "test_stopped_reaper_stop"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(6),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sent := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sent = true
			receivedState = state
			return nil
		},
	}

	doneJobChan := make(chan proto.Job)
	runnerRepo := runner.NewRepo()
	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   make(chan proto.Job),
		RunnerRepo:   runnerRepo,
	}
	reaper := factory.MakeStopped()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_RUNNING)
	c.SetJobState("job5", proto.STATE_RUNNING)
	c.SetJobState("job6", proto.STATE_RUNNING)
	runnerRepo.Set("job3", &mock.Runner{})
	runnerRepo.Set("job5", &mock.Runner{})
	runnerRepo.Set("job6", &mock.Runner{})

	job3 := jc.Jobs["job3"]
	job3.State = proto.STATE_STOPPED
	job5 := jc.Jobs["job5"]
	job5.State = proto.STATE_COMPLETE

	go func() {
		reaper.Run()
	}()

	doneJobChan <- job3
	runnerRepo.Remove("job3")

	doneJobChan <- job5
	runnerRepo.Remove("job5")

	// call Stop instead of sending job6
	reaper.Stop()

	if c.JobState("job3") != proto.STATE_STOPPED {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job3")], "job3", proto.StateName[proto.STATE_STOPPED])
	}
	if c.JobState("job5") != proto.STATE_COMPLETE {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job5")], "job5", proto.StateName[proto.STATE_COMPLETE])
	}
	if c.JobState("job6") != proto.STATE_FAIL {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job6")], "job6", proto.StateName[proto.STATE_FAIL])
	}

	// check that finalize was sent with correct state
	if !sent {
		t.Errorf("final chain state not sent to RM client")
		return
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_FAIL])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test suspendedChainReaper.Reap on a complete job
func TestSuspendedReapComplete(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 completes

	reqId := "test_suspended_reap_complete"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

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
	reaper.(*chain.SuspendedChainReaper).Reap(job)

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_COMPLETE {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_COMPLETE)
	}

	// Job data should have been copied to jobs 4 + 5
	if diff := deep.Equal(jc.Jobs["job4"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
	if diff := deep.Equal(jc.Jobs["job5"].Data, job.Data); diff != nil {
		t.Error(diff)
	}
}

// test suspendedChainReaper.Reap on a failed job (retryable)
func TestSuspendedReapFail(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 fails

	reqId := "test_suspended_reap_fail"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobsWithSequenceRetry(5, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just completed
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_FAIL,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	reaper.(*chain.SuspendedChainReaper).Reap(job)

	// all job states should have been set back to PENDING
	for id := range jc.Jobs {
		gotState := c.JobState(id)
		if gotState != proto.STATE_PENDING {
			t.Errorf("%s state in chain = %d, expected state = %d", id, gotState, proto.STATE_PENDING)
		}
	}

	// sequence tries in chain should not have been incremented
	tryCount := c.SequenceTries(job.Id)
	if tryCount != 1 {
		t.Errorf("got sequence try count %d, expected %d", tryCount, 1)
	}
}

// test suspendedChainReaper.Reap on a stopped job (this is the normal case)
func TestSuspendedReapStopped(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// Testing when Job 2 fails

	reqId := "test_suspended_reap_stopped"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobsWithSequenceRetry(5, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_RUNNING)

	// Job 2 has just completed
	job := proto.Job{
		Id:    "job2",
		State: proto.STATE_STOPPED,
		Data: map[string]interface{}{
			"key1": "val1",
		},
	}
	reaper.(*chain.SuspendedChainReaper).Reap(job)

	// job2's state should have been set in the chain
	gotState := c.JobState("job2")
	if gotState != proto.STATE_STOPPED {
		t.Errorf("job2 state in chain = %d, expected state = %d", gotState, proto.STATE_STOPPED)
	}
}

// test suspendedChainReaper.Run
func TestSuspendedReaper(t *testing.T) {
	// Job Chain:
	//          6
	//         /
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3
	// jobs 3, 5, 6 running

	reqId := "test_suspended_reaper"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(6),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job4"},
		},
	}
	job := jc.Jobs["job6"]
	job.SequenceId = "job6"
	job.SequenceRetry = 1
	jc.Jobs["job6"] = job
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sentState := false
	sentSJC := false
	var receivedSJC proto.SuspendedJobChain
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sentState = true
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			sentSJC = true
			receivedSJC = sjc
			return nil
		},
	}

	doneJobChan := make(chan proto.Job)
	runnerRepo := runner.NewRepo()
	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   make(chan proto.Job),
		RunnerRepo:   runnerRepo,
	}
	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.IncrementSequenceTries("job6")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_RUNNING)
	c.SetJobState("job5", proto.STATE_RUNNING)
	c.SetJobState("job6", proto.STATE_RUNNING)
	runnerRepo.Set("job3", &mock.Runner{})
	runnerRepo.Set("job5", &mock.Runner{})
	runnerRepo.Set("job6", &mock.Runner{})

	job3 := jc.Jobs["job3"]
	job3.State = proto.STATE_STOPPED
	job5 := jc.Jobs["job5"]
	job5.State = proto.STATE_COMPLETE
	job6 := jc.Jobs["job6"]
	job6.State = proto.STATE_FAIL

	doneChan := make(chan struct{})
	go func() {
		reaper.Run()
		close(doneChan)
	}()

	doneJobChan <- job3
	runnerRepo.Remove("job3")

	doneJobChan <- job5
	runnerRepo.Remove("job5")

	doneJobChan <- job6
	runnerRepo.Remove("job6")

	<-doneChan // wait for reaper to finish

	if c.JobState("job3") != proto.STATE_STOPPED {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job3")], "job3", proto.StateName[proto.STATE_STOPPED])
	}
	if c.JobState("job5") != proto.STATE_COMPLETE {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job5")], "job5", proto.StateName[proto.STATE_COMPLETE])
	}
	if c.JobState("job6") != proto.STATE_PENDING {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job6")], "job6", proto.StateName[proto.STATE_PENDING])
	}

	if sentState {
		t.Errorf("chain state sent to RM client, expected only SJC to be sent")
	}

	if !sentSJC {
		t.Errorf("SJC not sent to RM client")
		return
	}

	expectedSJC := proto.SuspendedJobChain{
		RequestId:         reqId,
		JobChain:          jc,
		TotalJobTries:     map[string]uint{},
		LatestRunJobTries: map[string]uint{},
		SequenceTries: map[string]uint{
			"job1": 1,
			"job6": 1,
		},
	}
	if diff := deep.Equal(receivedSJC, expectedSJC); diff != nil {
		t.Errorf("received SJC != expected SJC: %s", diff)
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test suspendedChainReaper.Run on a chain that completes
func TestSuspendedReaperCompleted(t *testing.T) {
	// Job Chain:
	// -> 1 -> 2 -> 3
	// job 3 running

	reqId := "test_suspended_reaper_completed"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sentState := false
	sentSJC := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sentState = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			sentSJC = true
			return nil
		},
	}

	doneJobChan := make(chan proto.Job)
	runnerRepo := runner.NewRepo()
	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   make(chan proto.Job),
		RunnerRepo:   runnerRepo,
	}
	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_RUNNING)
	runnerRepo.Set("job3", &mock.Runner{})

	job3 := jc.Jobs["job3"]
	job3.State = proto.STATE_COMPLETE

	doneChan := make(chan struct{})
	go func() {
		reaper.Run()
		close(doneChan)
	}()

	doneJobChan <- job3
	runnerRepo.Remove("job3")

	<-doneChan // wait for reaper to finish

	if c.JobState("job3") != proto.STATE_COMPLETE {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job3")], "job3", proto.StateName[proto.STATE_COMPLETE])
	}

	if !sentState {
		t.Errorf("final chain state not sent to RM Client")
		return
	}

	if sentSJC {
		t.Errorf("SJC sent to RM Client, expected only final chain state to be sent")
	}

	if receivedState != proto.STATE_COMPLETE {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_COMPLETE])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test suspendedChainReaper.Run + .Stop
func TestSuspendedReaperStop(t *testing.T) {
	// Job Chain:
	// -> 1 -> 2 -> 3
	// job 3 running

	reqId := "test_suspended_reaper_stop"
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	sentState := false
	sentSJC := false
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sentState = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			sentSJC = true
			return nil
		},
	}

	doneJobChan := make(chan proto.Job)
	runnerRepo := runner.NewRepo()
	factory := &chain.ChainReaperFactory{
		Chain:        c,
		ChainRepo:    chain.NewMemoryRepo(),
		RMClient:     rmc,
		Logger:       log.WithFields(log.Fields{"requestId": reqId}),
		RMCTries:     5,
		RMCRetryWait: 50 * time.Millisecond,
		DoneJobChan:  doneJobChan,
		RunJobChan:   make(chan proto.Job),
		RunnerRepo:   runnerRepo,
	}
	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_RUNNING)
	runnerRepo.Set("job3", &mock.Runner{})

	job3 := jc.Jobs["job3"]
	job3.State = proto.STATE_COMPLETE

	go func() {
		reaper.Run()
	}()

	reaper.Stop()

	if c.JobState("job3") != proto.STATE_FAIL {
		t.Errorf("got state %s for job %s, expected state %s", proto.StateName[c.JobState("job3")], "job3", proto.StateName[proto.STATE_FAIL])
	}

	if !sentState {
		t.Errorf("final chain state not sent to RM Client")
		return
	}

	if sentSJC {
		t.Errorf("SJC sent to RM Client, expected only final chain state to be sent")
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_FAIL])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test suspendedChainReaper.Finalize
func TestSuspendedFinalize(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3

	reqId := "test_suspended_finalize"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	sentState := false
	sentSJC := false
	var receivedSJC proto.SuspendedJobChain
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sentState = true
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			sentSJC = true
			receivedSJC = sjc
			return nil
		},
	}
	factory.RMClient = rmc

	reaper := factory.MakeSuspended()

	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_RUNNING)
	c.SetJobState("job5", proto.STATE_STOPPED)

	reaper.(*chain.SuspendedChainReaper).Finalize()

	// job3's state should have been set to FAIL
	gotState := c.JobState("job3")
	if gotState != proto.STATE_FAIL {
		t.Errorf("job3 state in chain = %d, expected state = %d", gotState, proto.STATE_FAIL)
	}

	// SJC should have been sent, since job 5 can still be run
	if sentState {
		t.Errorf("chain state sent to RM client, expected only SJC to be sent")
	}

	if !sentSJC {
		t.Errorf("SJC not sent to RM client")
		return
	}

	expectedSJC := proto.SuspendedJobChain{
		RequestId:         reqId,
		JobChain:          jc,
		TotalJobTries:     map[string]uint{},
		LatestRunJobTries: map[string]uint{},
		SequenceTries:     map[string]uint{},
	}
	if diff := deep.Equal(receivedSJC, expectedSJC); diff != nil {
		t.Errorf("received SJC != expected SJC: %s", diff)
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}

// test suspendedChainReaper.Finalize on a chain that finished
func TestSuspendedFinalizeFinished(t *testing.T) {
	// Job Chain:
	//       2 - 5
	//     /  \
	// -> 1    4
	//     \  /
	//      3

	reqId := "test_suspended_finalize_finished"
	factory := defaultFactory(reqId)
	jc := &proto.JobChain{
		RequestId: reqId,
		Jobs:      testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	factory.Chain = c

	sentState := false
	sentSJC := false
	var receivedState byte
	var rsjc proto.SuspendedJobChain
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte) error {
			sentState = true
			receivedState = state
			return nil
		},
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			sentSJC = true
			rsjc = sjc
			return nil
		},
	}
	factory.RMClient = rmc

	reaper := factory.MakeSuspended()

	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_FAIL)
	c.SetJobState("job5", proto.STATE_COMPLETE)

	reaper.(*chain.SuspendedChainReaper).Finalize()

	// state should have been sent since chain is done
	if !sentState {
		t.Errorf("final chain state not sent to RM Client")
		return
	}

	if sentSJC {
		t.Errorf("SJC sent to RM Client, expected only final chain state to be sent")
	}

	if receivedState != proto.STATE_FAIL {
		t.Errorf("chain state %s sent to RM client, expected state %s", proto.StateName[receivedState], proto.StateName[proto.STATE_FAIL])
	}

	// check that chain was removed from repo
	_, err := factory.ChainRepo.Get(jc.RequestId)
	if err == nil {
		t.Errorf("chain not removed from chain repo")
	}
}
