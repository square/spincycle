// Copyright 2018-2019, Square, Inc.

package chain

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/retry"
)

// A JobReaper handles jobs and chains that have finished running.
//
// The chain's current state (running as normal, stopped, or suspended) influences
// how jobs are handled once they finish running, and how the chain is handled
// once there are no more jobs to run. There are different implementations of
// the JobReaper for each of these cases - a running, stopped, or suspended chain.
type JobReaper interface {
	// Run reaps done jobs from doneJobChan, saving their states and enqueing
	// any jobs that should be run to runJobChan. When there are no more jobs to
	// reap, Run finalizes the chain and returns.
	Run()

	// Stop stops the JobReaper from reaping any more jobs. It blocks until
	// Run() returns and the reaper can be safely switched out for another
	// implementation.
	Stop()
}

// --------------------------------------------------------------------------
// JobReapers:
//  Each JobReaper implementation embeds a "reaper" struct with fields and methods
//  that all implementations use. In general, each reaper loops over doneJobChan,
//  receving jobs as they finish running, until there are no more jobs running.
//  Then the reaper finalizes the chain by sending some information about its state
//  to the Request Manager.
//
//  The RunningChainReaper is used for normally running chains - in the typical
//  case, this is the only reaper that will be used in the traverser. When a job
//  finished running and is sent to doneJobChan, the RunningChainReaper checks its
//  state and handles retrying the sequence if it failed or starting the next jobs
//  in the chain running if it completed. Once the chain is done (no more jobs are
//  or can be run), the reaper sends the chain's final state to the Request Manager.
//
//  The SuspendedChainReaper is switched out for the RunningChainReaper when the
//  traverser receives a signal that this Job Runner instance is shutting down.
//  It waits for all currently running jobs to finish and then determines whether
//  the chain is done (can any more jobs be run?). If the chain is done, the reaper
//  sends its final state to the Request Manager. In the more likely case that the
//  chain is NOT done, the reaper sends a SuspendedJobChain, containing all the info
//  required to later resume the chain, to the Request Manager.
//
//  The StoppedChainReaper is switched out for the RunningChainReaper when a user
//  requests that a currently-running chain be stopped. It waits for all currently
//  running jobs to finish and then sends the chain's final state (most often,
//  failed) to the Request Manager.
// --------------------------------------------------------------------------

const (
	// When checking if the runner repo is empty, wait 200ms before checking again.
	runnerRepoWait = 10 * time.Millisecond
)

// A ReaperFactory makes new JobReapers.
type ReaperFactory interface {
	MakeRunning() JobReaper
	MakeSuspended() JobReaper
	MakeStopped() JobReaper
}

// Implements ReaperFactory, creating 3 types of reapers - for a
// normally running chain, a stopped chain, or a suspended chain.
type ChainReaperFactory struct {
	Chain        *Chain
	ChainRepo    Repo
	Logger       *log.Entry
	RMClient     rm.Client
	RMCTries     int            // times to try sending info to RM
	RMCRetryWait time.Duration  // time to wait between tries to send info to RM
	DoneJobChan  chan proto.Job // chan jobs are reaped from
	RunJobChan   chan proto.Job // (running reaper) chan jobs to run are sent to
	RunnerRepo   runner.Repo    // (stopped + suspended reapers) repo of job runners
}

// Make a JobReaper for use on a running job chain.
func (f *ChainReaperFactory) MakeRunning() JobReaper {
	return &RunningChainReaper{
		reaper: reaper{
			chain:             f.Chain,
			rmc:               f.RMClient,
			logger:            f.Logger,
			finalizeTries:     f.RMCTries,
			finalizeRetryWait: f.RMCRetryWait,
			doneJobChan:       f.DoneJobChan,
			stopChan:          make(chan struct{}),
			doneChan:          make(chan struct{}),
			stopMux:           &sync.Mutex{},
		},
		runJobChan: f.RunJobChan,
	}
}

// Make a JobReaper for use on a job chain being suspended.
func (f *ChainReaperFactory) MakeSuspended() JobReaper {
	return &SuspendedChainReaper{
		reaper: reaper{
			chain:             f.Chain,
			rmc:               f.RMClient,
			logger:            f.Logger,
			finalizeTries:     f.RMCTries,
			finalizeRetryWait: f.RMCRetryWait,
			doneJobChan:       f.DoneJobChan,
			stopChan:          make(chan struct{}),
			doneChan:          make(chan struct{}),
			stopMux:           &sync.Mutex{},
		},
		runnerRepo: f.RunnerRepo,
	}
}

// Make a JobReaper for use on a job chain being stopped.
func (f *ChainReaperFactory) MakeStopped() JobReaper {
	return &StoppedChainReaper{
		reaper: reaper{
			chain:             f.Chain,
			rmc:               f.RMClient,
			logger:            f.Logger,
			finalizeTries:     f.RMCTries,
			finalizeRetryWait: f.RMCRetryWait,
			doneJobChan:       f.DoneJobChan,
			stopChan:          make(chan struct{}),
			doneChan:          make(chan struct{}),
			stopMux:           &sync.Mutex{},
		},
		runnerRepo: f.RunnerRepo,
	}
}

// -------------------------------------------------------------------------- //

// Job Reaper for running chains.
type RunningChainReaper struct {
	reaper
	runJobChan chan proto.Job // enqueue next jobs to run here
}

// Run reaps jobs when they finish running. For each job reaped, if...
// - chain is done: save final state + send to RM.
// - job failed:    retry sequence if possible.
// - job completed: prepared subsequent jobs and enqueue if runnable.
func (r *RunningChainReaper) Run() {
	defer close(r.doneChan)

	// If the chain is already done, skip straight to finalizing.
	done, complete := r.chain.IsDoneRunning()
	if done {
		r.Finalize(complete)
		return
	}

REAPER:
	for {
		select {
		case job := <-r.doneJobChan:
			r.Reap(job)
			done, complete = r.chain.IsDoneRunning()
			if done {
				break REAPER
			}
		case <-r.stopChan:
			// Don't Finalize the chain when stopping - the stopped or suspended
			// reaper will take care of that.
			return
		}
	}

	r.Finalize(complete)
}

// Stop stops the reaper from reaping any more jobs. It blocks until the reaper
// is stopped (will reap no more jobs and Run will return).
func (r *RunningChainReaper) Stop() {
	r.stopMux.Lock()
	defer r.stopMux.Unlock()
	if r.stopped {
		return
	}
	r.stopped = true

	close(r.stopChan)
	<-r.doneChan
	return
}

// reap takes a job that just finished running, saves its final state, and prepares
// to continue running the chain (or recognizes that the chain is done running).
//
// If chain is done: save final state + stop running more jobs.
// If job failed:    retry sequence if possible.
// If job completed: prepared subsequent jobs and enqueue if runnable.
func (r *RunningChainReaper) Reap(job proto.Job) {
	jLogger := r.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})

	// Set the final state of the job in the chain.
	r.chain.SetJobState(job.Id, job.State)

	switch job.State {
	case proto.STATE_COMPLETE:
		r.chain.IncrementFinishedJobs(1)

		for _, nextJob := range r.chain.NextJobs(job.Id) {
			nextJLogger := jLogger.WithFields(log.Fields{"next_job_id": nextJob.Id})

			// Copy job data to every child job, even if it's not ready to be run yet.
			// When a job has multiple parent jobs, it'll get job data copied from each
			// parent, not just the last one to finish. Be careful - it's possible for
			// parents to overwrite each other's job data if they set the same field.
			for k, v := range job.Data {
				nextJob.Data[k] = v
			}

			if !r.chain.IsRunnable(nextJob.Id) {
				nextJLogger.Infof("next job not runnable")
				continue
			}
			nextJLogger.Infof("enqueueing next job")
			r.runJobChan <- nextJob
		}
	case proto.STATE_STOPPED:
		jLogger.Infof("job stopped")
	default:
		// Job was NOT successful. The job.Runner already did job retries.
		// Retry sequence if possible.
		if !r.chain.CanRetrySequence(job.Id) {
			jLogger.Warn("job failed, no sequence tries left")
			return
		}
		jLogger.Warn("job failed, retrying sequence")
		sequenceStartJob := r.prepareSequenceRetry(job)
		r.runJobChan <- sequenceStartJob // re-enqueue first job in sequence
	}
}

// Finalize determines the final state of the chain and sends it to the Request Manager.
func (r *RunningChainReaper) Finalize(complete bool) {
	finishedAt := time.Now().UTC()
	if complete {
		r.logger.Infof("job chain complete")
		r.chain.SetState(proto.STATE_COMPLETE)
	} else {
		r.logger.Warn("job chain failed")
		r.chain.SetState(proto.STATE_FAIL)
	}
	r.sendFinalState(finishedAt)
}

// -------------------------------------------------------------------------- //

// Job Reaper for chains that are being suspended (stopped to be resumed later).
type SuspendedChainReaper struct {
	reaper
	runnerRepo runner.Repo
}

// Run reaps jobs when they finish running. For each job reaped, if it's...
// - completed: prepare subsequent jobs (copy jobData).
// - failed:    prepare a sequence retry.
// - stopped:   do nothing (job will be retried when chain is resumed).
func (r *SuspendedChainReaper) Run() {
	log.Infof("SuspendedChainReaper.Run: call")
	defer log.Infof("SuspendedChainReaper.Run: return")
	defer close(r.doneChan)

	// Sleep a short time to prevent a race condition on the runner Repo. If a job
	// was sent to traverser.runJobs() via runJobChan just before the chain was
	// suspended, runJobs() might not have created its runner + added it to the
	// Repo yet. Wait so we can be sure all running jobs have runners in the Repo,
	// so our checks to runnerRepo.Count will accurately reflect whether there are
	// any running jobs left.
	time.Sleep(runnerRepoWait)

	// If there are already no jobs left to reap, the running reaper must have
	// finished and finalized the chain before it got switched out for this reaper.
	// There's nothing left to do, so return right away.
	if r.runnerRepo.Count() == 0 {
		log.Infof("SuspendedChainReaper.Run: no active runners")
		return
	}

	// Reap jobs until there are no jobs left running, or the reaper is stopped.
REAPER:
	for r.runnerRepo.Count() > 0 {
		select {
		case job := <-r.doneJobChan:
			r.Reap(job)
		case <-time.After(runnerRepoWait):
			// No job to reap; go back to the loop condition to check if we're done.
			//
			// We need this case because it's possible for runnerRepo.Count() to be
			// > 0 even though there are no jobs left running. In traverser.runJobs(),
			// a runner gets removed from the repo AFTER its job is sent to the
			// reaper via doneJobChan. Reaping a job is fast in the suspended chain
			// reaper, so the reaper might receive the job, reap it, and go back to
			// check the loop condition before traverser.runJobs() gets a chance to
			// remove the runner from the repo. In that case, the loop condition
			// returns true, and we'd be stuck in this loop forever if we didn't
			//have this timeout case.
		case <-r.stopChan: // Stop called
			// Don't return right away - finalize the chain even when stopping.
			break REAPER
		}
	}

	r.Finalize()
}

// Stop stops the reaper from reaping any more jobs. It blocks until the reaper
// is stopped (will reap no more jobs and Run will return).
func (r *SuspendedChainReaper) Stop() {
	r.stopMux.Lock()
	defer r.stopMux.Unlock()
	if r.stopped {
		return
	}
	r.stopped = true

	close(r.stopChan)
	<-r.doneChan
	return
}

// reap takes a done job, saves its state, and prepares the chain to be resumed
// at a later time.
//
// If job is...
// Completed: prepare subsequent jobs (copy jobData).
// Failed:    prepare a sequence retry.
// Stopped:   nothing (job will be retried when chain is resumed).
func (r *SuspendedChainReaper) Reap(job proto.Job) {
	jLogger := r.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})

	// Set the final state of the job in the chain.
	r.chain.SetJobState(job.Id, job.State)

	switch job.State {
	case proto.STATE_FAIL:
		jLogger.Warn("job failed")
		// Prepare for sequence retry but don't actually start the retry.
		// This gets the chain ready to be resumed later on.
		if r.chain.CanRetrySequence(job.Id) {
			r.prepareSequenceRetry(job)
		}
	case proto.STATE_COMPLETE:
		jLogger.Infof("job completed")
		r.chain.IncrementFinishedJobs(1)
		// Copy job data to all child jobs.
		for _, nextJob := range r.chain.NextJobs(job.Id) {
			for k, v := range job.Data {
				nextJob.Data[k] = v
			}
		}
	default:
		// If job isn't complete or failed, must be stopped.
		jLogger.Infof("job stopped")
	}
}

// Finalize checks if the chain is done running or needs to be resumed later and
// either sends the Request Manager the chain's final state or a SuspendedJobChain
// that can be used to resume running the chain.
func (r *SuspendedChainReaper) Finalize() {
	finishedAt := time.Now().UTC()

	log.Infof("SuspendedChainReaper.Finalize: call")
	defer log.Infof("SuspendedChainReaper.Finalize: return")

	// Mark any jobs that didn't respond to Stop in time as Failed
	for jobId, jobStatus := range r.chain.Running() {
		r.logger.Infof("job %s still running, setting state to FAIL", jobId)
		jobId := jobStatus.JobId
		r.chain.SetJobState(jobId, proto.STATE_FAIL)
	}

	_, complete := r.chain.IsDoneRunning()
	if complete {
		r.logger.Infof("job chain complete")
		r.chain.SetState(proto.STATE_COMPLETE)
		r.sendFinalState(finishedAt)
		return
	}

	if r.chain.FailedJobs() > 0 {
		r.logger.Infof("job chain failed")
		r.chain.SetState(proto.STATE_FAIL)
		r.sendFinalState(finishedAt)
		return
	}

	// Send suspended job chain (SJC) to RM
	r.logger.Infof("suspending job chain")
	r.chain.SetState(proto.STATE_SUSPENDED)
	sjc := r.chain.ToSuspended()
	err := retry.Do(r.finalizeTries, r.finalizeRetryWait,
		func() error {
			return r.rmc.SuspendRequest(r.chain.RequestId(), sjc)
		},
		nil,
	)
	if err != nil {
		// If we couldn't suspend the request, mark it as failed instead.
		r.logger.Errorf("problem sending Suspended Job Chain to the Request Manager (%s). Treating chain as failed.", err)
		r.chain.SetState(proto.STATE_FAIL)
		r.sendFinalState(finishedAt)
	}
}

// -------------------------------------------------------------------------- //

// Job Reaper for chains that are being stopped.
type StoppedChainReaper struct {
	reaper
	runnerRepo runner.Repo
}

// Run reaps jobs when they finish running. For each job reaped, its state is saved.
func (r *StoppedChainReaper) Run() {
	defer close(r.doneChan)

	// Sleep a short time to prevent a race condition on the runner Repo. If a job
	// was sent to runJobChan just before the chain was suspended, its runner might
	// not have been created + added to the Repo yet. Wait so we can be sure all
	// running jobs have runners in the Repo, so our checks to runnerRepo.Count
	// will accurately reflect whether there are any running jobs left.
	time.Sleep(runnerRepoWait)

	// If there are already no jobs left to reap, the running reaper must have
	// finished and finalized the chain before it got switched out for this reaper.
	// There's nothing left to do, so return right away.
	if r.runnerRepo.Count() == 0 {
		return
	}

	// Reap jobs until there are no jobs left running, or the reaper is stopped.
REAPER:
	for r.runnerRepo.Count() > 0 {
		select {
		case job := <-r.doneJobChan:
			r.Reap(job)
		case <-time.After(runnerRepoWait):
			// No job to reap; go back to the loop condition to check if we're done.
			//
			// We need this case because it's possible for runnerRepo.Count() to be
			// > 0 even though there are no jobs left running. In traverser.runJobs(),
			// a runner gets removed from the repo AFTER its job is sent to the
			// reaper via doneJobChan. Reaping a job is fast in the stopped chain
			// reaper, so the reaper might receive the job, reap it, and go back to
			// check the loop condition before traverser.runJobs() gets a chance to
			// remove the runner from the repo. In that case, the loop condition
			// returns true, and we'd be stuck in this loop forever if we didn't
			// have this timeout case.
		case <-r.stopChan: // Stop called
			// Don't return right away - finalize the chain even when stopping.
			break REAPER
		}
	}

	r.Finalize()
}

// Stop stops the reaper from reaping any more jobs. It blocks until the reaper
// is stopped (will reap no more jobs and Run will return).
func (r *StoppedChainReaper) Stop() {
	r.stopMux.Lock()
	defer r.stopMux.Unlock()
	if r.stopped {
		return
	}
	r.stopped = true

	close(r.stopChan)
	<-r.doneChan
	return
}

// reap takes a done job and saves its state.
func (r *StoppedChainReaper) Reap(job proto.Job) {
	jLogger := r.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})
	jLogger.Info("job chain stopped")
	r.chain.SetJobState(job.Id, job.State)
	if job.State == proto.STATE_COMPLETE {
		r.chain.IncrementFinishedJobs(1)
	}
	return
}

// Finalize determines the final state of the chain and sends it to the Request Manager.
func (r *StoppedChainReaper) Finalize() {
	finishedAt := time.Now().UTC()

	// Mark any jobs that didn't respond to Stop in time as Failed
	for jobId, jobStatus := range r.chain.Running() {
		r.logger.Infof("job %s still running, setting state to FAIL", jobId)
		jobId := jobStatus.JobId
		r.chain.SetJobState(jobId, proto.STATE_FAIL)
	}

	// Check if the chain failed or managed to complete,
	// and send this final state to the RM.
	_, complete := r.chain.IsDoneRunning()
	if complete {
		r.logger.Infof("job chain complete")
		r.chain.SetState(proto.STATE_COMPLETE)
	} else {
		if r.chain.FailedJobs() > 0 {
			r.logger.Infof("job chain failed")
			r.chain.SetState(proto.STATE_FAIL)
		} else {
			r.logger.Infof("job chain stopped")
			r.chain.SetState(proto.STATE_STOPPED)
		}
	}
	r.sendFinalState(finishedAt)
}

// -------------------------------------------------------------------------- //

// reaper is embedded in each type of JobReaper - it has fields and methods
// they all use.
type reaper struct {
	chain             *Chain
	rmc               rm.Client
	logger            *log.Entry
	finalizeTries     int
	finalizeRetryWait time.Duration
	doneJobChan       chan proto.Job
	stopMux           *sync.Mutex
	stopped           bool
	stopChan          chan struct{}
	doneChan          chan struct{}
}

// Sends the final state of the chain to the Request Manager, retrying a few times
// if sending fails. It returns true if the final state was successfully sent;
// else false.
func (r *reaper) sendFinalState(finishedAt time.Time) {
	fr := proto.FinishRequest{
		RequestId:    r.chain.RequestId(),
		State:        r.chain.State(),
		FinishedAt:   finishedAt,
		FinishedJobs: r.chain.FinishedJobs(),
	}
	err := retry.Do(r.finalizeTries, r.finalizeRetryWait,
		func() error {
			return r.rmc.FinishRequest(fr)
		},
		nil,
	)
	if err != nil {
		r.logger.Errorf("problem sending final status of the finished chain to the Request Manager: %s", err)
	}
}

// prepareSequenceRetry prepares a sequence to retry. The caller should check
// r.chain.CanRetrySequence first; this func does not check the seq retry limit
// or increment seq try count (that's done in traverser.runJobs when the seq
// start job runs).
func (r *reaper) prepareSequenceRetry(failedJob proto.Job) proto.Job {
	sequenceStartJob := r.chain.SequenceStartJob(failedJob.Id)

	seqLogger := r.logger.WithFields(log.Fields{"sequence_id": sequenceStartJob.SequenceId})
	seqLogger.Info("preparing sequence retry")

	// sequenceJobsToRetry is a list containing the failed job and all previously
	// completed jobs in the sequence. For example, if job C of A -> B -> C -> D
	// fails, then A and B are the previously completed jobs and C is the failed
	// job. So, jobs A, B, and C will be added to sequenceJobsToRetry. D will not be
	// added because it was never run.
	sequenceJobsToRetry := r.sequenceJobsCompleted(sequenceStartJob)

	// @todo: fixme: sometimes we have failed job, sometimes we don't. The list
	//        must be unique, but depending on the job chain, sometimes the
	//        failed job is put in the list twice.
	haveFailedJob := false
	for _, j := range sequenceJobsToRetry {
		if j.Id == failedJob.Id {
			haveFailedJob = true
			break
		}
	}
	if !haveFailedJob {
		sequenceJobsToRetry = append(sequenceJobsToRetry, failedJob)
	}

	// Roll back completed sequence jobs
	// @todo: SPIN-501: Sequence retries don't work right for parallel jobs in the same sequence
	finishedJobs := 0
	for _, job := range sequenceJobsToRetry {
		state := r.chain.JobState(job.Id)
		if state == proto.STATE_COMPLETE {
			finishedJobs += 1
		}

		// Job current try count is per-seq try, so roll back to zero.
		// Job total try count never decrements, so this call only affects
		// current try count.
		// resets to zero.
		cur, _ := r.chain.JobTries(job.Id)             // job try count for current seq
		r.chain.IncrementJobTries(job.Id, -1*int(cur)) // decr job current tries by ^

		// Roll back job state to pending so it's runnable again
		r.chain.SetJobState(job.Id, proto.STATE_PENDING)
	}

	// Roll back finished job count. The -1 accounts for failedJob which did
	// not incr finished jobs.
	r.chain.IncrementFinishedJobs(-1 * finishedJobs)
	seqLogger.Infof("rolled back %d finished jobs", finishedJobs)

	// Running reaper will re-enqueue/re-run seq from this seq start job.
	// Suspend reaper will not, leaving seq in runnable state for when chain is resumed.
	return sequenceStartJob
}

// sequenceJobsCompleted does a BFS to find all jobs in the sequence that have
// completed. You can read how BFS works here:
// https://en.wikipedia.org/wiki/Breadth-first_search.
func (r *reaper) sequenceJobsCompleted(sequenceStartJob proto.Job) []proto.Job {
	toVisit := map[string]proto.Job{} // job id -> job to visit
	visited := map[string]proto.Job{} // job id -> job visited

	// Process sequenceStartJob
	for _, pJob := range r.chain.NextJobs(sequenceStartJob.Id) {
		toVisit[pJob.Id] = pJob
	}
	visited[sequenceStartJob.Id] = sequenceStartJob

PROCESS_TO_VISIT_LIST:
	for len(toVisit) > 0 {

	PROCESS_CURRENT_JOB:
		for currentJobId, currentJob := range toVisit {

		PROCESS_NEXT_JOBS:
			for _, nextJob := range r.chain.NextJobs(currentJobId) {
				// Don't add failed or pending jobs to toVisit list
				// For example, if job C of A -> B -> C -> D fails, then do not add C
				// or D to toVisit list. Because we have single sequence retries,
				// stopping at the failed job ensures we do not add jobs not in the
				// sequence to the toVisit list.
				if nextJob.State != proto.STATE_COMPLETE {
					continue PROCESS_NEXT_JOBS
				}

				// Make sure we don't visit a job multiple times. We can see a job
				// multiple times if it is a "fan in" node.
				if _, seen := visited[nextJob.Id]; !seen {
					toVisit[nextJob.Id] = nextJob
				}
			}

			// Since we have processed all of the next jobs for this current job, we
			// are done visiting the current job and can delete it from the toVisit
			// list and add it to the visited list.
			delete(toVisit, currentJobId)
			visited[currentJobId] = currentJob

			continue PROCESS_CURRENT_JOB
		}

		continue PROCESS_TO_VISIT_LIST
	}

	completedJobs := make([]proto.Job, 0, len(visited))
	for _, j := range visited {
		completedJobs = append(completedJobs, j)
	}

	return completedJobs
}
