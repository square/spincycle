// Copyright 2017-2019, Square, Inc.

// Package proto provide API message structures and constants.
package proto

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DO NOT change the state values. The raw byte values is stored in tables,
// so changing any value breaks everything. Add new states/values if needed.

const (
	STATE_UNKNOWN byte = 0

	// Normal states, in order
	STATE_PENDING  byte = 1 // not started
	STATE_RUNNING  byte = 2 // running
	STATE_COMPLETE byte = 3 // completed successfully

	STATE_FAIL     byte = 4 // failed, job/seq retry if possible
	STATE_RESERVED byte = 5 // reserved (used to be TIMEOUT)
	STATE_STOPPED  byte = 6 // stopped by user or API shutdown

	// A request or chain can be suspended and then resumed at a later time.
	// Jobs aren't suspended - they're stopped when a chain is suspended.
	STATE_SUSPENDED byte = 7
)

var StateName = map[byte]string{
	STATE_UNKNOWN:   "UNKNOWN",
	STATE_PENDING:   "PENDING",
	STATE_RUNNING:   "RUNNING",
	STATE_COMPLETE:  "COMPLETE",
	STATE_FAIL:      "FAIL",
	STATE_RESERVED:  "RESERVED",
	STATE_STOPPED:   "STOPPED",
	STATE_SUSPENDED: "SUSPENDED",
}

var StateValue = map[string]byte{
	"UNKNOWN":   STATE_UNKNOWN,
	"PENDING":   STATE_PENDING,
	"RUNNING":   STATE_RUNNING,
	"COMPLETE":  STATE_COMPLETE,
	"FAIL":      STATE_FAIL,
	"RESERVED":  STATE_RESERVED,
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
	Id                string                 `json:"id"`                  // unique id
	Name              string                 `json:"name"`                // name of the job
	Type              string                 `json:"type"`                // user-specific job type
	Bytes             []byte                 `json:"bytes,omitempty"`     // return value of Job.Serialize method
	State             byte                   `json:"state"`               // STATE_* const
	Args              map[string]interface{} `json:"args,omitempty"`      // the jobArgs a job was created with
	Data              map[string]interface{} `json:"data,omitempty"`      // job-specific data during Job.Run
	Retry             uint                   `json:"retry"`               // retry N times if first run fails
	RetryWait         string                 `json:"retryWait,omitempty"` // wait between tries (duration string: "N{ms|s|m|h}", default: 0s)
	SequenceId        string                 `json:"sequenceId"`          // Job.Id of first job in sequence
	SequenceRetry     uint                   `json:"sequenceRetry"`       // retry sequence N times if first run fails. Only set for first job in sequence.
	SequenceRetryWait string                 `json:"sequenceRetryWait"`   // wait between sequence tries (duration string: "N{ms|s|m|h}", default: 0s)
}

// JobChain represents a directed acyclic graph of jobs for one request.
// Job chains are identified by RequestId, which must be globally unique.
type JobChain struct {
	RequestId     string              `json:"requestId"`     // unique identifier for the chain
	Jobs          map[string]Job      `json:"jobs"`          // Job.Id => job
	AdjacencyList map[string][]string `json:"adjacencyList"` // Job.Id => next []Job.Id
	State         byte                `json:"state"`         // STATE_* const
	FinishedJobs  uint                `json:"finishedJobs"`  // number of jobs that ran and finished with state = STATE_COMPLETE
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
	TotalJobs    uint      `json:"totalJobs"`    // number of jobs in the request's job chain
	FinishedJobs uint      `json:"finishedJobs"` // number of jobs that ran and finished with state = STATE_COMPLETE

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
	RequestId string `json:"requestId"`
	JobId     string `json:"jobId"`
	Try       uint   `json:"try"` // try number that is monotonically increasing

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

type JobLogById []JobLog

func (jls JobLogById) Len() int { return len(jls) }
func (jls JobLogById) Less(i, j int) bool {
	return jls[i].RequestId+jls[i].JobId < jls[j].RequestId+jls[j].JobId
}
func (jls JobLogById) Swap(i, j int) { jls[i], jls[j] = jls[j], jls[i] }

// JobStatus represents the status of one job in a job chain.
type JobStatus struct {
	RequestId string `json:"requestId"`
	JobId     string `json:"jobId"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	StartedAt int64  `json:"startedAt"`        // when job started (UnixNano)
	State     byte   `json:"state"`            // usually proto.STATE_RUNNING
	Status    string `json:"status,omitempty"` // real-time status, if running
	Try       uint   `json:"try"`              // try number, can be >1+retry on sequence retry
}

// JobStatusByStartTime sorts []JobStatus by StartedAt ascending (oldest jobs first).
type JobStatusByStartTime []JobStatus

func (js JobStatusByStartTime) Len() int           { return len(js) }
func (js JobStatusByStartTime) Less(i, j int) bool { return js[i].StartedAt < js[j].StartedAt }
func (js JobStatusByStartTime) Swap(i, j int)      { js[i], js[j] = js[j], js[i] }

// RequestProgress updates request progress from the Job Runner.
type RequestProgress struct {
	RequestId    string `json:"requestId"`
	FinishedJobs uint   `json:"finishedJobs"` // number of jobs that ran and finished with state = STATE_COMPLETE
}

// RunningStatus represents running jobs and their requests. It is returned by
// Request Manager GET /api/v1/status/running
type RunningStatus struct {
	Jobs     []JobStatus        `json:"jobs"`
	Requests map[string]Request `json:"requests"` // keyed on RequestId
}

// StatusFilter represents optional filters for status requests.
type StatusFilter struct {
	RequestId string
	OrderBy   string // startTime
}

func (f StatusFilter) String() string {
	q := []string{}
	if f.RequestId != "" {
		q = append(q, "requestId="+f.RequestId)
	}
	if f.OrderBy != "" {
		q = append(q, "orderBy="+strings.ToLower(f.OrderBy))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + strings.Join(q, "&")
}

// CreateRequest represents the payload to create and start a new request.
type CreateRequest struct {
	Type string                 // the type of request being made
	Args map[string]interface{} // the arguments for the request
	User string                 // the user making the request
}

// FinishRequest represents the payload to tell the RM that a request has finished.
type FinishRequest struct {
	RequestId    string    `json:"requestId"`
	State        byte      `json:"state"`        // the final state of the chain
	FinishedAt   time.Time `json:"finishedAt"`   // when the Job Runner finished the request
	FinishedJobs uint      `json:"finishedJobs"` // number of jobs that ran and finished with state = STATE_COMPLETE
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

// RequestFilter represents optional filters when listing requests.
type RequestFilter struct {
	Type   string // Type of requests to return.
	States []byte // Request states to include.
	User   string // User who made the request.

	// Return only requests that were created and run at any point within the time
	// range. I.e. Requests created before Since but finished after Since will
	// still be returned, as will requests created before Until but not finished
	// until after Until.
	Since time.Time
	Until time.Time

	// Use these options for pagination of results:
	Limit  uint // Limit response to this many requests
	Offset uint // Skip the first <Offset> requests. Ignored if Limit is not set.
}

// Return the query string representation of the Request Filter.
func (f RequestFilter) String() string {
	params := url.Values{}
	if f.Type != "" {
		params.Add("type", f.Type)
	}
	if len(f.States) != 0 {
		for _, state := range f.States {
			params.Add("state", StateName[state])
		}
	}
	if f.User != "" {
		params.Add("user", f.User)
	}
	if !f.Since.IsZero() {
		params.Add("since", f.Since.Format(time.RFC3339Nano))
	}
	if !f.Until.IsZero() {
		params.Add("until", f.Until.Format(time.RFC3339Nano))
	}
	if f.Limit != 0 {
		params.Add("limit", strconv.FormatUint(uint64(f.Limit), 10))
	}
	if f.Offset != 0 {
		params.Add("offset", strconv.FormatUint(uint64(f.Offset), 10))
	}
	return params.Encode()
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
