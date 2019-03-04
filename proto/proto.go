// Copyright 2017-2019, Square, Inc.

// Package proto provide API message structures and constants.
package proto

import (
	"fmt"
	"time"
)

const (
	STATE_UNKNOWN byte = iota

	// Normal states, in order
	STATE_PENDING  // not started
	STATE_RUNNING  // running
	STATE_COMPLETE // completed successfully

	// Error states, no order
	STATE_FAIL    // failed due to error or non-zero exit
	STATE_TIMEOUT // timeout
	STATE_STOPPED // stopped by user

	// A request or chain can be suspended and then resumed at a later time.
	// Jobs aren't suspended - they're stopped when a chain is suspended.
	STATE_SUSPENDED
)

var StateName = map[byte]string{
	STATE_UNKNOWN:   "UNKNOWN",
	STATE_PENDING:   "PENDING",
	STATE_RUNNING:   "RUNNING",
	STATE_COMPLETE:  "COMPLETE",
	STATE_FAIL:      "FAIL",
	STATE_TIMEOUT:   "TIMEOUT",
	STATE_STOPPED:   "STOPPED",
	STATE_SUSPENDED: "SUSPENDED",
}

var StateValue = map[string]byte{
	"UNKNOWN":   STATE_UNKNOWN,
	"PENDING":   STATE_PENDING,
	"RUNNING":   STATE_RUNNING,
	"COMPLETE":  STATE_COMPLETE,
	"FAIL":      STATE_FAIL,
	"TIMEOUT":   STATE_TIMEOUT,
	"STOPPED":   STATE_STOPPED,
	"SUSPENDED": STATE_SUSPENDED,
}

const (
	REQUEST_OP_START = "start"
	REQUEST_OP_STOP  = "stop"
)

// Job represents one job in a job chain. Jobs are identified by Id, which
// must be unique within a job chain.
type Job struct {
	Id            string                 `json:"id"`              // unique id
	Name          string                 `json:"name"`            // name of the job
	Type          string                 `json:"type"`            // user-specific job type
	Bytes         []byte                 `json:"bytes,omitempty"` // return value of Job.Serialize method
	State         byte                   `json:"state"`           // STATE_* const
	Args          map[string]interface{} `json:"args,omitempty"`  // the jobArgs a job was created with
	Data          map[string]interface{} `json:"data,omitempty"`  // job-specific data during Job.Run
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
	Id    string       `json:"id"`             // unique identifier for the request
	Type  string       `json:"type"`           // the type of request
	State byte         `json:"state"`          // STATE_* const
	User  string       `json:"user"`           // the user who made the request
	Args  []RequestArg `json:"args,omitempty"` // final request args (request_archives.args)

	CreatedAt  time.Time  `json:"createdAt"`  // when the request was created
	StartedAt  *time.Time `json:"startedAt"`  // when the request was sent to the job runner
	FinishedAt *time.Time `json:"finishedAt"` // when the job runner finished the request. doesn't indicate success/failure

	JobChain     *JobChain `json:",omitempty"`   // job chain (request_archives.job_chain)
	TotalJobs    int       `json:"totalJobs"`    // the number of jobs in the request's job chain
	FinishedJobs int       `json:"finishedJobs"` // the number of finished jobs in the request

	JobRunnerURL string `json:"jrURL,omitempty"` // URL of the job runner running the request
}

// SuspendedJobChain (SJC) represents the data required to reconstruct and resume a
// running job chain in the Job Runner.
type SuspendedJobChain struct {
	// Request corresponding to this SJC - unique identifier
	RequestId string    `json:"requestId"`
	JobChain  *JobChain `json:"jobChain"`

	// The total number of times a job has ever been tried, keyed on job.Id
	// This is the sum of the number of times the job was tried each time
	// that its sequence was tried.
	TotalJobTries map[string]uint `json:"totalJobTries"`

	// The number of times a job was tried the latest time it was run
	// (during the latest try of the sequence it's in), keyed on job.Id.
	LatestRunJobTries map[string]uint `json:"latestRunJobTries"`

	// The number of times a sequence has been tried, keyed on the
	// id of the first job in the sequence.
	SequenceTries map[string]uint `json:"sequenceTries"`
}

// RequestSpec represents the metadata of a request necessary to start the request.
type RequestSpec struct {
	Name string
	Args []RequestArg
}

// RequestArg represents an request argument and its metadata.
type RequestArg struct {
	Pos     int // position in request spec relative to required:, optional:, or static: stanza
	Name    string
	Desc    string
	Type    string      // required, optional, static
	Given   bool        // true if Required or Optional and value given
	Default interface{} // default value if Optional or Static
	Value   interface{} // final value
}

const (
	ARG_TYPE_REQUIRED = "required"
	ARG_TYPE_OPTIONAL = "optional"
	ARG_TYPE_STATIC   = "static"
)

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
	Status    string                 `json:"status"`    // real-time status, if running
	N         uint                   `json:"n"`         // Nth job ran in chain
	// @todo: Try uint
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

// CreateRequest represents the payload to create and start a new request.
type CreateRequest struct {
	Type string                 // the type of request being made
	Args map[string]interface{} // the arguments for the request
	User string                 // the user making the request
}

// FinishRequest represents the payload to tell the RM that a request has finished.
type FinishRequest struct {
	State      byte      // the final state of the chain
	FinishedAt time.Time // when the Job Runner finished the request
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

// Error is the standard response for all handled errors. Client errors (HTTP 400
// codes) and internal errors (HTTP 500 codes) are returned as an Error, if handled.
// If not handled (API crash, panic, etc.), Spin Cycle returns an HTTP 500 code and the
// response data is undefined; the client should print any response data as a string.
type Error struct {
	Message    string `json:"message"`    // human-readable and loggable error message
	RequestId  string `json:"requestId"`  // entity ID that caused error, if any
	HTTPStatus int    `json:"httpStatus"` // HTTP status code
}

func NewError(msgFmt string, msgArgs ...interface{}) Error {
	e := Error{}
	if msgFmt != "" {
		e.Message = fmt.Sprintf(msgFmt, msgArgs...)
	}
	return e
}

func (e Error) String() string {
	return e.Message
}

func (e Error) Error() string {
	return e.Message
}
