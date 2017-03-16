// Copyright 2017, Square, Inc.

package mock

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
)

var (
	ErrJob = errors.New("forced error in job")
)

type JobFactory struct {
	JobToReturn job.Job
	MakeErr     error
}

func (f *JobFactory) Make(jobType, jobName string) (job.Job, error) {
	return f.JobToReturn, f.MakeErr
}

type Job struct {
	CreateErr      error
	SerializeBytes []byte
	SerializeErr   error
	DeserializeErr error
	RunReturn      job.Return
	AddedJobData   map[string]interface{} // Data to add to jobData.
	RunBlock       chan struct{}          // Channel that job.Run() will block on, if defined.
	StopErr        error
	StatusResp     string
	NameResp       string
	TypeResp       string
}

func (j *Job) Create(jobArgs map[string]string) error {
	return j.CreateErr
}

func (j *Job) Serialize() ([]byte, error) {
	return j.SerializeBytes, j.SerializeErr
}

func (j *Job) Deserialize(jobArgs []byte) error {
	return j.DeserializeErr
}

func (j *Job) Run(jobData map[string]interface{}) job.Return {
	if j.RunBlock != nil {
		<-j.RunBlock
	}
	// Add job data.
	for k, v := range j.AddedJobData {
		jobData[k] = v
	}
	return j.RunReturn
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

// InitJobs initializes some mock proto jobs.
func InitJobs(count int) map[string]proto.Job {
	jobs := make(map[string]proto.Job)
	for i := 1; i <= count; i++ {
		bytes := make([]byte, 10)
		rand.Read(bytes)
		job := proto.Job{
			Name:  fmt.Sprintf("job%d", i),
			Bytes: bytes,
		}
		jobs[job.Name] = job
	}
	return jobs
}
