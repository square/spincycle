// Copyright 2017-2019, Square, Inc.

// Package chain implements a job chain. It provides the ability to traverse a chain
// and run all of the jobs in it.
package chain

import (
	"fmt"
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

	triesMux          *sync.RWMutex   // for access to sequence/job tries maps
	sequenceTries     map[string]uint // Number of sequence retries attempted so far
	latestRunJobTries map[string]uint // job.Id -> number of times tried for current sequence try
	totalJobTries     map[string]uint // job.Id -> total number of times tried
}

// NewChain takes a JobChain proto and maps of sequence + jobs tries, and turns them
// into a Chain that the JR can use.
func NewChain(jc *proto.JobChain, sequenceTries map[string]uint, totalJobTries map[string]uint, latestRunJobTries map[string]uint) *Chain {
	for jobName, job := range jc.Jobs {
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

// IsDoneRunning returns two booleans: done indicates if there are running or
// runnable jobs, and complete indicates if all jobs finished successfully
// (STATE_COMPLETE).
//
// A chain is done running if there are no running or runnable jobs. We must
// wait for running jobs to reap them. This happens if all jobs in the chain
// complete, or some fail but can be retried in a sequence retry. (Job retries
// are done in the job runner.) Failed jobs do _not_ mean the chain is done
// because there can still be running jobs. This also means a failed job does
// not fail the whole chain immediately; rather, the chain fails when it reaches
// one or more pending jobs that can't run because of the previous failure.
//
// A chain is complete iff every job finished successfully (STATE_COMPLETE).
func (c *Chain) IsDoneRunning() (done bool, complete bool) {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	complete = true
	for _, job := range c.jobChain.Jobs {
		switch job.State {
		case proto.STATE_COMPLETE:
			// Move on to the next job.
			continue
		case proto.STATE_RUNNING:
			// If any jobs are still running, the chain isn't done or complete.
			return false, false
		case proto.STATE_STOPPED:
			// Stopped jobs are a special case: the chain _will be_ done and
			// not complete, but not yet because there can be other running
			// or runnable jobs. So do nothing here and just fall through to
			// set complete = false below.
		case proto.STATE_PENDING:
			// If any job is runnable, the chain isn't done or complete.
			if c.isRunnable(job.Id) {
				return false, false
			}
			// This job is pending but not runnable which means a previous job
			// failed.
		case proto.STATE_FAIL:
			// If sequence can retry, then chain isn't done or complete,
			if c.canRetrySequence(job.Id) {
				return false, false
			}
			// Failed but no seq retry means the chain has failed
		default:
			panic("IsDoneRunning: invalid job state: " + proto.StateName[job.State])
		}

		// We can only arrive here if a job is not complete (pending or failed).
		// If there is at least one job that is not complete, the whole chain
		// is not complete. The chain could still be done, though, so we aren't
		// ready to return yet.
		complete = false
	}
	return true, complete
}

// FailedJobs returns the number of failed jobs. This is used by reapers to
// determine if a chain failed, or if it can be finalized as stopped or suspended.
func (c *Chain) FailedJobs() uint {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	n := uint(0)
	for _, job := range c.jobChain.Jobs {
		if job.State != proto.STATE_FAIL {
			continue
		}
		n++
	}
	return n
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

func (c *Chain) IncrementJobTries(jobId string, delta int) {
	c.triesMux.Lock()
	if delta > 0 {
		// Total job tries can only increase. This is the job try count
		// that's monotonically increasing across all sequence retries.
		c.totalJobTries[jobId] += uint(delta)
	}
	// Job count wrt current sequnce try can reset to zero
	cur := int(c.latestRunJobTries[jobId])
	if cur+delta < 0 { // shouldn't happen
		panic(fmt.Sprintf("IncrementJobTries jobId %s: cur %d + delta %d < 0", jobId, cur, delta))
	}
	c.latestRunJobTries[jobId] = uint(cur + delta)
	c.triesMux.Unlock()
}

func (c *Chain) JobTries(jobId string) (cur uint, total uint) {
	c.triesMux.RLock()
	defer c.triesMux.RUnlock()
	return c.latestRunJobTries[jobId], c.totalJobTries[jobId]
}

func (c *Chain) IncrementSequenceTries(jobId string, delta int) {
	c.jobsMux.RLock()
	seqId := c.jobChain.Jobs[jobId].SequenceId
	c.jobsMux.RUnlock()
	c.triesMux.Lock()
	cur := int(c.sequenceTries[seqId])
	c.sequenceTries[seqId] = uint(cur + delta)
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

// IncrementFinishedJobs increments the finished jobs count by delta. Negative delta
// is given on sequence retry. Returns the new finished jobs count.
func (c *Chain) IncrementFinishedJobs(delta int) {
	c.jobsMux.Lock()
	defer c.jobsMux.Unlock()
	// delta can be negative (on seq retry), but FinishedJobs is unsigned,
	// so get int of FinishedJobs to add int delta, then set back and return.
	cur := int(c.jobChain.FinishedJobs)
	if cur+delta < 0 { // shouldn't happen
		panic(fmt.Sprintf("IncrementFinishedJobs cur %d + delta %d < 0", cur, delta))
	}
	c.jobChain.FinishedJobs = uint(cur + delta)
	return
}

func (c *Chain) FinishedJobs() uint {
	c.jobsMux.RLock()
	defer c.jobsMux.RUnlock()
	return c.jobChain.FinishedJobs
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
		jobStatus := proto.JobStatus{
			RequestId: c.jobChain.RequestId,
			JobId:     jobId,
			Type:      j.Type,
			Name:      j.Name,
			Args:      map[string]interface{}{},
			StartedAt: now,
			State:     state,
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
	case proto.STATE_PENDING, proto.STATE_STOPPED:
		// Runnable (or re-runnable) states
	default:
		return false // not runnable
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
