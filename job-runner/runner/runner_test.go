// Copyright 2017, Square, Inc.

package runner_test

import (
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/test/mock"
)

var noJobData = map[string]interface{}{}

// Return errors when creating a new Runner.
func TestFactory(t *testing.T) {
	// Making the job factory return an error.
	jf := &mock.JobFactory{
		MockJobs: map[string]*mock.Job{},
		MakeErr:  mock.ErrJob,
	}
	rmc := &mock.RMClient{}
	rf := runner.NewFactory(jf, rmc)

	pJob := proto.Job{
		Id:    "j1",
		Type:  "jtype",
		Bytes: []byte{},
	}

	jr, err := rf.Make(pJob, "abc", 0, 0, 0)
	if err != mock.ErrJob {
		t.Errorf("err = nil, expected %s", mock.ErrJob)
	}
	if jr != nil {
		t.Error("got a JobRunner, expected nil")
	}
}

func TestRunFail(t *testing.T) {
	attemptNumber := 0
	// Create a mock job that will fail despite 2 retry attempts.
	mJob := &mock.Job{
		RunFunc: func(jobData map[string]interface{}) (job.Return, error) {
			defer func() { attemptNumber += 1 }()
			switch attemptNumber {
			case 0, 1, 2:
				return job.Return{State: proto.STATE_FAIL}, nil
			}
			return job.Return{State: proto.STATE_COMPLETE}, nil // shouldn't get here.
		},
	}
	pJob := proto.Job{
		Id:    "failJob",
		Type:  "jtype",
		Bytes: []byte{},
		Retry: 2,
	}
	// Create a mock rmClient that will keep track of how many JLs get sent through it.
	jlsSent := 0
	rmc := &mock.RMClient{
		CreateJLFunc: func(reqId string, jl proto.JobLog) error {
			jlsSent += 1
			return nil
		},
	}
	jr := runner.NewRunner(pJob, mJob, "abc", 1, 1, rmc)

	ret := jr.Run(noJobData)
	if ret.FinalState != proto.STATE_FAIL {
		t.Errorf("final state = %d, expected %d", ret.FinalState, proto.STATE_FAIL)
	}
	if ret.Tries != 3 {
		t.Errorf("tries= %d, expected %d", ret.Tries, 3)
	}

	if jlsSent != 3 {
		t.Errorf("runner sent %d JLs, expected %d", jlsSent, 3)
	}
}

func TestRunSuccess(t *testing.T) {
	attemptNumber := 0
	// Create a mock job that will succeed on the third of four retries.
	mJob := &mock.Job{
		RunFunc: func(jobData map[string]interface{}) (job.Return, error) {
			defer func() { attemptNumber += 1 }()
			switch attemptNumber {
			case 3:
				return job.Return{State: proto.STATE_COMPLETE}, nil
			}
			return job.Return{State: proto.STATE_FAIL}, nil
		},
	}
	pJob := proto.Job{
		Id:    "successJob",
		Type:  "jtype",
		Bytes: []byte{},
		Retry: 4,
	}
	// Create a mock rmClient that will keep track of how many JLs get sent through it.
	jlsSent := 0
	rmc := &mock.RMClient{
		CreateJLFunc: func(reqId string, jl proto.JobLog) error {
			jlsSent += 1
			return nil
		},
	}
	jr := runner.NewRunner(pJob, mJob, "abc", 1, 1, rmc)

	ret := jr.Run(noJobData)
	if ret.FinalState != proto.STATE_COMPLETE {
		t.Errorf("final state = %d, expected %d", ret.FinalState, proto.STATE_COMPLETE)
	}
	if ret.Tries != 4 {
		t.Errorf("tries= %d, expected %d", ret.Tries, 4)
	}

	if jlsSent != 4 {
		t.Errorf("runner sent %d JLs, expected %d", jlsSent, 4)
	}
}

// Test to make sure the runner will return when Stop is called.
func TestRunStop(t *testing.T) {
	mJob := &mock.Job{
		RunFunc: func(jobData map[string]interface{}) (job.Return, error) {
			return job.Return{State: proto.STATE_FAIL}, nil
		},
	}
	pJob := proto.Job{
		Id:        "successJob",
		Type:      "jtype",
		Bytes:     []byte{},
		Retry:     1,
		RetryWait: 30000, // important...the runner will sleep for 30 seconds after the job fails the first time
	}
	rmc := &mock.RMClient{}
	jr := runner.NewRunner(pJob, mJob, "abc", 1, 1, rmc)

	// Run the job and let it block.
	stateChan := make(chan byte)
	go func() {
		ret := jr.Run(noJobData)
		stateChan <- ret.FinalState
	}()

	// Sleep for a second to allow the runner to get to the state where it's sleeping for the
	// duration of the retry delay.
	time.Sleep(1 * time.Second)
	err := jr.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	finalState := <-stateChan
	if finalState != proto.STATE_FAIL {
		t.Errorf("final state = %d, expected %d", finalState, proto.STATE_FAIL)
	}

	// Make sure calling stop on an already stopped job doesn't panic.
	err = jr.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

func TestRunStatus(t *testing.T) {
	expectedStatus := "in progress"
	mJob := &mock.Job{
		StatusResp: expectedStatus,
	}
	pJob := proto.Job{
		Id:    "j1",
		Type:  "jtype",
		Bytes: []byte{},
	}
	rmc := &mock.RMClient{}
	jr := runner.NewRunner(pJob, mJob, "abc", 1, 1, rmc)

	status, try := jr.Status()
	if status != expectedStatus {
		t.Errorf("status = %s, expected %s", status, expectedStatus)
	}
	if try != 1 {
		t.Errorf("try = %d, expected %d", try, 1)
	}
}

func TestRunPanic(t *testing.T) {
	attemptNum := 0
	// Create a mock job that will panic.
	mJob := &mock.Job{
		RunFunc: func(jobData map[string]interface{}) (job.Return, error) {
			attemptNum += 1
			panic("forced job.Run panic")
			return job.Return{State: proto.STATE_COMPLETE}, nil // shouldn't get here.
		},
	}
	pJob := proto.Job{
		Id:    "panicJob",
		Type:  "jtype",
		Name:  "jobName",
		Bytes: []byte{},
		Retry: 1,
	}
	// Create a mock rmClient that will keep track of how many JLs get sent through it.
	jlsSent := 0
	var sentJLs []proto.JobLog
	rmc := &mock.RMClient{
		CreateJLFunc: func(reqId string, jl proto.JobLog) error {
			jlsSent += 1

			sentJLs = append(sentJLs, jl)
			return nil
		},
	}
	jr := runner.NewRunner(pJob, mJob, "abc", 0, 1, rmc)

	ret := jr.Run(noJobData)
	if ret.FinalState != proto.STATE_FAIL {
		t.Errorf("final state = %d, expected %d", ret.FinalState, proto.STATE_FAIL)
	}
	if ret.Tries != 2 {
		t.Errorf("tries= %d, expected %d", ret.Tries, 2)
	}

	expectedJLs := []proto.JobLog{
		proto.JobLog{
			RequestId:   "abc",
			JobId:       "panicJob",
			Name:        "jobName",
			Type:        "jtype",
			Try:         1,
			SequenceTry: 1,
			StartedAt:   sentJLs[0].StartedAt,
			FinishedAt:  sentJLs[0].FinishedAt,
			State:       proto.STATE_FAIL,
			Exit:        1,
			Error:       "panic from job.Run: forced job.Run panic",
		},
		proto.JobLog{
			RequestId:   "abc",
			JobId:       "panicJob",
			Name:        "jobName",
			Type:        "jtype",
			Try:         2,
			SequenceTry: 1,
			StartedAt:   sentJLs[1].StartedAt,
			FinishedAt:  sentJLs[1].FinishedAt,
			State:       proto.STATE_FAIL,
			Exit:        1,
			Error:       "panic from job.Run: forced job.Run panic",
		},
	}
	if jlsSent != 2 {
		t.Errorf("runner sent %d JLs, expected %d", jlsSent, 2)
	}
	if diff := deep.Equal(expectedJLs, sentJLs); diff != nil {
		t.Error(diff)
	}
	if sentJLs[0].StartedAt == 0 {
		t.Errorf("expected real value for job log StartedAt, got placeholder 0")
	}
}
