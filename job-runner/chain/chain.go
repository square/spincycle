// Copyright 2017-2019, Square, Inc.

// Package chain implements a job chain. It provides the ability to traverse a chain
// and run all of the jobs in it.
package chain

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/square/spincycle/proto"
)

// chain represents a job chain and some meta information about it.
type Chain struct {
	// For access to jobChain.Jobs map. Be careful not to make nested RLock()
	// calls on jobsMux within the same goroutine.
	jobsMux  *sync.RWMutex
	jobChain *proto.JobChain

	runningMux *sync.RWMutex
	running    map[string]proto.JobStatus // keyed on job id
	numJobsRun uint                       // Number of jobs run so far

	triesMux          *sync.RWMutex   // for access to sequence/job tries maps
	sequenceTries     map[string]uint // Number of sequence retries attempted so far
	latestRunJobTries map[string]uint // job.Id -> number of times tried within the latest time it was run (i.e. within the latest sequence try)
	totalJobTries     map[string]uint // job.Id -> total number of times tried
}

// NewChain takes a JobChain proto and maps of sequence + jobs tries, and turns them
// into a Chain that the JR can use.
func NewChain(jc *proto.JobChain, sequenceTries map[string]uint, totalJobTries map[string]uint, latestRunJobTries map[string]uint) *Chain {
	// Make sure all jobs have valid State + Data fields, and count the number of
	// completed + failed jobs (the number of jobs that have finished running).
	numJobsRun := uint(0)
	for jobName, job := range jc.Jobs {
		switch job.State {
		case proto.STATE_PENDING:
			// Pending is a valid job state - do nothing.
		case proto.STATE_STOPPED:
			// Valid state when resuming a suspended chain. Treated the same as
			// pending jobs.
		case proto.STATE_COMPLETE:
			// Valid state, job is done running.
			numJobsRun += 1
		case proto.STATE_FAIL:
			// Valid state, job is done running.
			numJobsRun += 1
		default:
			// Job isn't pending, stopped, failed, or complete. For a new /
			// suspended chain, these are the only valid states (no jobs can be
			// running before the chain is started or resumed). Treat jobs with
			// other states as failed.
			job.State = proto.STATE_FAIL
			numJobsRun += 1
		}

		if job.Data == nil {
			job.Data = map[string]interface{}{}
		}
		jc.Jobs[jobName] = job
	}

	return &Chain{
		jobsMux:           &sync.RWMutex{},
		jobChain:          jc,
		runningMux:        &sync.RWMutex{},
		running:           map[string]proto.JobStatus{},
		numJobsRun:        numJobsRun,
		sequenceTries:     sequenceTries,
		triesMux:          &sync.RWMutex{},
		totalJobTries:     totalJobTries,
		latestRunJobTries: latestRunJobTries,
	}
}

// NextJobs finds all of the jobs adjacent to the given job.
func (c *Chain) NextJobs(jobId string) proto.Jobs {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
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

// IsRunnable returns whether or not a job is runnable. A job is considered
// runnable if it is Pending or Stopped with some retry attempts remaining,
// and all of its previous jobs are complete. If any previous jobs are not
// complete, the job is not runnable.
func (c *Chain) IsRunnable(jobId string) bool {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	return c.isRunnable(jobId)
}

// RunnableJobs returns a list of all jobs that are runnable. A job is
// runnable if all of its previous jobs are complete and it is Pending
// or Stopped with some retries still remaining.
func (c *Chain) RunnableJobs() proto.Jobs {
	var runnableJobs proto.Jobs
	for jobId, job := range c.jobChain.Jobs {
		if !c.IsRunnable(jobId) {
			continue
		}
		runnableJobs = append(runnableJobs, job)
	}
	return runnableJobs
}

// IsDoneRunning returns two booleans - the first indicates whether the chain
// is done, and the second indicates whether the chain is complete.
//
// A chain is done running if there are no jobs in it running and there are no more
// jobs in it that can be run. This happens if all of the jobs in the chain are
// complete, or if some or all of the jobs in the chain failed. Note that one
// failed job does not mean the chain is done - there may still be pending jobs
// independent of this failed job that can be run. Stopped jobs are treated the
// same as pending jobs - they can be rerun (as they are when a suspended chain is
// resumed).
//
// A chain is complete if every job in it completed successfully.
func (c *Chain) IsDoneRunning() (done bool, complete bool) {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()

	complete = true

	// Loop through every job in the chain and act on its state. Keep
	// track of the jobs that aren't running or in a finished state so
	// that we can later check to see if they are capable of running.
	for _, job := range c.jobChain.Jobs {
		switch job.State {
		case proto.STATE_COMPLETE:
			// Move on to the next job.
			continue
		case proto.STATE_RUNNING:
			// If any jobs are still running, the chain isn't done or complete.
			return false, false
		case proto.STATE_PENDING, proto.STATE_STOPPED:
			// If any job can be run, the chain is not done or complete.
			// Treat stopped jobs as pending jobs because they may be retried,
			// as when resuming a suspended job chain.
			if c.isRunnable(job.Id) {
				return false, false
			}
		default:
			// Any job that matches none of the above cases is failed
			if c.canRetrySequence(job.Id) {
				// This failed job is part of a sequence that can be retried.
				return false, false
			}
		}

		// We can only arrive here if a job is not complete (pending or failed).
		// If there is at least one job that is not complete, the whole chain
		// is not complete. The chain could still be done, though, so we aren't
		// ready to return yet.
		complete = false
	}

	return true, complete
}

func (c *Chain) SequenceStartJob(jobId string) proto.Job {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	return c.jobChain.Jobs[c.jobChain.Jobs[jobId].SequenceId]
}

func (c *Chain) IsSequenceStartJob(jobId string) bool {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	return jobId == c.jobChain.Jobs[jobId].SequenceId
}

func (c *Chain) CanRetrySequence(jobId string) bool {
	sequenceStartJob := c.SequenceStartJob(jobId)
	c.triesMux.RLock()
	defer c.triesMux.RUnlock()
	return c.sequenceTries[sequenceStartJob.Id] <= sequenceStartJob.SequenceRetry
}

func (c *Chain) IncrementSequenceTries(jobId string) {
	c.jobsMux.RLock()
	seqId := c.jobChain.Jobs[jobId].SequenceId
	c.jobsMux.RUnlock()
	c.triesMux.Lock()
	c.sequenceTries[seqId] += 1
	c.triesMux.Unlock()
}

func (c *Chain) SequenceTries(jobId string) uint {
	c.jobsMux.RLock()
	seqId := c.jobChain.Jobs[jobId].SequenceId
	c.jobsMux.RUnlock()
	c.triesMux.RLock()
	defer c.triesMux.RUnlock()
	return c.sequenceTries[seqId]
}

func (c *Chain) AddJobTries(jobId string, tries uint) {
	c.triesMux.Lock()
	c.totalJobTries[jobId] += tries
	c.latestRunJobTries[jobId] = tries
	c.triesMux.Unlock()
}

func (c *Chain) SetLatestRunJobTries(jobId string, tries uint) {
	c.triesMux.Lock()
	defer c.triesMux.Unlock()
	c.latestRunJobTries[jobId] = tries
}

func (c *Chain) TotalTries(jobId string) uint {
	c.triesMux.RLock()
	defer c.triesMux.RUnlock()
	return c.totalJobTries[jobId]
}

func (c *Chain) LatestRunTries(jobId string) uint {
	c.triesMux.RLock()
	defer c.triesMux.RUnlock()
	return c.latestRunJobTries[jobId]
}

func (c *Chain) ToSuspended() proto.SuspendedJobChain {
	c.triesMux.RLock()
	seqTries := c.sequenceTries
	totalJobTries := c.totalJobTries
	latestTries := c.latestRunJobTries
	c.triesMux.RUnlock()

	sjc := proto.SuspendedJobChain{
		RequestId:         c.RequestId(),
		JobChain:          c.jobChain,
		TotalJobTries:     totalJobTries,
		LatestRunJobTries: latestTries,
		SequenceTries:     seqTries,
	}
	return sjc
}

// RequestId returns the request id of the job chain.
func (c *Chain) RequestId() string {
	return c.jobChain.RequestId
}

// JobState returns the state of a given job.
func (c *Chain) JobState(jobId string) byte {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	return c.jobChain.Jobs[jobId].State
}

// SetState sets the chain's state.
func (c *Chain) SetState(state byte) {
	c.jobChain.State = state
}

// State returns the chain's state.
func (c *Chain) State() byte {
	return c.jobChain.State
}

// JobChain returns the chain's JobChain.
func (c *Chain) JobChain() *proto.JobChain {
	return c.jobChain
}

// Set the state of a job in the chain.
func (c *Chain) SetJobState(jobId string, state byte) {
	now := time.Now().UnixNano()

	c.jobsMux.Lock() // -- lock
	j := c.jobChain.Jobs[jobId]
	prevState := j.State
	j.State = state
	c.jobChain.Jobs[jobId] = j
	c.jobsMux.Unlock() // -- unlock

	if prevState == state {
		return
	}

	// Keep Chain.running up to date
	c.runningMux.Lock()
	defer c.runningMux.Unlock()
	if state == proto.STATE_RUNNING {
		c.numJobsRun += 1 // Nth job to run

		jobStatus := proto.JobStatus{
			RequestId: c.jobChain.RequestId,
			JobId:     jobId,
			Type:      j.Type,
			Name:      j.Name,
			Args:      map[string]interface{}{},
			StartedAt: now,
			State:     state,
			N:         c.numJobsRun,
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

	if state == proto.STATE_PENDING || state == proto.STATE_STOPPED {
		// Job was stopped or job was previously done but set back to pending
		// (i.e. on sequence retry). Decrement Chain.numJobsRun so that # of jobs
		// run stays correct.
		if c.numJobsRun == 0 {
			// Don't decrement below 0 - n is unsigned
			return
		}
		c.numJobsRun--
	}
}

// Running returns a list of running jobs.
func (c *Chain) Running() map[string]proto.JobStatus {
	// Return copy of c.running
	c.runningMux.RLock()
	defer c.runningMux.RUnlock()
	running := make(map[string]proto.JobStatus, len(c.running))
	for jobId, jobStatus := range c.running {
		running[jobId] = jobStatus
	}
	return running
}

// Length returns the total number of jobs in the chain.
func (c *Chain) Length() int {
	return len(c.jobChain.Jobs)
}

// //////////////////////////////////////////////////////////////////////////
// Implement JSON interfaces for custom (un)marshalling by chain.Repo
// //////////////////////////////////////////////////////////////////////////

type chainJSON struct {
	RequestId         string
	JobChain          *proto.JobChain
	TotalJobTries     map[string]uint
	LatestRunJobTries map[string]uint
	SequenceTries     map[string]uint
	Running           map[string]proto.JobStatus `json:"running"`
	NumJobsRun        uint                       `json:"numJobsRun"`
}

func (c *Chain) MarshalJSON() ([]byte, error) {
	c.runningMux.RLock()
	running := c.running
	numJobsRun := c.numJobsRun
	c.runningMux.RUnlock()

	c.triesMux.RLock()
	seqTries := c.sequenceTries
	totalJobTries := c.totalJobTries
	latestTries := c.latestRunJobTries
	c.triesMux.RUnlock()

	m := chainJSON{
		RequestId:         c.RequestId(),
		JobChain:          c.jobChain,
		TotalJobTries:     totalJobTries,
		LatestRunJobTries: latestTries,
		SequenceTries:     seqTries,
		Running:           running,
		NumJobsRun:        numJobsRun,
	}
	return json.Marshal(m)
}

func (c *Chain) UnmarshalJSON(bytes []byte) error {
	var m chainJSON
	err := json.Unmarshal(bytes, &m)
	if err != nil {
		return err
	}

	c.jobsMux = &sync.RWMutex{}
	c.jobChain = m.JobChain
	c.triesMux = &sync.RWMutex{}
	c.sequenceTries = m.SequenceTries
	c.totalJobTries = m.TotalJobTries
	c.latestRunJobTries = m.LatestRunJobTries
	c.numJobsRun = m.NumJobsRun
	c.runningMux = &sync.RWMutex{}
	c.running = m.Running

	return nil
}

// -------------------------------------------------------------------------- //

// isRunnable returns whether or not a job is runnable. A job is considered
// runnable if it is Pending or Stopped with some retry attempts remaining,
// and all of its previous jobs are complete. If any previous jobs are not
// complete, the job is not runnable.
//
// isRunnable doesn't lock jobsMux, so it's only safe to call if you've already
// locked that mutex. Call it instead of IsRunnable within other Chain methods that
// lock jobsMux to avoid recursive locks.
func (c *Chain) isRunnable(jobId string) bool {
	job := c.jobChain.Jobs[jobId]
	switch job.State {
	case proto.STATE_PENDING:
		// Pending job may be runnable.
	case proto.STATE_STOPPED:
		// Stopped job may be runnable if it has retries remaining.
		triesDone := c.LatestRunTries(jobId)
		if triesDone != 0 {
			// If not already 0, subtract 1 because we don't count the try the job
			// was stopped on.
			triesDone--
		}
		if triesDone > job.Retry {
			// no retries remaining
			return false
		}
	default:
		// Job isn't pending or stopped - not runnable.
		return false
	}

	// Check that all previous jobs are complete.
	for _, job := range c.previousJobs(jobId) {
		if job.State != proto.STATE_COMPLETE {
			return false
		}
	}
	return true
}

// Just like CanRetrySequence but without read locking jobsMux. Used within methods
// that already read lock the jobsMux to avoid nested read locks.
func (c *Chain) canRetrySequence(jobId string) bool {
	sequenceStartJob := c.sequenceStartJob(jobId)
	c.triesMux.RLock()
	defer c.triesMux.RUnlock()
	return c.sequenceTries[sequenceStartJob.Id] <= sequenceStartJob.SequenceRetry
}

// Just like SequenceStartJob but without read locking jobsMux. Used within methods
// that already read lock the jobsMux to avoid nested read locks.
func (c *Chain) sequenceStartJob(jobId string) proto.Job {
	return c.jobChain.Jobs[c.jobChain.Jobs[jobId].SequenceId]
}

// previousJobs finds all of the immediately previous jobs to a given job.
func (c *Chain) previousJobs(jobId string) proto.Jobs {
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

// contains returns whether or not a slice of strings contains a specific string.
func contains(s []string, t string) bool {
	for _, i := range s {
		if i == t {
			return true
		}
	}
	return false
}
