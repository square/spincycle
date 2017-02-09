// Copyright 2017, Square, Inc.

package chain

import (
	"fmt"
	"time"

	"github.com/square/spincycle/proto"
)

// SerializedJob contains the serialized byte array of a job along with some other
// meta information about the job.
type SerializedJob struct {
	// The name of the job.
	Name string `json:"name"`

	// The serialized byte array of the job.
	Bytes []byte `json:"bytes"`

	// The state of the job.
	State byte `json:"state"`

	// The type of the job.
	Type string `json:"type"`
}

// SerializedJobs is a slice of type SerializedJob.
type SerializedJobs []*SerializedJob

// Len retruns the length of a SerializedJobs slice. It is needed to implement sorting.
func (s SerializedJobs) Len() int {
	return len(s)
}

// Less returns whether or not the element with index i sorts before the element with
// index j. It is needed to implent sorting.
func (s SerializedJobs) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// Swap swaps the element with index i with the element with index j. It is needed to
// implement sorting.
func (s SerializedJobs) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// A chain represents a job chain. It contains meta information about the job chain
// itself, as well as the serialized representations of all the jobs in the job
// chain and their dependencies.
type Chain struct {
	// Unique id for the chain. This is the common identifier for the chain
	// between the Request Manager and the Job Runner API.
	RequestId uint `json:"requestId"`

	// List of serialized jobs.
	Jobs map[string]*SerializedJob `json:"jobs"`

	// Adjaceny list for jobs. Used to determine run dependencies in the chian.
	AdjacencyList map[string][]string `json:"adjacencyList"`

	// The state of the chain.
	State byte `json:"state"`

	// Data shared between jobs. Any job in the chain can read from or write to
	// this at any time in a free-for-all matter (no concurrency control)
	JobData map[string]string `json:"data"`

	// Time of when the chain first started.
	Started time.Time `json:"started"`

	// Time of when the chain finished.
	Finished time.Time `json:"finished"`
}

// NewChain takes a JobChain proto (from the RM) and turns it into a Chain that
// the JR can use.
func NewChain(c *proto.JobChain) *Chain {
	serializedJobs := make(map[string]*SerializedJob)
	for _, job := range c.Jobs {
		serializedJobs[job.Name] = &SerializedJob{job.Name, job.Bytes, proto.STATE_PENDING, job.Type}
	}

	return &Chain{
		RequestId:     c.RequestId,
		Jobs:          serializedJobs,
		AdjacencyList: c.AdjacencyList,
		State:         proto.STATE_PENDING,
		JobData:       make(map[string]string),
	}

}

// RootJob finds the job in the chain with indegree 0. If there is not
// exactly one of these jobs, it returns an error.
func (c *Chain) RootJob() (*SerializedJob, error) {
	var jobNames []string
	for jobName, count := range c.indegreeCounts() {
		if count == 0 {
			jobNames = append(jobNames, jobName)
		}
	}

	if len(jobNames) != 1 {
		err := fmt.Errorf("Chain %d does not have exactly one root job.", c.RequestId)
		return nil, err
	}

	return c.Jobs[jobNames[0]], nil
}

// NextJobs finds all of the jobs adjacent to given job.
func (c *Chain) NextJobs(jobName string) SerializedJobs {
	var nextJobs SerializedJobs
	if nextJobNames, ok := c.AdjacencyList[jobName]; ok {
		for _, name := range nextJobNames {
			nextJobs = append(nextJobs, c.Jobs[name])
		}
	}

	return nextJobs
}

// PreviousJobs finds all of the immediately previous jobs to a given job.
func (c *Chain) PreviousJobs(jobName string) SerializedJobs {
	var prevJobs SerializedJobs
	for curJob, nextJobs := range c.AdjacencyList {
		if contains(nextJobs, jobName) {
			prevJobs = append(prevJobs, c.Jobs[curJob])
		}
	}
	return prevJobs
}

// JobIsReady returns whether or not a job is ready to run. A job is considered
// ready to run if all of its previous jobs are complete. If any previous jobs
// are not complete, the job is not ready to run.
func (c *Chain) JobIsReady(jobName string) bool {
	isReady := true
	for _, serialized := range c.PreviousJobs(jobName) {
		if serialized.State != proto.STATE_COMPLETE {
			isReady = false
		}
	}
	return isReady
}

// Complete returns whether or not a chain is complete. A chain is considered
// complete if it is done running and all of the jobs in it are complete.
func (c *Chain) Complete() bool {
	for _, serialized := range c.Jobs {
		if serialized.State != proto.STATE_COMPLETE {
			return false
		}
	}
	return true
}

// Incomplete returns whether or not a chain is incomplete. A chain is
// considered incomplete if it is done running but not all of the jobs in it
// are complete (i.e., there are failed jobs or pending jobs which can never
// run because they depend on failed jobs).
func (c *Chain) Incomplete() bool {
	pendingJobs := SerializedJobs{}
	for _, serialized := range c.Jobs {
		// If any jobs are running, the chain can't be incomplete.
		if serialized.State == proto.STATE_RUNNING {
			return false
		} else if serialized.State == proto.STATE_PENDING {
			pendingJobs = append(pendingJobs, serialized)
		}
	}

	for _, serialized := range pendingJobs {
		allPrevComplete := true
		for _, prevJob := range c.PreviousJobs(serialized.Name) {
			if prevJob.State != proto.STATE_COMPLETE {
				allPrevComplete = false
				break
			}
		}

		// If all of the previous jobs of any pending jobs are
		// complete, the chain can't be incomplete because the
		// pending job can still run.
		if allPrevComplete == true {
			return false
		}
	}

	return true
}

// indegreeCounts finds the indegree for each job in the chain.
func (c *Chain) indegreeCounts() map[string]int {
	indegreeCounts := make(map[string]int)
	for job, _ := range c.Jobs {
		indegreeCounts[job] = 0
	}

	for _, nextJobs := range c.AdjacencyList {
		for _, nextJob := range nextJobs {
			indegreeCounts[nextJob] += 1
		}
	}

	return indegreeCounts
}

// contains returns whether or not a slice of strings contains a specific string.
func contains(s []string, t string) bool {
	for _, i := range s {
		if i == t {
			return true
		}
	}
	return false
}
