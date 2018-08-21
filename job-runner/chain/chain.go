// Copyright 2017-2018, Square, Inc.

// Package chain implements a job chain. It provides the ability to traverse a chain
// and run all of the jobs in it.
package chain

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/square/spincycle/proto"
)

// chain represents a job chain and some meta information about it.
type chain struct {
	jcMux    *sync.RWMutex
	jobChain *proto.JobChain

	runningMux         *sync.RWMutex
	running            map[string]proto.JobStatus // keyed on job id
	n                  uint                       // N jobs ran
	sequenceRetryCount map[string]uint            // Number of retries attempted so far
}

// NewChain takes a JobChain proto (from the RM) and turns it into a Chain that
// the JR can use.
func NewChain(jc *proto.JobChain) *chain {
	sequenceRetryCount := make(map[string]uint)
	for jobName, job := range jc.Jobs {
		job.State = proto.STATE_PENDING
		job.Data = map[string]interface{}{}
		jc.Jobs[jobName] = job
	}

	return &chain{
		jcMux:    &sync.RWMutex{},
		jobChain: jc,

		runningMux:         &sync.RWMutex{},
		running:            map[string]proto.JobStatus{},
		n:                  0,
		sequenceRetryCount: sequenceRetryCount,
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

	return c.jobChain.Jobs[jobIds[0]], nil
}

func (c *chain) SequenceStartJob(j proto.Job) proto.Job {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return c.jobChain.Jobs[j.SequenceId]
}

// NextJobs finds all of the jobs adjacent to the given job.
func (c *chain) NextJobs(jobId string) proto.Jobs {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	var nextJobs proto.Jobs
	if nextJobIds, ok := c.jobChain.AdjacencyList[jobId]; ok {
		for _, id := range nextJobIds {
			if val, ok := c.jobChain.Jobs[id]; ok {
				nextJobs = append(nextJobs, val)
			}
		}
	}

	return nextJobs
}

// JobIsReady returns whether or not a job is ready to run. A job is considered
// ready to run if all of its previous jobs are complete. If any previous jobs
// are not complete, the job is not ready to run.
func (c *chain) JobIsReady(jobId string) bool {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	for _, job := range c.previousJobs(jobId) {
		if job.State != proto.STATE_COMPLETE {
			return false
		}
	}
	return true
}

// IsDone returns two booleans - the first one indicates whether or not the
// chain is done, and the second one indicates whether or not the chain is
// complete.
//
// A chain is done running if there are no more jobs in it that can run. This
// can happen if all of the jobs in the chain are complete, or if some or all
// of the jobs in the chain failed.
//
// A chain is complete if every job in it completed successfully.
func (c *chain) IsDone() (done bool, complete bool) {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()

	done = true
	complete = true
	pendingJobs := proto.Jobs{}

	// If any jobs still running (even if others are failed/stopped), not done.
	for _, job := range c.jobChain.Jobs {
		if job.State == proto.STATE_RUNNING {
			return false, false
		}
	}

	// Loop through every job in the chain and act on its state. Keep
	// track of the jobs that aren't running or in a finished state so
	// that we can later check to see if they are capable of running.
LOOP:
	for _, job := range c.jobChain.Jobs {
		switch job.State {
		// case proto.STATE_RUNNING:
		// 	// If any jobs are running, the chain can't be done
		// 	// or complete, so return false for both now.
		// 	return false, false
		case proto.STATE_COMPLETE:
			// Move on to the next job.
			continue LOOP
		case proto.STATE_FAIL:
			// This failed job is part of a sequence that can be retried
			if c.CanRetrySequence(job) {
				return false, false
			}
		case proto.STATE_STOPPED:
			return true, false
			// Like STATE_FAIL, but ignore sequence retries.
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
		for _, prevJob := range c.previousJobs(job.Id) {
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

func (c *chain) CanRetrySequence(j proto.Job) bool {
	sequenceStartJob := c.SequenceStartJob(j)
	return c.sequenceRetryCount[sequenceStartJob.Id] < sequenceStartJob.SequenceRetry
}

func (c *chain) IncrementSequenceRetryCount(j proto.Job) {
	sequenceStartJob := c.SequenceStartJob(j)
	c.sequenceRetryCount[sequenceStartJob.Id] += 1
}

func (c *chain) SequenceRetryCount(j proto.Job) uint {
	sequenceStartJob := c.SequenceStartJob(j)
	return c.sequenceRetryCount[sequenceStartJob.Id]
}

func (c *chain) SequenceRetryCounts() map[string]uint {
	return c.sequenceRetryCount
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
	if _, err := c.FirstJob(); err != nil {
		return err
	}

	// Make sure there is one last job.
	if _, err := c.lastJob(); err != nil {
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
	return c.jobChain.RequestId
}

// JobState returns the state of a given job.
func (c *chain) JobState(jobId string) byte {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return c.jobChain.Jobs[jobId].State
}

// SetState sets the chain's state.
func (c *chain) SetState(state byte) {
	c.jcMux.Lock() // -- lock
	c.jobChain.State = state
	c.jcMux.Unlock() // -- unlock
}

// State returns the chain's state.
func (c *chain) State() byte {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return c.jobChain.State
}

// State returns the chain's state.
func (c *chain) JobChain() *proto.JobChain {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return c.jobChain
}

// Set the state of a job in the chain.
func (c *chain) SetJobState(jobId string, state byte) {
	now := time.Now().UnixNano()

	c.jcMux.Lock() // -- lock
	j := c.jobChain.Jobs[jobId]
	j.State = state
	c.jobChain.Jobs[jobId] = j
	c.jcMux.Unlock() // -- unlock

	// Keep chain.running up to date
	c.runningMux.Lock()
	defer c.runningMux.Unlock()
	if state == proto.STATE_RUNNING {
		c.n += 1 // Nth job to run

		jobStatus := proto.JobStatus{
			RequestId: c.jobChain.RequestId,
			JobId:     jobId,
			Type:      j.Type,
			Name:      j.Name,
			Args:      map[string]interface{}{},
			StartedAt: now,
			State:     state,
			N:         c.n,
		}
		for k, v := range j.Args {
			jobStatus.Args[k] = v
		}
		c.running[jobId] = jobStatus
	} else {
		// STATE_RUNNING is the only running state, and it's not that, so the
		// job must not be running.
		delete(c.running, jobId)
	}
}

// Running returns a list of running jobs.
func (c *chain) Running() map[string]proto.JobStatus {
	// Return copy of c.running
	c.runningMux.Lock()
	defer c.runningMux.Unlock()
	running := make(map[string]proto.JobStatus, len(c.running))
	for jobId, jobStatus := range c.running {
		running[jobId] = jobStatus
	}
	return running
}

// //////////////////////////////////////////////////////////////////////////
// Implement JOSN interfaces for custom (un)marshalling by chain.Repo
// //////////////////////////////////////////////////////////////////////////

type chainJSON struct {
	JobChain *proto.JobChain            `json:"jobChain"`
	Running  map[string]proto.JobStatus `json:"running"`
	N        uint                       `json:"n"`
}

func (c *chain) MarshalJSON() ([]byte, error) {
	c.jcMux.Lock()
	defer c.jcMux.Unlock()

	c.runningMux.Lock()
	defer c.runningMux.Unlock()

	m := chainJSON{
		JobChain: c.jobChain,
		Running:  c.running,
		N:        c.n,
	}
	return json.Marshal(m)
}

func (c *chain) UnmarshalJSON(bytes []byte) error {
	var m chainJSON
	err := json.Unmarshal(bytes, &m)
	if err != nil {
		return err
	}

	c.jcMux = &sync.RWMutex{}
	c.jobChain = m.JobChain

	c.runningMux = &sync.RWMutex{}
	c.running = m.Running
	c.n = m.N

	return nil
}

// -------------------------------------------------------------------------- //

// previousJobs finds all of the immediately previous jobs to a given job.
func (c *chain) previousJobs(jobId string) proto.Jobs {
	var prevJobs proto.Jobs
	for curJob, nextJobs := range c.jobChain.AdjacencyList {
		if contains(nextJobs, jobId) {
			if val, ok := c.jobChain.Jobs[curJob]; ok {
				prevJobs = append(prevJobs, val)
			}
		}
	}
	return prevJobs
}

// lastJob finds the job in the chain with outdegree 0. If there is not
// exactly one of these jobs, it returns an error.
func (c *chain) lastJob() (proto.Job, error) {
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

	return c.jobChain.Jobs[jobIds[0]], nil
}

// indegreeCounts finds the indegree for each job in the chain.
func (c *chain) indegreeCounts() map[string]int {
	indegreeCounts := make(map[string]int)
	for job := range c.jobChain.Jobs {
		indegreeCounts[job] = 0
	}

	for _, nextJobs := range c.jobChain.AdjacencyList {
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
	for job := range c.jobChain.Jobs {
		outdegreeCounts[job] = len(c.jobChain.AdjacencyList[job])
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
		for _, adjJob := range c.jobChain.AdjacencyList[curJob] {
			indegreeCounts[adjJob] -= 1
			if indegreeCounts[adjJob] == 0 {
				queue[adjJob] = struct{}{}
			}
		}

		// Keep track of the number of jobs we've visited. If there is
		// a cycle in the chain, we won't end up visiting some jobs.
		jobsVisited += 1
	}

	if jobsVisited != len(c.jobChain.Jobs) {
		return false
	}

	return true
}

// adjacencyListIsValid returns whether or not the chain's adjacency list is
// not valid. An adjacency list is not valid if any of the jobs in it do not
// exist in chain.Jobs.
func (c *chain) adjacencyListIsValid() bool {
	for job, adjJobs := range c.jobChain.AdjacencyList {
		if _, ok := c.jobChain.Jobs[job]; !ok {
			return false
		}

		for _, adjJob := range adjJobs {
			if _, ok := c.jobChain.Jobs[adjJob]; !ok {
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
