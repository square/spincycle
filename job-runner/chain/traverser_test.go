// Copyright 2017-2018, Square, Inc.

package chain_test

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
	"github.com/square/spincycle/test/mock"
)

// Default timeout for traverser (stopping jobs + sending to doneJobChan)
const timeout = 100 * time.Millisecond

// All jobs in the chain complete successfully.
func TestRunComplete(t *testing.T) {
	// Job Chain:
	//      2
	//     / \
	// -> 1   4
	//     \ /
	//      3

	requestId := "test_run_complete"
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	traverser.Run()

	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}

	_, err := chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Not all jobs in the chain complete successfully.
func TestRunNotComplete(t *testing.T) {
	// Job Chain:
	//      2
	//     / \
	// -> 1   4
	//     \ /
	//      3

	requestId := "test_run_not_complete"
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_FAIL}},
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	traverser.Run()

	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_FAIL)
	}
	if c.JobState("job4") != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobState("job4"), proto.STATE_PENDING)
	}

	_, err := chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Resume a suspended job chain.
func TestResume(t *testing.T) {
	// Job chain:
	//       2 - 5
	//      / \   \
	// -> 1    4 - 6
	//     \  /
	//      3
	// Job 3 and 5 should be runnable
	requestId := "test_resume"
	chainRepo := chain.NewMemoryRepo()
	var gotTotalTries uint
	runnersToReturn := map[string]*mock.Runner{
		"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE, Tries: 1}},
		"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE, Tries: 1}},
		"job5": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE, Tries: 1}},
		"job6": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE, Tries: 1}},
	}
	rf := &mock.RunnerFactory{
		MakeFunc: func(job proto.Job, requestId string, prevTryNo uint, totalTries uint) (runner.Runner, error) {
			if job.Id == "job3" {
				gotTotalTries = totalTries
			}
			return runnersToReturn[job.Id], nil
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})
	tf := chain.NewTraverserFactory(chainRepo, rf, rmc, shutdownChan)

	jobs := map[string]proto.Job{
		"job1": proto.Job{
			Id:            "job1",
			State:         proto.STATE_COMPLETE,
			SequenceId:    "job1",
			SequenceRetry: 1,
		},
		"job2": proto.Job{
			Id:         "job2",
			State:      proto.STATE_COMPLETE,
			SequenceId: "job1",
		},
		"job3": proto.Job{ // can be run
			Id:         "job3",
			State:      proto.STATE_STOPPED,
			SequenceId: "job1",
			Retry:      2, // try 3 times
		},
		"job4": proto.Job{
			Id:         "job4",
			State:      proto.STATE_PENDING,
			SequenceId: "job1",
		},
		"job5": proto.Job{ // can be run
			Id:         "job5",
			State:      proto.STATE_PENDING,
			SequenceId: "job1",
		},
		"job6": proto.Job{
			Id:         "job6",
			State:      proto.STATE_PENDING,
			SequenceId: "job1",
		},
	}
	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
			"job4": {"job6"},
			"job5": {"job6"},
		},
		FinishedJobs: 2,
	}
	sjc := proto.SuspendedJobChain{
		RequestId: requestId,
		JobChain:  jc,
		TotalJobTries: map[string]uint{
			"job1": 2, // sequence retried once (job4 failed)
			"job2": 2,
			"job3": 3, // failed on seq try 2, job try 1 (job total job try = 3)
			"job4": 1,
		},
		LatestRunJobTries: map[string]uint{
			"job1": 1,
			"job2": 1,
			"job3": 1, // job3 should have 2 tries left
			"job4": 1,
		},
		SequenceTries: map[string]uint{
			"job1": 2,
		},
	}
	traverser, err := tf.MakeFromSJC(&sjc)
	if err != nil {
		t.Errorf("got error when making traverser: %s", err)
		return
	}
	c, err := chainRepo.Get(requestId) // get the chain before running
	if err != nil {
		t.Errorf("got error retrieving chain from chain repo: %s", err)
	}

	traverser.Run()

	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}

	_, err = chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}

	expectedTotalTries := map[string]uint{
		"job1": 2,
		"job2": 2,
		"job3": 4, // tried 1 time
		"job4": 2, // tried 1 time
		"job5": 1, // tried 1 time
		"job6": 1, // tried 1 time
	}
	if diff := deep.Equal(c.ToSuspended().TotalJobTries, expectedTotalTries); diff != nil {
		t.Error(diff)
	}

	// Tries to be skipped when running a stopped job should be the number of times
	// job was tried before being stopped - 1 (don't count the try it was stopped
	// on)
	if gotTotalTries != 3 {
		t.Errorf("got job3 tries before Stopped = %d, expected 3", gotTotalTries)
	}
}

// Unknown job state should not cause the traverser to panic when running.
func TestJobUnknownState(t *testing.T) {
	requestId := "test_job_unknown_state"
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	for _, j := range jc.Jobs {
		j.State = proto.STATE_UNKNOWN
	}
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	traverser.Run()

	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}

	_, err := chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Error creating a runner.
func TestRunJobsRunnerError(t *testing.T) {
	requestId := "test_run_jobs_runner_error"
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
		// This is what causes the error, even though the job returns STATE_COMPLETE
		MakeErr: mock.ErrRunner,
	}
	var recvdjl proto.JobLog // record the jl that gets sent to the RM
	rmc := &mock.RMClient{
		CreateJLFunc: func(reqId string, jl proto.JobLog) error {
			if reqId == requestId {
				recvdjl = jl
				return nil
			}
			return mock.ErrRMClient
		},
	}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(1),
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	traverser.Run()

	if jc.State != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", jc.State, proto.STATE_FAIL)
	}

	// Make sure the JL sent to the RM matches what we expect.
	if recvdjl.RequestId != requestId {
		t.Errorf("jl request id = %s, expected %s", recvdjl.RequestId, requestId)
	}
	if recvdjl.JobId != "job1" {
		t.Errorf("jl job id = %s, expected %s", recvdjl.JobId, "job1")
	}
	if recvdjl.State != proto.STATE_FAIL {
		t.Errorf("jl state = %d, expected %d", recvdjl.State, proto.STATE_FAIL)
	}
	if recvdjl.Error == "" {
		t.Errorf("jl error is empty, expected something")
	}
	if recvdjl.StartedAt != 0 {
		t.Errorf("jobLog.StartedAt = %d, expected 0", recvdjl.StartedAt)
	}
	if recvdjl.FinishedAt != 0 {
		t.Errorf("jobLog.Finished = %d, expected 0", recvdjl.FinishedAt)
	}

	_, err := chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Stop the traverser and all running jobs.
func TestStop(t *testing.T) {
	requestId := "test_stop"
	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(2)
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_STOPPED}, RunBlock: make(chan struct{}), RunWg: &runWg},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_STOPPED}, RunBlock: make(chan struct{}), RunWg: &runWg},
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will
	// run until Stop is called (which will close their RunBlock channels).
	runWg.Wait()

	err := traverser.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Wait for the traverser to finish.
	select {
	case <-doneChan:
	case <-time.After(time.Second):
		t.Error("traverser did not finish running within 1 second")
		return
	}

	if c.State() != proto.STATE_STOPPED {
		t.Errorf("chain state = %s, expected STOPPED", proto.StateName[c.State()])
	}
	if c.JobState("job2") != proto.STATE_STOPPED {
		t.Errorf("job2 state = %d, expected %d", c.JobState("job2"), proto.STATE_STOPPED)
	}
	if c.JobState("job3") != proto.STATE_STOPPED {
		t.Errorf("job3 state = %d, expected %d", c.JobState("job3"), proto.STATE_STOPPED)
	}
	if c.JobState("job4") != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobState("job4"), proto.STATE_PENDING)
	}

	_, err = chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Stop a chain but runner.Run() never returns for one of the jobs
func TestStopRunnerHangs(t *testing.T) {
	requestId := "test_stop_runner_hangs"
	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(2)
	doneChan := make(chan struct{}) // indicates traverser.Run returned
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_STOPPED}, RunBlock: make(chan struct{}), RunWg: &runWg},
			"job3": &mock.Runner{
				RunFunc: func(jobData map[string]interface{}) byte {
					runWg.Done()
					// Wait longer than the Stop timeout
					<-time.After(5 * time.Second)
					select {
					case <-doneChan:
					default:
						t.Errorf("traverser didn't finish before runner returned")
					}
					return proto.STATE_COMPLETE
				},
			},
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	// Start the traverser.
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will
	// run until Stop is called (which will close their RunBlock channels).
	runWg.Wait()

	err := traverser.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Wait for the traverser to finish.
	select {
	case <-doneChan:
	case <-time.After(1 * time.Second):
		t.Error("traverser did not finish stopping within 2 seconds")
		return
	}

	if c.State() != proto.STATE_FAIL {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_FAIL)
	}
	if c.JobState("job2") != proto.STATE_STOPPED {
		t.Errorf("job2 state = %d, expected %d", c.JobState("job2"), proto.STATE_STOPPED)
	}
	if c.JobState("job3") != proto.STATE_FAIL { // failed because didn't return
		t.Errorf("job3 state = %d, expected %d", c.JobState("job3"), proto.STATE_STOPPED)
	}
	if c.JobState("job4") != proto.STATE_PENDING {
		t.Errorf("job4 state = %d, expected %d", c.JobState("job4"), proto.STATE_PENDING)
	}

	_, err = chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}
}

// Stop the traverser after it's already done running
func TestStopDoneRunning(t *testing.T) {
	requestId := "test_stop_done_running"
	chainRepo := chain.NewMemoryRepo()
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
		},
	}

	sentCount := 0
	var receivedState byte
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte, finishedAt time.Time) error {
			sentCount++
			receivedState = state
			return nil
		},
	}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	traverser.Run()

	err := traverser.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	if sentCount != 1 {
		t.Errorf("final chain state sent to RM %d times, expected %d time", sentCount, 1)
	}
	if receivedState != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}
}

func TestStopAfterSuspend(t *testing.T) {
	requestId := "test_stop_done_running"
	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(1)
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_STOPPED}, RunBlock: make(chan struct{}), RunWg: &runWg},
		},
	}
	sent := false
	rmc := &mock.RMClient{
		FinishRequestFunc: func(reqId string, state byte, finishedAt time.Time) error {
			sent = true
			return nil
		},
	}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	go func() {
		traverser.Run()
	}()

	runWg.Wait()
	close(shutdownChan)

	time.Sleep(10 * time.Millisecond) // give time for traverser.shutdown() to start
	err := traverser.Stop()
	if err == nil {
		t.Errorf("got no error, but expected %s", chain.ErrShuttingDown)
	} else if err != chain.ErrShuttingDown {
		t.Errorf("got err = %s, expected %s", err, chain.ErrShuttingDown)
	}

	if sent {
		t.Errorf("final chain state sent to RM, expected only SJC to be sent")
	}
}

// Suspend a running chain
func TestSuspend(t *testing.T) {
	// Job Chain:
	//       2 -> 4
	//     /
	// -> 1
	//     \
	//       3 -> 5
	// Chain is suspended while 2 and 3 are running:
	// 2 completes successfully, 3 stops

	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(2)
	requestId := "test_suspend"
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{
				RunReturn: runner.Return{
					FinalState: proto.STATE_COMPLETE,
					Tries:      1,
				},
				AddedJobData: map[string]interface{}{"data1": "v1"},
			}, // job 1 completes before chain is suspended
			"job2": &mock.Runner{
				RunReturn: runner.Return{
					FinalState: proto.STATE_COMPLETE,
					Tries:      1,
				},
				AddedJobData: map[string]interface{}{"data2": "v2"},
				RunBlock:     make(chan struct{}),
				RunWg:        &runWg,
				IgnoreStop:   true, // completes successfully after Stop is called
			}, // job 2 completes after chain is suspended
			"job3": &mock.Runner{
				RunReturn: runner.Return{
					FinalState: proto.STATE_STOPPED,
					Tries:      2,
				},
				RunBlock: make(chan struct{}),
				RunWg:    &runWg,
			}, // job 3 is stopped while running
		},
	}
	var receivedSJC proto.SuspendedJobChain
	receivedSJCChan := make(chan struct{})
	rmc := &mock.RMClient{
		SuspendRequestFunc: func(reqId string, sjc proto.SuspendedJobChain) error {
			receivedSJC = sjc
			close(receivedSJCChan)
			return nil
		},
	}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobsWithSequenceRetry(5, 1),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job5"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will
	// run until Stop is called (which will close their RunBlock channels).
	runWg.Wait()

	// Suspend the traverser
	close(shutdownChan)

	// SJC should be sent and traverser.Run should return quickly.
	waitChan := time.After(1 * time.Second)
	select {
	case <-waitChan:
		t.Errorf("SJC not sent within 1 second of shutdown signal")
	case <-receivedSJCChan:
		select {
		case <-waitChan:
			t.Errorf("traverser.Run didn't return within 1 second of shutdown signal")
		case <-doneChan:
		}
	}

	if c.State() != proto.STATE_SUSPENDED { // chain should be suspended
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_SUSPENDED)
	}
	if c.JobState("job1") != proto.STATE_COMPLETE { // job1 finished before suspend
		t.Errorf("job1 state = %d, expected %d", c.JobState("job1"), proto.STATE_COMPLETE)
	}
	if c.JobState("job2") != proto.STATE_COMPLETE { // job2 finished after suspend
		t.Errorf("job2 state = %d, expected %d", c.JobState("job2"), proto.STATE_COMPLETE)
	}
	if c.JobState("job3") != proto.STATE_STOPPED { // job3 stopped by suspend
		t.Errorf("job3 state = %d, expected %d", c.JobState("job3"), proto.STATE_STOPPED)
	}
	if c.JobState("job4") != proto.STATE_PENDING { // job4 never started
		t.Errorf("job4 state = %d, expected %d", c.JobState("job4"), proto.STATE_PENDING)
	}
	if c.JobState("job5") != proto.STATE_PENDING { // job5 never started
		t.Errorf("job5 state = %d, expected %d", c.JobState("job5"), proto.STATE_PENDING)
	}

	_, err := chainRepo.Get(requestId)
	if err != chain.ErrNotFound {
		t.Error("chain still in repo, expected it to be removed")
	}

	// Make sure the SJC sent to the RM matches what we expect.
	if receivedSJC.RequestId != requestId {
		t.Errorf("sjc request id = %s, expected %s", receivedSJC.RequestId, requestId)
	}

	expectedTotalJobTries := map[string]uint{
		"job1": 1,
		"job2": 1,
		"job3": 2,
	}
	if diff := deep.Equal(receivedSJC.TotalJobTries, expectedTotalJobTries); diff != nil {
		t.Error(diff)
	}

	expectedLatestRunJobTries := map[string]uint{
		"job1": 1,
		"job2": 1,
		"job3": 2,
	}
	if diff := deep.Equal(receivedSJC.LatestRunJobTries, expectedLatestRunJobTries); diff != nil {
		t.Error(diff)
	}

	expectedSequenceTries := map[string]uint{
		"job1": 1,
	}
	if diff := deep.Equal(receivedSJC.SequenceTries, expectedSequenceTries); diff != nil {
		t.Error(diff)
	}

	// job3 should still have its job data
	expectedJob3Data := map[string]interface{}{"data1": "v1"}
	if diff := deep.Equal(jc.Jobs["job3"].Data, expectedJob3Data); diff != nil {
		t.Error(diff)
	}
	// job4 should have job data copied from job 2
	expectedJob4Data := map[string]interface{}{"data1": "v1", "data2": "v2"}
	if diff := deep.Equal(jc.Jobs["job4"].Data, expectedJob4Data); diff != nil {
		t.Error(diff)
	}
	// job5 should not have any job data (job3 didn't complete)
	expectedJob5Data := map[string]interface{}{}
	if diff := deep.Equal(jc.Jobs["job5"].Data, expectedJob5Data); diff != nil {
		t.Error(diff)
	}
}

// Get the status from all running jobs.
func TestStatus(t *testing.T) {
	now := time.Now().UnixNano()

	requestId := "test_status"
	chainRepo := chain.NewMemoryRepo()
	var runWg sync.WaitGroup
	runWg.Add(2)
	rf := &mock.RunnerFactory{
		RunnersToReturn: map[string]*mock.Runner{
			"job1": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job1 running"},
			"job2": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job2 running", RunBlock: make(chan struct{}), RunWg: &runWg},
			"job3": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job3 running", RunBlock: make(chan struct{}), RunWg: &runWg},
			"job4": &mock.Runner{RunReturn: runner.Return{FinalState: proto.STATE_COMPLETE}, StatusResp: "job4 running"},
		},
	}
	rmc := &mock.RMClient{}
	shutdownChan := make(chan struct{})

	jc := &proto.JobChain{
		RequestId: requestId,
		Jobs:      testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := chain.NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	traverser := chain.NewTraverser(chain.TraverserConfig{c, chainRepo, rf, rmc, shutdownChan, timeout, timeout})

	// Start the traverser.
	doneChan := make(chan struct{})
	go func() {
		traverser.Run()
		close(doneChan)
	}()

	// Wait until jobs 2 and 3 are running (until they call wg.Done()). They will
	// run until Status is called (which will close their RunBlock channels).
	runWg.Wait()

	expectedStatus := proto.JobChainStatus{
		RequestId: requestId,
		JobStatuses: proto.JobStatuses{
			proto.JobStatus{
				RequestId: requestId,
				JobId:     "job2",
				State:     proto.STATE_RUNNING,
				Status:    "job2 running",
				Args:      map[string]interface{}{},
			},
			proto.JobStatus{
				RequestId: requestId,
				JobId:     "job3",
				State:     proto.STATE_RUNNING,
				Status:    "job3 running",
				Args:      map[string]interface{}{},
			},
		},
	}
	status, err := traverser.Status()
	sort.Sort(status.JobStatuses)

	for i, j := range status.JobStatuses {
		if j.StartedAt < now {
			t.Errorf("StartedAt <= 0: %+v", j)
		}
		status.JobStatuses[i].StartedAt = 0
	}
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if diff := deep.Equal(status, expectedStatus); diff != nil {
		t.Error(diff)
	}

	// Wait for the traverser to finish.
	select {
	case <-doneChan:
	case <-time.After(time.Second):
		t.Error("traverser did not finish running within 1 second")
		return
	}

	if c.State() != proto.STATE_COMPLETE {
		t.Errorf("chain state = %d, expected %d", c.State(), proto.STATE_COMPLETE)
	}
}
