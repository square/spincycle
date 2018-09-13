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
	// A note on the read/write mutexes in chain:
	//   A lot of the methods in chain call each other - be careful not to make
	//   nested RLock calls within one goroutine, or a Lock call from another
	//   goroutine can cause a deadlock (and it will be very annoying to debug).
	jcMux    *sync.RWMutex
	jobChain *proto.JobChain

	runningMux *sync.RWMutex
	running    map[string]proto.JobStatus // keyed on job id
	n          uint                       // Number of jobs run so far

	seqTriesMux   *sync.RWMutex
	sequenceTries map[string]uint // Number of sequence retries attempted so far

	latestTriesMux *sync.RWMutex
	latestRunTries map[string]uint // job.Id -> number of times tried within this sequence try
	totalTriesMux  *sync.RWMutex
	totalTries     map[string]uint // job.Id -> total number of times tried
}

// NewChain takes a JobChain proto (from the RM) and turns it into a Chain that
// the JR can use.
func NewChain(jc *proto.JobChain) *chain {
	for jobName, job := range jc.Jobs {
		job.State = proto.STATE_PENDING
		job.Data = map[string]interface{}{}
		jc.Jobs[jobName] = job
	}

	return &chain{
		jcMux:    &sync.RWMutex{},
		jobChain: jc,

		runningMux:     &sync.RWMutex{},
		running:        map[string]proto.JobStatus{},
		n:              0,
		sequenceTries:  make(map[string]uint),
		seqTriesMux:    &sync.RWMutex{},
		totalTries:     make(map[string]uint),
		totalTriesMux:  &sync.RWMutex{},
		latestRunTries: make(map[string]uint),
		latestTriesMux: &sync.RWMutex{},
	}
}

// ResumeChain takes a SuspendedJobChain proto (from the RM) and turns it into a
// Chain that the JR can use.
func ResumeChain(sjc *proto.SuspendedJobChain) *chain {
	// make sure all jobs have valid State + Data fields
	for jobName, job := range sjc.JobChain.Jobs {
		if job.State == proto.STATE_UNKNOWN {
			job.State = proto.STATE_PENDING
		}
		if job.Data == nil {
			job.Data = map[string]interface{}{}
		}
		sjc.JobChain.Jobs[jobName] = job
	}

	return &chain{
		jcMux:          &sync.RWMutex{},
		jobChain:       sjc.JobChain,
		runningMux:     &sync.RWMutex{},
		running:        map[string]proto.JobStatus{},
		n:              sjc.NumJobsRun,
		sequenceTries:  sjc.SequenceTries,
		seqTriesMux:    &sync.RWMutex{},
		totalTries:     sjc.TotalJobTries,
		totalTriesMux:  &sync.RWMutex{},
		latestRunTries: sjc.LastRunJobTries,
		latestTriesMux: &sync.RWMutex{},
	}
}

// ErrInvalidChain is the error returned when a chain is not valid.
type ErrInvalidChain struct {
	Message string
}

func (e ErrInvalidChain) Error() string {
	return e.Message
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
// ready to run if it is Pending or Stopped with some retry attempts remaining,
// and all of its previous jobs are complete. If any previous jobs are not
// complete, the job is not ready to run.
func (c *chain) JobIsReady(jobId string) bool {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return c.jobIsReady(jobId)
}

// RunnableJobs returns a list of all jobs that are ready to run. A job is
// ready to run if all of its previous jobs are complete and it is Pending
// or Stopped with some retries still remaining.
func (c *chain) RunnableJobs() proto.Jobs {
	runnableJobs := []proto.Job{}

	// Loop through every job in the chain and check if it's ready to run. Only
	// Pending + Stopped jobs may be ready to run.
	for jobId, job := range c.jobChain.Jobs {
		if !c.JobIsReady(jobId) {
			continue
		}

		runnableJobs = append(runnableJobs, job)
	}

	return runnableJobs
}

// IsDoneRunning returns two booleans - the first one indicates whether or not the
// chain is done, and the second one indicates whether or not the chain is
// complete.
//
// A chain is done running if there are no more jobs in it that can run or
// are running. This can happen if all of the jobs in the chain are complete,
// or if some or all of the jobs in the chain failed. Note that one failed job
// does not mean the chain is done - there may still be pending jobs independent
// of this failed job that can be run. Stopped jobs are treated the same as pending
// jobs - they can be rerun (as they are when a suspended chain is resumed).
//
// A chain is complete if every job in it completed successfully.
func (c *chain) IsDoneRunning() (done bool, complete bool) {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()

	done = true
	complete = true
	pendingJobs := proto.Jobs{}

	// If any jobs are still running, the chain isn't done or complete.
	for _, job := range c.jobChain.Jobs {
		if job.State == proto.STATE_RUNNING {
			return false, false
		}
	}

	// Loop through every job in the chain and act on its state. Keep
	// track of the jobs that aren't running or in a finished state so
	// that we can later check to see if they are capable of running.
	for _, job := range c.jobChain.Jobs {
		switch job.State {
		case proto.STATE_RUNNING:
			// We should never reach this case because we previously check for running jobs.
			return false, false
		case proto.STATE_COMPLETE:
			// Move on to the next job.
			continue
		case proto.STATE_PENDING, proto.STATE_STOPPED:
			// Treat stopped jobs as pending jobs because they may be retried
			// (as in a resumed job chain)
			pendingJobs = append(pendingJobs, job)
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

	// For each pending job, check to see if it's ready to be run.
	for _, job := range pendingJobs {
		if c.jobIsReady(job.Id) {
			// If a job can be run, the chain can't be done.
			done = false
			break
		}
	}
	return done, complete
}

func (c *chain) SequenceStartJob(jobId string) proto.Job {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return c.jobChain.Jobs[c.jobChain.Jobs[jobId].SequenceId]
}

func (c *chain) IsSequenceStartJob(jobId string) bool {
	c.jcMux.RLock()
	defer c.jcMux.RUnlock()
	return (jobId == c.jobChain.Jobs[jobId].SequenceId)
}

func (c *chain) CanRetrySequence(jobId string) bool {
	sequenceStartJob := c.SequenceStartJob(jobId)
	c.seqTriesMux.RLock()
	defer c.seqTriesMux.RUnlock()
	return c.sequenceTries[sequenceStartJob.Id] <= sequenceStartJob.SequenceRetry
}

func (c *chain) IncrementSequenceTries(jobId string) {
	c.jcMux.RLock()
	seqId := c.jobChain.Jobs[jobId].SequenceId
	c.jcMux.RUnlock()
	c.seqTriesMux.Lock()
	c.sequenceTries[seqId] += 1
	c.seqTriesMux.Unlock()
}

func (c *chain) SequenceTries(jobId string) uint {
	c.jcMux.RLock()
	seqId := c.jobChain.Jobs[jobId].SequenceId
	c.jcMux.RUnlock()
	c.seqTriesMux.Lock()
	defer c.seqTriesMux.Unlock()
	return c.sequenceTries[seqId]
}

func (c *chain) AddJobTries(jobId string, tries uint) {
	c.totalTriesMux.Lock()
	c.totalTries[jobId] += tries
	c.totalTriesMux.Unlock()

	c.latestTriesMux.Lock()
	c.latestRunTries[jobId] = tries
	c.latestTriesMux.Unlock()
}

func (c *chain) AddLatestJobTries(jobId string, tries uint) {
	c.latestTriesMux.Lock()
	c.latestRunTries[jobId] = tries
	c.latestTriesMux.Unlock()
}

func (c *chain) TotalTries(jobId string) uint {
	c.totalTriesMux.RLock()
	defer c.totalTriesMux.RUnlock()
	return c.totalTries[jobId]
}

func (c *chain) LatestRunTries(jobId string) uint {
	c.latestTriesMux.RLock()
	defer c.latestTriesMux.RUnlock()
	return c.latestRunTries[jobId]
}

func (c *chain) ToSuspended() proto.SuspendedJobChain {
	c.jcMux.RLock()
	jc := c.jobChain
	c.jcMux.RUnlock()

	c.seqTriesMux.RLock()
	seqTries := c.sequenceTries
	c.seqTriesMux.RUnlock()

	c.totalTriesMux.RLock()
	totalTries := c.totalTries
	c.totalTriesMux.RUnlock()

	c.latestTriesMux.RLock()
	latestTries := c.latestRunTries
	c.latestTriesMux.RUnlock()

	c.runningMux.RLock()
	n := c.n
	c.runningMux.RUnlock()

	sjc := proto.SuspendedJobChain{
		RequestId:       c.RequestId(),
		JobChain:        jc,
		TotalJobTries:   totalTries,
		LastRunJobTries: latestTries,
		SequenceTries:   seqTries,
		NumJobsRun:      n,
	}
	return sjc
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

// JobChain returns the chain's JobChain.
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
	prevState := j.State
	j.State = state
	c.jobChain.Jobs[jobId] = j
	c.jcMux.Unlock() // -- unlock

	if prevState == state {
		return
	}

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

	if state == proto.STATE_PENDING || state == proto.STATE_STOPPED {
		// Job was stopped or job was previously done but set back to pending
		// (i.e. on sequence retry). Decrement chain.n so that # of jobs run
		// stays correct.
		if c.n == 0 {
			// Don't decrement below 0 - n is unsigned
			return
		}
		c.n--
	}
}

// Running returns a list of running jobs.
func (c *chain) Running() map[string]proto.JobStatus {
	// Return copy of c.running
	c.runningMux.RLock()
	defer c.runningMux.RUnlock()
	running := make(map[string]proto.JobStatus, len(c.running))
	for jobId, jobStatus := range c.running {
		running[jobId] = jobStatus
	}
	return running
}

// //////////////////////////////////////////////////////////////////////////
// Implement JSON interfaces for custom (un)marshalling by chain.Repo
// //////////////////////////////////////////////////////////////////////////

type chainJSON struct {
	// An SJC has already fields for most of the info we need to store.
	proto.SuspendedJobChain
	Running map[string]proto.JobStatus `json:"running"`
}

func (c *chain) MarshalJSON() ([]byte, error) {
	c.runningMux.RLock()
	running := c.running
	c.runningMux.RUnlock()

	m := chainJSON{
		SuspendedJobChain: c.ToSuspended(),
		Running:           running,
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
	c.seqTriesMux = &sync.RWMutex{}
	c.sequenceTries = m.SequenceTries
	c.totalTriesMux = &sync.RWMutex{}
	c.totalTries = m.TotalJobTries
	c.latestTriesMux = &sync.RWMutex{}
	c.latestRunTries = m.LastRunJobTries
	c.n = m.NumJobsRun
	c.runningMux = &sync.RWMutex{}
	c.running = m.Running

	return nil
}

// -------------------------------------------------------------------------- //

// jobIsReady returns whether or not a job is ready to run. A job is considered
// ready to run if it is Pending or Stopped with some retry attempts remaining,
// and all of its previous jobs are complete. If any previous jobs are not
// complete, the job is not ready to run.
// Doesn't lock jcMux, so suitable to call within other chain methods lock jcMux.
func (c *chain) jobIsReady(jobId string) bool {
	// Only Pending or Stopped jobs with available retries can be ready to run.
	job := c.jobChain.Jobs[jobId]
	switch job.State {
	case proto.STATE_PENDING:
	case proto.STATE_STOPPED:
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

// Just like CanRetrySequence but without read locking jcMux. Used within methods
// that already read lock the jcMux to avoid nested read locks.
func (c *chain) canRetrySequence(jobId string) bool {
	sequenceStartJob := c.sequenceStartJob(jobId)
	c.seqTriesMux.RLock()
	defer c.seqTriesMux.RUnlock()
	return c.sequenceTries[sequenceStartJob.Id] <= sequenceStartJob.SequenceRetry
}

// Just like SequenceStartJob but without read locking jcMux. Used within methods
// that already read lock the jcMux to avoid nested read locks.
func (c *chain) sequenceStartJob(jobId string) proto.Job {
	return c.jobChain.Jobs[c.jobChain.Jobs[jobId].SequenceId]
}

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
