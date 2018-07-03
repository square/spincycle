// Copyright 2017-2018, Square, Inc.

// Package proto provides all API and service-to-service (s2s) message
// structures and constants.
package proto

import (
	"time"
)

// Job represents one job in a job chain. Jobs are identified by Id, which
// must be unique within a job chain.
type Job struct {
	Id            string                 `json:"id"`              // unique id
	Name          string                 `json:"name"`            // name of the job
	Type          string                 `json:"type"`            // user-specific job type
	Bytes         []byte                 `json:"bytes"`           // return value of Job.Serialize method
	State         byte                   `json:"state"`           // STATE_* const
	Args          map[string]interface{} `json:"args"`            // the jobArgs a job was created with
	Data          map[string]interface{} `json:"data"`            // job-specific data during Job.Run
	Retry         uint                   `json:"retry"`           // retry N times if first run fails
	RetryWait     uint                   `json:"retryWait"`       // wait time (milliseconds) between retries
	SequenceId    string                 `json:"sequenceStartId"` // ID for first job in sequence
	SequenceRetry uint                   `json:"sequenceRetry"`   // retry sequence N times if first run fails. Only set for first job in sequence.
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

// RequestSpec represents the metadata of a request necessary to start the request.
type RequestSpec struct {
	Name string
	Args []RequestArg
}

// RequestArg represents one request arg.
type RequestArg struct {
	Name     string
	Desc     string
	Required bool
	Static   bool
	Default  string
}

// JobLog represents a log entry for a finished job.
type JobLog struct {
	// These three fields uniquely identify an entry in the job log.
	RequestId   string `json:"requestId"`
	JobId       string `json:"jobId"`
	Try         uint   `json:"try"`         // try number that is monotonically increasing
	SequenceTry uint   `json:"sequenceTry"` // try number N of 1 + Job's sequence retry
	SequenceId  string `json:"sequenceId"`  // ID of first job in sequence

	Name       string `json:"name"`
	Type       string `json:"type"`
	StartedAt  int64  `json:"startedAt"`  // when job started (UnixNano)
	FinishedAt int64  `json:"finishedAt"` // when job finished, regardless of state (UnixNano)

	State  byte   `json:"state"`  // STATE_* const
	Exit   int64  `json:"exit"`   // unix exit code
	Error  string `json:"error"`  // error message
	Stdout string `json:"stdout"` // stdout output
	Stderr string `json:"stderr"` // stderr output
}

// JobStatus represents the status of one job in a job chain.
type JobStatus struct {
	RequestId string                 `json:"requestId"`
	JobId     string                 `json:"jobId"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	Args      map[string]interface{} `json:"jobArgs"`
	StartedAt int64                  `json:"startedAt"` // when job started (UnixNano)
	State     byte                   `json:"state"`     // usually proto.STATE_RUNNING
	Status    string                 `json:"status"`    // @todo: job.Status()
	N         uint                   `json:"n"`         // Nth job ran in chain
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

type RunningStatus struct {
	Jobs     []JobStatus        `json:"jobs,omitempty"`
	Requests map[string]Request `json:"requests,omitempty"` // keyed on RequestId
}

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

// JobStatuses are a list of job status sorted by job id.
type JobStatuses []JobStatus

func (js JobStatuses) Len() int           { return len(js) }
func (js JobStatuses) Less(i, j int) bool { return js[i].JobId < js[j].JobId }
func (js JobStatuses) Swap(i, j int)      { js[i], js[j] = js[j], js[i] }

type JobStatusByStartTime []JobStatus

func (js JobStatusByStartTime) Len() int           { return len(js) }
func (js JobStatusByStartTime) Less(i, j int) bool { return js[i].StartedAt > js[j].StartedAt }
func (js JobStatusByStartTime) Swap(i, j int)      { js[i], js[j] = js[j], js[i] }

// JobLogById is a slice of job logs sorted by request id + job id.
type JobLogById []JobLog

func (jls JobLogById) Len() int { return len(jls) }
func (jls JobLogById) Less(i, j int) bool {
	return jls[i].RequestId+jls[i].JobId < jls[j].RequestId+jls[j].JobId
}
func (jls JobLogById) Swap(i, j int) { jls[i], jls[j] = jls[j], jls[i] }

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
