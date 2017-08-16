// Copyright 2017, Square, Inc.

// Package chain implements a job chain. It provides the ability to traverse a chain
// and run all of the jobs in it.
package chain

import (
	"fmt"
	"sync"

	"github.com/square/spincycle/proto"
)

// chain represents a job chain and some meta information about it.
type chain struct {
	// The jobchain.
	JobChain *proto.JobChain `json:"jobChain"`

	// Protection for the chain.
	*sync.RWMutex
}

// NewChain takes a JobChain proto (from the RM) and turns it into a Chain that
// the JR can use.
func NewChain(jc *proto.JobChain) *chain {
	// Set the state of all jobs in the chain to "Pending".
	for jobName, job := range jc.Jobs {
		job.State = proto.STATE_PENDING
		job.Data = map[string]interface{}{}
		jc.Jobs[jobName] = job
	}

	return &chain{
		JobChain: jc,
		RWMutex:  &sync.RWMutex{},
	}
}

// ErrInvalidChain is the error returned when a chain is not valid.
type ErrInvalidChain struct {
	Message string
}

func (e ErrInvalidChain) Error() string {
	return e.Error()
}

// FirstJob finds the job in the chain with indegree 0. If there is not
// exactly one of these jobs, it returns an error.
func (c *chain) FirstJob() (proto.Job, error) {
	var jobIds []string
	for jobId, count := range c.indegreeCounts() {
		if count == 0 {
			jobIds = append(jobIds, jobId)
		}
	}

	if len(jobIds) != 1 {
		return proto.Job{}, ErrInvalidChain{
			Message: fmt.Sprintf("chain has %d first job(s), should "+
				"have one (first job(s) = %v)", len(jobIds), jobIds),
		}
	}

	return c.JobChain.Jobs[jobIds[0]], nil
}

// LastJob finds the job in the chain with outdegree 0. If there is not
// exactly one of these jobs, it returns an error.
func (c *chain) LastJob() (proto.Job, error) {
	var jobIds []string
	for jobId, count := range c.outdegreeCounts() {
		if count == 0 {
			jobIds = append(jobIds, jobId)
		}
	}

	if len(jobIds) != 1 {
		return proto.Job{}, ErrInvalidChain{
			Message: fmt.Sprintf("chain has %d last job(s), should "+
				"have one (last job(s) = %v)", len(jobIds), jobIds),
		}
	}

	return c.JobChain.Jobs[jobIds[0]], nil
}

// NextJobs finds all of the jobs adjacent to the given job.
func (c *chain) NextJobs(jobId string) proto.Jobs {
	var nextJobs proto.Jobs
	if nextJobIds, ok := c.JobChain.AdjacencyList[jobId]; ok {
		for _, id := range nextJobIds {
			if val, ok := c.JobChain.Jobs[id]; ok {
				nextJobs = append(nextJobs, val)
			}
		}
	}

	return nextJobs
}

// PreviousJobs finds all of the immediately previous jobs to a given job.
func (c *chain) PreviousJobs(jobId string) proto.Jobs {
	var prevJobs proto.Jobs
	for curJob, nextJobs := range c.JobChain.AdjacencyList {
		if contains(nextJobs, jobId) {
			if val, ok := c.JobChain.Jobs[curJob]; ok {
				prevJobs = append(prevJobs, val)
			}
		}
	}
	return prevJobs
}

// JobIsReady returns whether or not a job is ready to run. A job is considered
// ready to run if all of its previous jobs are complete. If any previous jobs
// are not complete, the job is not ready to run.
func (c *chain) JobIsReady(jobId string) bool {
	isReady := true
	for _, job := range c.PreviousJobs(jobId) {
		if job.State != proto.STATE_COMPLETE {
			isReady = false
		}
	}
	return isReady
}

// IsDone returns two booleans - the first one indicates whether or not the
// chain is done, and the second one indicates whether or not the chain is
// complete.
//
// A chain is done running if there are no more jobs in it that can run. This
// can happen if all of the jobs in the chain or complete, or if some or all
// of the jobs in the chain failed.
//
// A chain is complete if every job in it completed successfully.
func (c *chain) IsDone() (done bool, complete bool) {
	done = true
	complete = true
	pendingJobs := proto.Jobs{}

	// Loop through every job in the chain and act on its state. Keep
	// track of the jobs that aren't running or in a finished state so
	// that we can later check to see if they are capable of running.
LOOP:
	for _, job := range c.JobChain.Jobs {
		switch job.State {
		case proto.STATE_RUNNING:
			// If any jobs are running, the chain can't be done
			// or complete, so return false for both now.
			return false, false
		case proto.STATE_COMPLETE:
			// Move on to the next job.
			continue LOOP
		case proto.STATE_FAIL:
			// do nothing
		default:
			// Any job that's not running, complete, or failed.
			pendingJobs = append(pendingJobs, job)
		}

		// We can only arrive here if a job is not complete. If there
		// is at least one job that is not complete, the whole chain is
		// not complete. The chain could still be done, though, so we
		// aren't ready to return yet.
		complete = false
	}

	// For each pending job, check to see if all of its previous jobs
	// completed. If they did, there's no reason the pending job can't run.
	for _, job := range pendingJobs {
		complete = false
		allPrevComplete := true
		for _, prevJob := range c.PreviousJobs(job.Id) {
			if prevJob.State != proto.STATE_COMPLETE {
				allPrevComplete = false
				// We can break out of this loop if a single
				// one of the previous jobs is not complete.
				break
			}
		}

		// If all of the previous jobs of a pending job are complete, the
		// chain can't be complete because the pending job can still run.
		if allPrevComplete == true {
			return false, complete
		}
	}

	return
}

// Validate checks if a job chain is valid. It returns an error if it's not.
func (c *chain) Validate() error {
	// Make sure the adjacency list is valid.
	if !c.adjacencyListIsValid() {
		return ErrInvalidChain{
			Message: "invalid adjacency list: some jobs exist in " +
				"chain.AdjacencyList but not chain.Jobs",
		}
	}

	// Make sure there is one first job.
	_, err := c.FirstJob()
	if err != nil {
		return err
	}

	// Make sure there is one last job.
	_, err = c.LastJob()
	if err != nil {
		return err
	}

	// Make sure there are no cycles.
	if !c.isAcyclic() {
		return ErrInvalidChain{Message: "chain is cyclic"}
	}

	return nil
}

// RequestId returns the request id of the job chain.
func (c *chain) RequestId() string {
	return c.JobChain.RequestId
}

// JobState returns the state of a given job.
func (c *chain) JobState(jobId string) byte {
	c.RLock()         // -- lock
	defer c.RUnlock() // -- unlock
	return c.JobChain.Jobs[jobId].State
}

// Set the state of a job in the chain.
func (c *chain) SetJobState(jobId string, state byte) {
	c.Lock() // -- lock
	j := c.JobChain.Jobs[jobId]
	j.State = state
	c.JobChain.Jobs[jobId] = j
	c.Unlock() // -- unlock
}

// SetState sets the chain's state.
func (c *chain) SetState(state byte) {
	c.Lock() // -- lock
	c.JobChain.State = state
	c.Unlock() // -- unlock
}

// -------------------------------------------------------------------------- //

// indegreeCounts finds the indegree for each job in the chain.
func (c *chain) indegreeCounts() map[string]int {
	indegreeCounts := make(map[string]int)
	for job := range c.JobChain.Jobs {
		indegreeCounts[job] = 0
	}

	for _, nextJobs := range c.JobChain.AdjacencyList {
		for _, nextJob := range nextJobs {
			if _, ok := indegreeCounts[nextJob]; ok {
				indegreeCounts[nextJob] += 1
			}
		}
	}

	return indegreeCounts
}

// outdegreeCounts finds the outdegree for each job in the chain.
func (c *chain) outdegreeCounts() map[string]int {
	outdegreeCounts := make(map[string]int)
	for job := range c.JobChain.Jobs {
		outdegreeCounts[job] = len(c.JobChain.AdjacencyList[job])
	}

	return outdegreeCounts
}

// isAcyclic returns whether or not a job chain is acyclic. It essentially
// works by moving through the job chain from the top (the first job)
// down to the bottom (the last job), and if there are any cycles in the
// chain (dependencies that go in the opposite direction...i.e., bottom to
// top), it returns false.
func (c *chain) isAcyclic() bool {
	indegreeCounts := c.indegreeCounts()
	queue := make(map[string]struct{})

	// Add all of the first jobs to the queue (in reality there should
	// only be 1).
	for job, indegreeCount := range indegreeCounts {
		if indegreeCount == 0 {
			queue[job] = struct{}{}
		}
	}

	jobsVisited := 0
	for {
		// Break when there are no more jobs in the queue. This happens
		// when either there are no first jobs, or when a cycle
		// prevents us from enqueuing a job below.
		if len(queue) == 0 {
			break
		}

		// Get a job from the queue.
		var curJob string
		for k := range queue {
			curJob = k
		}
		delete(queue, curJob)

		// Visit each job adjacent to the current job and decrement
		// their indegree count by 1. When a job's indegree count
		// becomes 0, add it to the queue.
		//
		// If there is a cycle somewhere, at least one jobs indegree
		// count will never reach 0, and therefore it will never be
		// enqueued and visited.
		for _, adjJob := range c.JobChain.AdjacencyList[curJob] {
			indegreeCounts[adjJob] -= 1
			if indegreeCounts[adjJob] == 0 {
				queue[adjJob] = struct{}{}
			}
		}

		// Keep track of the number of jobs we've visited. If there is
		// a cycle in the chain, we won't end up visiting some jobs.
		jobsVisited += 1
	}

	if jobsVisited != len(c.JobChain.Jobs) {
		return false
	}

	return true
}

// adjacencyListIsValid returns whether or not the chain's adjacency list is
// not valid. An adjacency list is not valid if any of the jobs in it do not
// exist in chain.Jobs.
func (c *chain) adjacencyListIsValid() bool {
	for job, adjJobs := range c.JobChain.AdjacencyList {
		if _, ok := c.JobChain.Jobs[job]; !ok {
			return false
		}

		for _, adjJob := range adjJobs {
			if _, ok := c.JobChain.Jobs[adjJob]; !ok {
				return false
			}
		}
	}
	return true
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
