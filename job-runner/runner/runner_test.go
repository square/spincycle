// Copyright 2017, Square, Inc.

package runner_test

import (
	"testing"
	"time"

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
		JobsToReturn: map[string]*mock.Job{},
		MakeErr:      mock.ErrJob,
	}
	rmc := &mock.RMClient{}
	rf := runner.NewFactory(jf, rmc)

	pJob := proto.Job{
		Id:    "j1",
		Type:  "jtype",
		Bytes: []byte{},
	}

	jr, err := rf.Make(pJob, "abc")
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
	jr := runner.NewRunner(pJob, mJob, "abc", rmc)

	finalState := jr.Run(noJobData)
	if finalState != proto.STATE_FAIL {
		t.Errorf("final state = %d, expected %d", finalState, proto.STATE_FAIL)
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
	jr := runner.NewRunner(pJob, mJob, "abc", rmc)

	finalState := jr.Run(noJobData)
	if finalState != proto.STATE_COMPLETE {
		t.Errorf("final state = %d, expected %d", finalState, proto.STATE_COMPLETE)
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
	jr := runner.NewRunner(pJob, mJob, "abc", rmc)

	// Run the job and let it block.
	stateChan := make(chan byte)
	go func() {
		stateChan <- jr.Run(noJobData)
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
	jr := runner.NewRunner(pJob, mJob, "abc", rmc)

	status := jr.Status()
	if status != expectedStatus {
		t.Errorf("status = %s, expected %s", status, expectedStatus)
	}
}
