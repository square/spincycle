// Copyright 2017, Square, Inc.

// Package proto provides all API and service-to-service (s2s) message
// structures and constants.
package proto

import (
	"time"
)

// Job represents one job in a job chain. Jobs are identified by Name, which
// must be unique within a job chain.
type Job struct {
	Name  string                 `json:"name"`  // unique name
	Type  string                 `json:"type"`  // user-specific job type
	Bytes []byte                 `json:"bytes"` // return value of Job.Serialize method
	State byte                   `json:"state"` // STATE_* const
	Data  map[string]interface{} `json:"data"`  // job-specific data during Job.Run
}

// JobChain represents a directed acyclic graph of jobs for one request.
// Job chains are identified by RequestId, which must be globally unique.
type JobChain struct {
	RequestId     uint                `json:"requestId"`     // unique identifier for the chain
	Jobs          map[string]Job      `json:"jobs"`          // Job.Name => job
	AdjacencyList map[string][]string `json:"adjacencyList"` // Job.Name => next jobs
	State         byte                `json:"state"`         // STATE_* const
	StartTime     time.Time           `json:"startTime"`     // when the chain started running
	EndTime       time.Time           `json:"endTime"`       // when the chain ended running
}

// JobStatus represents the status of one job in a job chain.
type JobStatus struct {
	Name   string `json:"name"`   // unique name
	Status string `json:"status"` // stdout of job, if any
	State  byte   `json:"state"`  // STATE_* const
}

// JobChainStatus represents the status of a job chain reported by the Job Runner.
type JobChainStatus struct {
	RequestId   uint        `json:"requestId"`
	JobStatuses JobStatuses `json:"jobStatuses"`
}

// JobStatuses are a list of job status sorted by job name.
type JobStatuses []JobStatus

func (js JobStatuses) Len() int {
	return len(js)
}

func (js JobStatuses) Less(i, j int) bool {
	return js[i].Name < js[j].Name
}

func (js JobStatuses) Swap(i, j int) {
	js[i], js[j] = js[j], js[i]
}

// Jobs are a list of jobs sorted by name.
type Jobs []Job

func (j Jobs) Len() int {
	return len(j)
}
func (j Jobs) Less(i, k int) bool {
	return j[i].Name < j[k].Name
}
func (j Jobs) Swap(i, k int) {
	j[i], j[k] = j[k], j[i]
}
