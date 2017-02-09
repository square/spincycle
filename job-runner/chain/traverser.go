// Copyright 2017, Square, Inc.

package chain

import (
	"sync"
	"time"

	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"

	"github.com/golang/glog"
)

type Traverser interface {
	Run() error
	Stop()
	Status() proto.JobChainStatus
}

// A traverser represents a job chain and everything needed to traverse it.
type traverser struct {
	// The job chain that will be traversed.
	chain *Chain

	// Protection for the chain.
	chainMutex *sync.RWMutex

	// Repo for acessing/updating chains.
	chainRepo Repo

	// jobData that will be passed between jobs in the chain.
	jobData map[string]string

	// A map of active Runners, keyed on job name.
	activeR map[string]runner.Runner

	// Protection for activeR.
	activeRMutex *sync.RWMutex

	// Factory for creating Runners.
	rf runner.RunnerFactory

	// Used as a job queue.
	jobChan chan *SerializedJob

	// Used to stop all running jobs in a chain.
	stopChan chan struct{}
}

// NewTraverser creates a new traverser for a given job chain.
func NewTraverser(repo Repo, rf runner.RunnerFactory, chain *Chain) *traverser {
	return &traverser{
		chain:        chain,
		chainMutex:   &sync.RWMutex{},
		chainRepo:    repo,
		jobData:      make(map[string]string),
		activeR:      make(map[string]runner.Runner),
		activeRMutex: &sync.RWMutex{},
		rf:           rf,
		jobChan:      make(chan *SerializedJob),
		stopChan:     make(chan struct{}),
	}
}

// Run starts traversing a job chain. It returns if it cannot begin,
// or when it finishes traversing an entire chain.
func (t *traverser) Run() error {
	glog.Infof("[chain=%d]: Starting the chain traverser.", t.chain.RequestId)
	rootJob, err := t.chain.RootJob()
	if err != nil {
		return err
	}

	// Set the chain state to "running".
	t.chainMutex.Lock()
	t.chain.Started = time.Now()
	t.chain.State = proto.STATE_RUNNING
	t.chainRepo.Set(t.chain)
	t.chainMutex.Unlock()

	// Start processing jobs.
	doneChan := make(chan struct{})
	go func() {
		process := true
		for job := range t.jobChan {
			select {
			case <-t.stopChan:
				// Stop processing jobs.
				process = false
			default:
			}

			if process {
				go t.processJob(job)
			}
		}
		close(doneChan)
	}()

	// Add the root job of the chain to the queue.
	t.jobChan <- rootJob

	<-doneChan
	return nil
}

// Stop makes a traverser stop traversing its job chain. It also sends a stop
// signal to all of the jobs that a traverser is running, but does not wait for
// confirmation that the jobs actually stopped.
//
// It is the responsibility of the last job that is stopped to set the state
// of the chain to "incomplete" and record the time.
func (t *traverser) Stop() {
	glog.Infof("[chain=%d]: Stopping the chain traverser and all jobs.", t.chain.RequestId)
	close(t.stopChan)

	t.activeRMutex.Lock()         // LOCK
	defer t.activeRMutex.Unlock() // UNLOCK
	glog.Infof("[chain=%d]: Stopping the chain traverser and all jobs.", t.chain.RequestId)
	for jobName, r := range t.activeR {
		r.Stop()
		delete(t.activeR, jobName)
	}
}

// Status returns the status and state of all of the jobs that a traverser is
// running, as well as the state of the entire chain.
func (t *traverser) Status() proto.JobChainStatus {
	glog.Infof("[chain=%d]: Getting the status of all running jobs.", t.chain.RequestId)
	var statusWg sync.WaitGroup
	jobStatuses := proto.JobStatuses{}
	activeStatuses := make(map[string]string)
	asMutex := &sync.Mutex{} // Protection for activeStatuses.

	// ---------------------------------------------------------------
	// Ask each active Runner in the traverser for its status.
	//
	// Since we do not hold the activeRMutex write lock while we wait for
	// each job to report its status, it is possible for a Runner to be
	// removed from the activeR map while we are querying its status. This
	// is acceptable because a Runner can still report its status after
	// being removed from this map. The alternative would be to hold the
	// lock until we get a response back from every active Runner, but this
	// would block Runners from starting and completing.
	// ---------------------------------------------------------------
	t.activeRMutex.RLock()
	for jobName, r := range t.activeR {
		statusWg.Add(1)
		go func(jobName string, jr runner.Runner) {
			defer statusWg.Done()
			asMutex.Lock()
			activeStatuses[jobName] = jr.Status()
			asMutex.Unlock()
		}(jobName, r)
	}
	t.activeRMutex.RUnlock()
	// Wait to get responses from all active Runners.
	statusWg.Wait()

	// Get the state (not the status) of all jobs in the chain, as well as
	// the state of the chain itself. If a job has a status (is actively
	// running), get that too.
	t.chainMutex.RLock()
	for _, job := range t.chain.Jobs {
		var status string
		val, ok := activeStatuses[job.Name]
		if ok {
			status = val
		}
		jobStatuses = append(jobStatuses, proto.JobStatus{job.Name, status, job.State})
	}
	chainState := t.chain.State
	t.chainMutex.RUnlock()

	return proto.JobChainStatus{t.chain.RequestId, chainState, jobStatuses}
}

// processJob creates a new Runner for a job, runs it, and then updates the
// chain and enqueues the next jobs.
func (t *traverser) processJob(serialized *SerializedJob) {
	jobName := serialized.Name
	var finalState byte
	// Call finalizeJob when processJob is done.
	defer func() { t.finalizeJob(jobName, finalState) }()

	// Create a job Runner.
	jr, err := t.rf.Make(serialized.Type, serialized.Name, serialized.Bytes, t.chain.RequestId)
	if err != nil {
		glog.Errorf("[chain=%d,job=%s]: Error creating runner (error: %s).", t.chain.RequestId, jobName, err)
		finalState = proto.STATE_FAIL
		return
	}

	// Add to activeR.
	t.addRunner(jobName, jr)

	// Set the state of the job in the chain to "running".
	t.chainMutex.Lock() // LOCK
	t.setJobState(jobName, proto.STATE_RUNNING)
	t.chainMutex.Unlock() // UNLOCK

	// Run the job. This is a blocking operation that could take a long time.
	completed := jr.Run(t.jobData)
	if completed {
		finalState = proto.STATE_COMPLETE
	} else {
		finalState = proto.STATE_FAIL
	}

	// Remove from activeR.
	t.removeRunner(jobName)
}

// finalizeJob sets the final state of a job in the chain, and, if it completed
// successfully, enqueues the next jobs. It also sets the final state of the
// entire chain if this is the last job that can run in it.
//
// The chain mutex is locked for all of this so that, 1) we prevent the
// possibility of accidentally enqueuing a job more than once (from different
// goroutines), and 2) we can accurately determine if this is the last job
// that can run in the chain.
func (t *traverser) finalizeJob(jobName string, finalState byte) {
	t.chainMutex.Lock()         // LOCK
	defer t.chainMutex.Unlock() // defer UNLOCK

	// Set the state of the job in the chain.
	t.setJobState(jobName, finalState)

	// If the job completed successfully, enqueue its next jobs.
	if finalState == proto.STATE_COMPLETE {
		t.enqueueNextJobs(jobName)
	} else {
		glog.Infof("[chain=%d,job=%s]: Job did not complete successfully, so not "+
			"enqueuing its next jobs.", t.chain.RequestId, jobName)
	}

	// Check to see if the job chain is now complete (there are no more
	// jobs to run, all jobs in the chain are complete) or incomplete
	// (there are no more jobs to run, but some jobs failed).
	if finalState == proto.STATE_COMPLETE && t.chain.Complete() {
		glog.Infof("[chain=%d]: Chain finished running, all jobs completed.", t.chain.RequestId)
		t.chain.Finished = time.Now()
		t.chain.State = proto.STATE_COMPLETE
		t.chainRepo.Set(t.chain)

		// Stop the traverser from processing more jobs.
		close(t.jobChan)
	} else if t.chain.Incomplete() {
		glog.Infof("[chain=%d]: Chain finished running, some jobs failed.", t.chain.RequestId)
		t.chain.Finished = time.Now()
		t.chain.State = proto.STATE_INCOMPLETE
		t.chainRepo.Set(t.chain)

		// Stop the traverser from processing more jobs.
		close(t.jobChan)
	}
}

// setJobState sets the state of a job in a chain and then writes the entire
// chain to the chain repo. The chain mutex is expected to be locked before
// calling this.
func (t *traverser) setJobState(jobName string, state byte) {
	t.chain.Jobs[jobName].State = state
	t.chainRepo.Set(t.chain)
}

// enqueueNextJobs enqueues a jobs next jobs that are ready to run. The chain
// mutex is expected to be locked before calling this.
func (t *traverser) enqueueNextJobs(jobName string) {
	for _, nextJob := range t.chain.NextJobs(jobName) {
		if t.chain.JobIsReady(nextJob.Name) {
			glog.Infof("[chain=%d,job=%s]: Next job %s is ready to run. Enqueuing it.",
				t.chain.RequestId, jobName, nextJob.Name)
			t.jobChan <- nextJob
		} else {
			glog.Infof("[chain=%d,job=%s]: Next job %s is not ready to run. "+
				"Not enqueuing it.", t.chain.RequestId, jobName, nextJob.Name)
		}
	}
}

// Add a Runner to the active Runner map.
func (t *traverser) addRunner(jobName string, jr runner.Runner) {
	t.activeRMutex.Lock() // LOCK
	t.activeR[jobName] = jr
	t.activeRMutex.Unlock() // UNLOCK
}

// Remove a Runner from the active Runner map.
func (t *traverser) removeRunner(jobName string) {
	t.activeRMutex.Lock() // LOCK
	delete(t.activeR, jobName)
	t.activeRMutex.Unlock() // UNLOCK
}
