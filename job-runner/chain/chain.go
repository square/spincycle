// Copyright 2017, Square, Inc.

// Package chain implements a job chain. It provides the ability to traverse a chain
// and run all of the jobs in it.
package chain

import (
	"errors"
	"sync"
	"time"

	"github.com/square/spincycle/proto"
)

var (
	ErrFirstJob             = errors.New("chain does not have exactly one first job")
	ErrLastJob              = errors.New("chain does not have exactly one last job")
	ErrCyclic               = errors.New("chain is cyclic")
	ErrInvalidAdjacencyList = errors.New("chain does not have a valid adjacency list")
)

// chain represents a job chain and some meta information about it.
type chain struct {
	// The jobchain.
	JobChain *proto.JobChain `json:"jobChain"`

	// Data shared between jobs. Any job in the chain can read from or write
	// to this.
	JobData map[string]string `json:"data"`

	// Protection for the chain.
	*sync.RWMutex
}

// NewChain takes a JobChain proto (from the RM) and turns it into a Chain that
// the JR can use.
func NewChain(jc *proto.JobChain) *chain {
	// Set the state of all jobs in the chain to "Pending".
	for _, job := range jc.Jobs {
		job.State = proto.STATE_PENDING
		jc.Jobs[job.Name] = job
	}

	return &chain{
		JobChain: jc,
		JobData:  make(map[string]string),
		RWMutex:  &sync.RWMutex{},
	}
}

// FirstJob finds the job in the chain with indegree 0. If there is not
// exactly one of these jobs, it returns an error.
func (c *chain) FirstJob() (proto.Job, error) {
	var jobNames []string
	for jobName, count := range c.indegreeCounts() {
		if count == 0 {
			jobNames = append(jobNames, jobName)
		}
	}

	if len(jobNames) != 1 {
		err := ErrFirstJob
		return proto.Job{}, err
	}

	return c.JobChain.Jobs[jobNames[0]], nil
}

// LastJob finds the job in the chain with outdegree 0. If there is not
// exactly one of these jobs, it returns an error.
func (c *chain) LastJob() (proto.Job, error) {
	var jobNames []string
	for jobName, count := range c.outdegreeCounts() {
		if count == 0 {
			jobNames = append(jobNames, jobName)
		}
	}

	if len(jobNames) != 1 {
		err := ErrLastJob
		return proto.Job{}, err
	}

	return c.JobChain.Jobs[jobNames[0]], nil
}

// NextJobs finds all of the jobs adjacent to the given job.
func (c *chain) NextJobs(jobName string) proto.Jobs {
	var nextJobs proto.Jobs
	if nextJobNames, ok := c.JobChain.AdjacencyList[jobName]; ok {
		for _, name := range nextJobNames {
			if val, ok := c.JobChain.Jobs[name]; ok {
				nextJobs = append(nextJobs, val)
			}
		}
	}

	return nextJobs
}

// PreviousJobs finds all of the immediately previous jobs to a given job.
func (c *chain) PreviousJobs(jobName string) proto.Jobs {
	var prevJobs proto.Jobs
	for curJob, nextJobs := range c.JobChain.AdjacencyList {
		if contains(nextJobs, jobName) {
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
func (c *chain) JobIsReady(jobName string) bool {
	isReady := true
	for _, job := range c.PreviousJobs(jobName) {
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
// A chain is complete if every job in it completed succesfully.
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
		for _, prevJob := range c.PreviousJobs(job.Name) {
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
		return ErrInvalidAdjacencyList
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
		return ErrCyclic
	}

	return nil
}

// CurrentJobData returns a copy of the chains jobData as it exists when this
// method is called.
func (c *chain) CurrentJobData() map[string]string {
	c.RLock() // -- lock
	resp := make(map[string]string)
	for k, v := range c.JobData {
		resp[k] = v
	}
	c.RUnlock() // -- unlock
	return resp
}

// AddJobData adds new data to the chain's jobData. It doesn't care if it
// overwrites existing data.
func (c *chain) AddJobData(newData map[string]string) {
	c.Lock() // -- lock
	for k, v := range newData {
		c.JobData[k] = v
	}
	c.Unlock() // -- unlock
}

// RequestId returns the request id of the job chain.
func (c *chain) RequestId() uint {
	return c.JobChain.RequestId
}

// Allows tests to mock the time.
var now func() time.Time = time.Now

// JobState returns the state of a given job.
func (c *chain) JobState(jobName string) byte {
	c.RLock()         // -- lock
	defer c.RUnlock() // -- unlock
	return c.JobChain.Jobs[jobName].State
}

// Set the state of a job in the chain.
func (c *chain) SetJobState(jobName string, state byte) {
	c.Lock() // -- lock
	j := c.JobChain.Jobs[jobName]
	j.State = state
	c.JobChain.Jobs[jobName] = j
	c.Unlock() // -- unlock
}

// Set the start time of the chain, and set the chain's state to RUNNING.
func (c *chain) SetStart() {
	c.Lock() // -- lock
	c.JobChain.StartTime = now()
	c.JobChain.State = proto.STATE_RUNNING
	c.Unlock() // -- unlock
}

// Set the end time of the chain, and set the chain's state to COMPLETE.
func (c *chain) SetComplete() {
	c.Lock() // -- lock
	c.JobChain.EndTime = now()
	c.JobChain.State = proto.STATE_COMPLETE
	c.Unlock() // -- unlock
}

// Set the end time of the chain, and set the chain's state to INCOMPLETE.
func (c *chain) SetIncomplete() {
	c.Lock() // -- lock
	c.JobChain.EndTime = now()
	c.JobChain.State = proto.STATE_INCOMPLETE
	c.Unlock() // -- unlock
}

// -------------------------------------------------------------------------- //

// indegreeCounts finds the indegree for each job in the chain.
func (c *chain) indegreeCounts() map[string]int {
	indegreeCounts := make(map[string]int)
	for job, _ := range c.JobChain.Jobs {
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
	for job, _ := range c.JobChain.Jobs {
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
		for k, _ := range queue {
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
