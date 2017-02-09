// Copyright 2017, Square, Inc.

// Package proto provides all API and service-to-service (s2s) message
// structures and constants.
package proto

type Job struct {
	Name  string
	Type  string
	Bytes []byte
}

type Jobs []Job

// This is the JobChain that the RM sends to the JR.
type JobChain struct {
	RequestId     uint
	Jobs          Jobs
	AdjacencyList map[string][]string
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
	ChainState  byte
	JobStatuses JobStatuses
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
