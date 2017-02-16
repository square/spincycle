// Copyright 2017, Square, Inc.

// Package proto provides all API and service-to-service (s2s) message
// structures and constants.
package proto

import (
	"time"
)

type Job struct {
	Name  string
	Type  string
	Bytes []byte
	State byte
}

type Jobs []Job

// This is the JobChain that the RM sends to the JR.
type JobChain struct {
	RequestId     uint                `json:"requestId"`     // Unique identifier for the chain.
	Jobs          map[string]Job      `json:"jobs"`          // jobName => job
	AdjacencyList map[string][]string `json:"adjacencyList"` // jobName => slice of jobNames
	State         byte                `json:"state"`         // The state of the chain.
	StartTime     time.Time           `json:"startTime"`     // When the chain started running.
	EndTime       time.Time           `json:"endTime"`       // When the chain ended running.
}

type JobStatus struct {
	Name   string
	Status string // Live status response from a running job.
	State  byte   // The state that a job is in (ex: running, pending, etc.).
}

type JobStatuses []JobStatus

// This is how the JR sends statuses back to the RM.
type JobChainStatus struct {
	RequestId   uint
	JobStatuses JobStatuses
}

// Function needed to implement sorting on Jobs.
func (j Jobs) Len() int {
	return len(j)
}

// Function needed to implement sorting on Jobs.
func (j Jobs) Less(i, k int) bool {
	return j[i].Name < j[k].Name
}

// Function needed to implement sorting on Jobs.
func (j Jobs) Swap(i, k int) {
	j[i], j[k] = j[k], j[i]
}

// Function needed to implement sorting on JobStatuses.
func (js JobStatuses) Len() int {
	return len(js)
}

// Function needed to implement sorting on JobStatuses.
func (js JobStatuses) Less(i, j int) bool {
	return js[i].Name < js[j].Name
}

// Function needed to implement sorting on JobStatuses.
func (js JobStatuses) Swap(i, j int) {
	js[i], js[j] = js[j], js[i]
}
