// Copyright 2017, Square, Inc.

// Package proto provides all API and service-to-service (s2s) message
// structures and constants.
package proto

import (
	"time"
)

// Job represents one job in a job chain. Jobs are identified by Id, which
// must be unique within a job chain.
type Job struct {
	Id    string                 `json:"id"`    // unique id
	Type  string                 `json:"type"`  // user-specific job type
	Bytes []byte                 `json:"bytes"` // return value of Job.Serialize method
	State byte                   `json:"state"` // STATE_* const
	Args  map[string]interface{} `json:"args"`  // the jobArgs a job was created with
	Data  map[string]interface{} `json:"data"`  // job-specific data during Job.Run

	RetriesAllowed int `json:"retriesAllowed"` // the number of times a job can be retried
	RetryDelay     int `json:"retryDelay"`     // delay, in seconds, between retries
}

// JobChain represents a directed acyclic graph of jobs for one request.
// Job chains are identified by RequestId, which must be globally unique.
type JobChain struct {
	RequestId     string              `json:"requestId"`     // unique identifier for the chain
	Jobs          map[string]Job      `json:"jobs"`          // Job.Id => job
	AdjacencyList map[string][]string `json:"adjacencyList"` // Job.Id => next []Job.Id
	State         byte                `json:"state"`         // STATE_* const
}

// Request represents something that a user asks Spin Cycle to do.
type Request struct {
	Id    string `json:"id"`    // unique identifier for the request
	Type  string `json:"type"`  // the type of request
	State byte   `json:"state"` // STATE_* const
	User  string `json:"user"`  // the user who made the request

	CreatedAt time.Time `json:"createdAt"` // when the request was created
	// These are pointers so that they can have nil values.
	StartedAt  *time.Time `json:"startedAt"`  // when the request was sent to the job runner
	FinishedAt *time.Time `json:"finishedAt"` // when the job runner finished the request. doesn't indicate success/failure

	JobChain     *JobChain `json:",omitempty"`   // the job chain
	TotalJobs    int       `json:"totalJobs"`    // the number of jobs in the request's job chain
	FinishedJobs int       `json:"finishedJobs"` // the number of finished jobs in the request
}

// JobLog represents a log entry for a finished job.
type JobLog struct {
	RequestId string `json:"requestId"` // the request that the job belongs to
	JobId     string `json:"jobId"`     // the id of the job
	Type      string `json:"type"`      // the type of the job

	StartedAt  time.Time `json:"startedAt"`  // when the request was sent to the job runner
	FinishedAt time.Time `json:"finishedAt"` // when the job runner finished the request. doesn't indicate success/failure

	State  byte   `json:"state"`  // STATE_* const
	Exit   int64  `json:"exit"`   // unix exit code
	Error  string `json:"error"`  // error message
	Stdout string `json:"stdout"` // stdout output
	Stderr string `json:"stderr"` // stderr output

	Attempt int `json:"attempt"` // the attempt number for running the job (ex: 1 for first, 2 for second)
}

// JobStatus represents the status of one job in a job chain.
type JobStatus struct {
	Id     string `json:"id"`     // unique id
	Status string `json:"status"` // stdout of job, if any
	State  byte   `json:"state"`  // STATE_* const
}

// JobChainStatus represents the status of a job chain reported by the Job Runner.
type JobChainStatus struct {
	RequestId   string      `json:"requestId"`
	JobStatuses JobStatuses `json:"jobStatuses"`
}

// RequestStatus represents the status of a request reported by the Request Manager.
type RequestStatus struct {
	Request        `json:"request"`
	JobChainStatus JobChainStatus `json:"jobChainStatus"`
}

// JobStatuses are a list of job status sorted by job id.
type JobStatuses []JobStatus

// CreateRequestParams represents the payload that is required to create a new
// request in the RM.
type CreateRequestParams struct {
	Type string                 // the type of request being made
	Args map[string]interface{} // the arguments for the request
	User string                 // the user making the request
}

// FinishRequestParams represents the payload that is required to tell the RM
// that a request has finished.
type FinishRequestParams struct {
	State byte // the final state of the chain
}

func (js JobStatuses) Len() int {
	return len(js)
}

func (js JobStatuses) Less(i, j int) bool {
	return js[i].Id < js[j].Id
}

func (js JobStatuses) Swap(i, j int) {
	js[i], js[j] = js[j], js[i]
}

// Jobs are a list of jobs sorted by id.
type Jobs []Job

func (j Jobs) Len() int {
	return len(j)
}
func (j Jobs) Less(i, k int) bool {
	return j[i].Id < j[k].Id
}
func (j Jobs) Swap(i, k int) {
	j[i], j[k] = j[k], j[i]
}
