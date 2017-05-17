// Copyright 2017, Square, Inc.

package mock

import (
	"errors"
	"sync"

	"github.com/square/spincycle/job"
)

var (
	ErrJob = errors.New("forced error in job")
)

type JobFactory struct {
	JobsToReturn map[string]*Job // Keyed on job name.
	MakeErr      error
}

func (f *JobFactory) Make(jobType, jobName string) (job.Job, error) {
	return f.JobsToReturn[jobName], f.MakeErr
}

type Job struct {
	CreateErr      error
	SerializeBytes []byte
	SerializeErr   error
	DeserializeErr error
	RunReturn      job.Return
	RunErr         error
	AddedJobData   map[string]interface{} // Data to add to jobData.
	RunWg          *sync.WaitGroup        // WaitGroup that gets released from when a job starts running.
	RunBlock       chan struct{}          // Channel that job.Run() will block on, if defined.
	StopErr        error
	StatusResp     string
	NameResp       string
	TypeResp       string
}

func (j *Job) Create(jobArgs map[string]interface{}) error {
	return j.CreateErr
}

func (j *Job) Serialize() ([]byte, error) {
	return j.SerializeBytes, j.SerializeErr
}

func (j *Job) Deserialize(jobArgs []byte) error {
	return j.DeserializeErr
}

func (j *Job) Run(jobData map[string]interface{}) (job.Return, error) {
	if j.RunWg != nil {
		j.RunWg.Done()
	}
	if j.RunBlock != nil {
		<-j.RunBlock
	}
	// Add job data.
	for k, v := range j.AddedJobData {
		jobData[k] = v
	}
	return j.RunReturn, j.RunErr
}

func (j *Job) Stop() error {
	return j.StopErr
}

func (j *Job) Status() string {
	return j.StatusResp
}

func (j *Job) Name() string {
	return j.NameResp
}

func (j *Job) Type() string {
	return j.TypeResp
}
