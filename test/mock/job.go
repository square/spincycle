// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

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
	RunFunc        func(jobData map[string]interface{}) (job.Return, error) // can use this instead of RunErr and RunFunc for more involved mocks
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
	// If RunFunc is defined, use that.
	if j.RunFunc != nil {
		return j.RunFunc(jobData)
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
