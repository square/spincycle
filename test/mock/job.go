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
	MockJobs map[string]*Job // keyed on job type
	MakeErr  error
	cnt      uint
	Created  map[string]*Job // keyed on name
}

func (f *JobFactory) Make(jobType, jobName string) (job.Job, error) {
	// Test-provided job
	job, ok := f.MockJobs[jobType]
	if !ok {
		// Auto-create new mock job
		f.cnt++
		job = &Job{
			TypeResp: jobType,
			NameResp: jobName,
		}
		if f.Created != nil {
			f.Created[jobName] = job
		}
	} else {
		job.NameResp = jobName
		job.TypeResp = jobType
	}
	return job, f.MakeErr
}

type Job struct {
	CreateErr       error
	SerializeBytes  []byte
	SerializeErr    error
	DeserializeErr  error
	RunReturn       job.Return
	RunErr          error
	RunFunc         func(jobData map[string]interface{}) (job.Return, error) // can use this instead of RunErr and RunFunc for more involved mocks
	StopErr         error
	StatusResp      string
	NameResp        string
	TypeResp        string
	CreatedWithArgs map[string]interface{}
	SetJobArgs      map[string]interface{}
}

func (j *Job) Create(jobArgs map[string]interface{}) error {
	j.CreatedWithArgs = map[string]interface{}{}
	for k, v := range jobArgs {
		j.CreatedWithArgs[k] = v
	}
	if j.SetJobArgs != nil {
		for k, v := range j.SetJobArgs {
			jobArgs[k] = v
		}
	}
	return j.CreateErr
}

func (j *Job) Serialize() ([]byte, error) {
	return j.SerializeBytes, j.SerializeErr
}

func (j *Job) Deserialize(jobArgs []byte) error {
	return j.DeserializeErr
}

func (j *Job) Run(jobData map[string]interface{}) (job.Return, error) {
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
