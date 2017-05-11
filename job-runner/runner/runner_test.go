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
	// Making the job returns an error.
	job := &mock.Job{}
	jf := &mock.JobFactory{
		JobToReturn: job,
		MakeErr:     mock.ErrJob,
	}
	rf := runner.NewFactory(jf)

	jr, err := rf.Make("jtype", "jname", []byte{}, "abc")
	if err != mock.ErrJob {
		t.Errorf("err = nil, expected %s", mock.ErrJob)
	}
	if jr != nil {
		t.Error("got a JobRunner, expected nil")
	}
}

func TestRunFail(t *testing.T) {
	job := &mock.Job{
		RunReturn: job.Return{State: proto.STATE_FAIL},
	}
	jr := runner.NewJobRunner(job, "abc")

	completed := jr.Run(noJobData)
	if completed != false {
		t.Errorf("completed = %t, expected false", completed)
	}
}

func TestRunSuccess(t *testing.T) {
	job := &mock.Job{
		RunReturn:    job.Return{State: proto.STATE_COMPLETE},
		AddedJobData: map[string]interface{}{"some": "thing"},
	}
	jr := runner.NewJobRunner(job, "abc")

	jobData := make(map[string]interface{})

	completed := jr.Run(jobData)
	if completed != true {
		t.Errorf("completed = %t, expected true", completed)
	}

	val, ok := jobData["some"]
	if !ok || val.(string) != "thing" {
		t.Errorf("jobData is not what we expected")
	}
}

func TestRunStop(t *testing.T) {
	runBlock := make(chan struct{})
	defer close(runBlock)
	job := &mock.Job{
		RunBlock: runBlock,
	}
	jr := runner.NewJobRunner(job, "abc")

	// Run the job and let it block
	completedChan := make(chan bool)
	go func() {
		completedChan <- jr.Run(noJobData)
	}()

	// Sleep just a moment to let Run ^ run, then stop it
	time.Sleep(200 * time.Millisecond)
	err := jr.Stop()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	completed := <-completedChan
	if completed != false {
		t.Errorf("completed = %t, expected false", completed)
	}
}

func TestRunStatus(t *testing.T) {
	expectedStatus := "in progress"
	job := &mock.Job{
		StatusResp: expectedStatus,
	}
	jr := runner.NewJobRunner(job, "abc")

	status := jr.Status()
	if status != expectedStatus {
		t.Errorf("status = %s, expected %s", status, expectedStatus)
	}
}
