// Copyright 2018, Square, Inc.

package chain

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/retry"
)

const (
	// Number of times to attempt sending chain state / SJC to RM.
	FINALIZE_TRIES = 5
	// Time to wait between the above tries.
	FINALIZE_RETRY_WAIT = 500 * time.Millisecond
)

// A JobReaper handles jobs and chains that have finished running.
type JobReaper interface {
	// Reap takes a done job, saves its state, and then prepares the chain for
	// whatever should be done next.
	Reap(job proto.Job)

	// Finalize finalizes a job chain, sending the Request Manager its final state
	// or information that can be used to resume running the chain later.
	Finalize()
}

// A ReaperFactory makes new JobReapers.
type ReaperFactory interface {
	MakeRunning(chan proto.Job) JobReaper
	MakeSuspended() JobReaper
	MakeStopped() JobReaper
}

type reaperFactory struct {
	reaper reaper
}

// Create a new ReaperFactory.
func NewReaperFactory(chain *chain, chainRepo Repo, rmc rm.Client) ReaperFactory {
	return NewReaperFactoryWithRetries(chain, chainRepo, rmc, FINALIZE_TRIES, FINALIZE_RETRY_WAIT)
}

// Create a new ReaperFactory with custom settings for retrying RMClient calls.
// Used in testing to decrease default retry wait duration.
func NewReaperFactoryWithRetries(chain *chain, chainRepo Repo, rmc rm.Client, rmTries int, rmRetryWait time.Duration) ReaperFactory {
	return &reaperFactory{
		reaper: reaper{
			chain:             chain,
			chainRepo:         chainRepo,
			rmc:               rmc,
			logger:            log.WithFields(log.Fields{"requestId": chain.RequestId()}),
			finalizeTries:     rmTries,
			finalizeRetryWait: rmRetryWait,
		},
	}
}

// Make a JobReaper for use on a running job chain.
func (f *reaperFactory) MakeRunning(runJobChan chan proto.Job) JobReaper {
	return &runningChainReaper{
		reaper:     f.reaper,
		runJobChan: runJobChan,
	}
}

// Make a JobReaper for use on a job chain being suspended.
func (f *reaperFactory) MakeSuspended() JobReaper {
	return &suspendedChainReaper{
		reaper: f.reaper,
	}
}

// Make a JobReaper for use on a job chain being stopped.
func (f *reaperFactory) MakeStopped() JobReaper {
	return &stoppedChainReaper{
		reaper: f.reaper,
	}
}

// -------------------------------------------------------------------------- //

// Job Reaper for running chains.
type runningChainReaper struct {
	reaper
	runJobChan chan proto.Job // enqueue next jobs to run here
}

// Reap takes a job that just finished running, saves its final state, and prepares
// to continue running the chain (or recognizes that the chain is done running).
//
// If chain is done: save final state + stop running more jobs.
// If job failed:    retry sequence if possible.
// If job completed: prepared subsequent jobs and enqueue if ready to run.
func (r *runningChainReaper) Reap(job proto.Job) {
	jLogger := r.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})

	// Set the final state of the job in the chain.
	r.chain.SetJobState(job.Id, job.State)
	r.chainRepo.Set(r.chain)

	// A chain is done if there are no more jobs that can be run and there are
	// no jobs currently running. This reaper won't be used when stopping
	// suspending a chain, so we don't need to worry about Stopped jobs when
	// checking if the chain is done.
	done, complete := r.chain.IsDoneRunning()
	if done {
		// The chain is done - close runJobChan to start finishing up the chain.
		// Since we only reap one job at a time, and the reaper sets the job's
		// final state in the chain, the chain will only be done on the last
		// job that is ever reaped. This means we won't accidently close runJobChan
		// twice.
		close(r.runJobChan)

		if complete {
			jLogger.Infof("chain is done, all jobs finished successfully")
			r.chain.SetState(proto.STATE_COMPLETE)
		} else {
			jLogger.Warn("chain is done, some jobs failed")
			r.chain.SetState(proto.STATE_FAIL)
		}
		r.chainRepo.Set(r.chain)
		return
	}

	if job.State == proto.STATE_FAIL {
		if r.chain.CanRetrySequence(job.Id) {
			// Retry the sequence if possible.
			jLogger.Info("job did not complete successfully. retrying sequence.")
			r.prepareSequenceRetry(job)

			// Re-enqueue the first job of the sequence and increment the
			// sequence's try count. Subsequent jobs in the sequence will be
			// enqueued after the first job completes.
			sequenceStartJob := r.chain.SequenceStartJob(job.Id)
			r.chain.IncrementSequenceTries(sequenceStartJob.Id)
			seqLogger := r.logger.WithFields(log.Fields{"sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})
			seqLogger.Info("starting try of sequence")
			r.runJobChan <- sequenceStartJob // re-enqueue first job in sequence
			return
		}

		// If the failed job was stopped or is not part of a retryable
		// sequence, then ignore subsequent jobs. For example, if job B of
		// A -> B -> C  fails, then C is not run.
		jLogger.Warn("job did not complete successfully.")
		return
	}
	jLogger.Infof("job completed successfully.")

	// Copy job data from this job to all child jobs. When a job has multiple
	// parent jobs, it will get jobData copied from all of its parent jobs in
	// the order that they finish. Therefore, if multiple parent jobs try to set
	// the same jobData field, it is undetermined which parent job the job will
	// receive the final copy of that jobData field from.
	for _, nextJob := range r.chain.NextJobs(job.Id) {
		nextJLogger := jLogger.WithFields(log.Fields{"next_job_id": nextJob.Id})

		for k, v := range job.Data {
			nextJob.Data[k] = v
		}
		r.chainRepo.Set(r.chain)

		// Enqueue any ready-to-run child jobs.
		if !r.chain.JobIsReady(nextJob.Id) {
			nextJLogger.Infof("next job is not ready to run - not enqueuing it")
			continue
		}
		nextJLogger.Infof("next job is ready to run - enqueuing it")
		if r.chain.IsSequenceStartJob(nextJob.Id) {
			// Starting a sequence, so increment sequence try count.
			r.chain.IncrementSequenceTries(nextJob.Id)
			seqLogger := r.logger.WithFields(log.Fields{"sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})
			seqLogger.Info("starting try of sequence")
		}
		r.runJobChan <- nextJob
	}
}

// Finalize sends the final state of the chain to the Request Manager.
func (r *runningChainReaper) Finalize() {
	r.sendFinalState()
}

// -------------------------------------------------------------------------- //

// Job Reaper for chains that are being suspended (stopped to be resumed later).
type suspendedChainReaper struct {
	reaper
}

// Reap takes a done job, saves its state, and prepares the chain to be resumed
// at a later time.
//
// If job...
// Completed: prepare subsequent jobs (copy jobData).
// Failed:    prepare a sequence retry.
// Stopped:   nothing (job will be retried when chain is resumed).
func (r *suspendedChainReaper) Reap(job proto.Job) {
	jLogger := r.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})

	// Set the final state of the job in the chain.
	r.chain.SetJobState(job.Id, job.State)
	r.chainRepo.Set(r.chain)

	switch job.State {
	case proto.STATE_FAIL:
		jLogger.Warn("job did not complete successfully.")
		// Prepare for sequence retry but don't actually start the retry.
		// This gets the chain ready to be resumed later on.
		if r.chain.CanRetrySequence(job.Id) {
			r.prepareSequenceRetry(job)
		}
	case proto.STATE_COMPLETE:
		jLogger.Infof("job completed successfully.")
		// Copy job data to all child jobs.
		for _, nextJob := range r.chain.NextJobs(job.Id) {
			for k, v := range job.Data {
				nextJob.Data[k] = v
			}
		}
	default:
		// If job isn't complete or failed, must be stopped.
		jLogger.Infof("job was stopped.")
	}
}

// Finalize checks if the chain is done running or needs to be resumed later and
// either sends the Request Manager the chain's final state or a SuspendedJobChain
// that can be used to resume running the chain.
func (r *suspendedChainReaper) Finalize() {
	// Mark any jobs that didn't respond to Stop in time as Failed
	for _, jobStatus := range r.chain.Running() {
		jobId := jobStatus.JobId
		r.chain.SetJobState(jobId, proto.STATE_FAIL)
	}

	done, complete := r.chain.IsDoneRunning()
	if done {
		// If chain finished, send final state to RM.
		if complete {
			r.chain.SetState(proto.STATE_COMPLETE)
		} else {
			r.chain.SetState(proto.STATE_FAIL)
		}
		r.chainRepo.Set(r.chain)
		r.sendFinalState()
		return
	}

	// Chain isn't done - send SuspendedJobChain to RM.
	r.chain.SetState(proto.STATE_SUSPENDED)
	r.chainRepo.Set(r.chain)
	r.sendSJC()
}

// -------------------------------------------------------------------------- //

// Job Reaper for chains that are being stopped.
type stoppedChainReaper struct {
	reaper
}

// Reap takes a done job and saves its state.
func (r *stoppedChainReaper) Reap(job proto.Job) {
	jLogger := r.logger.WithFields(log.Fields{"job_id": job.Id, "sequence_id": job.SequenceId, "sequence_try": r.chain.SequenceTries(job.Id)})
	jLogger.Info("job was stopped.")

	// Set the final state of the job in the chain.
	r.chain.SetJobState(job.Id, job.State)
	r.chainRepo.Set(r.chain)

	return
}

// Finalize determines the final state of the chain and sends it to the Request
// Manager.
func (r *stoppedChainReaper) Finalize() {
	// Mark any jobs that didn't respond to Stop in time as Failed
	for _, jobStatus := range r.chain.Running() {
		jobId := jobStatus.JobId
		r.chain.SetJobState(jobId, proto.STATE_FAIL)
	}

	// Check if the chain failed or managed to complete,
	// and send this final state to the RM.
	_, complete := r.chain.IsDoneRunning()
	if complete {
		r.chain.SetState(proto.STATE_COMPLETE)
	} else {
		r.chain.SetState(proto.STATE_FAIL)
	}
	r.chainRepo.Set(r.chain)
	r.sendFinalState()
}

// -------------------------------------------------------------------------- //

// reaper is embedded in each type of JobReaper - it has fields and methods
// they all use.
type reaper struct {
	chain             *chain
	chainRepo         Repo
	rmc               rm.Client
	logger            *log.Entry
	finalizeTries     int
	finalizeRetryWait time.Duration
}

// Sends the final state of the chain to the Request Manager, retrying a few times
// if sending fails.
func (r *reaper) sendFinalState() {
	err := retry.Do(r.finalizeTries, r.finalizeRetryWait,
		func() error {
			return r.rmc.FinishRequest(r.chain.RequestId(), r.chain.State())
		},
		func(err error) {
			r.logger.Errorf("problem reporting status of the finished chain - retrying: %s", err)
		},
	)
	if err != nil {
		r.logger.Errorf("failed to report final status of the finished chain to Request Manager")
		return
	}
	r.logger.Infof("final status of finished chain sent to Request Manager")
	r.chainRepo.Remove(r.chain.RequestId())
}

// Sends the suspended job chain to the Request Manager, retrying a few times
// if sending fails.
func (r *reaper) sendSJC() {
	sjc := r.chain.ToSuspended()
	err := retry.Do(r.finalizeTries, r.finalizeRetryWait,
		func() error {
			return r.rmc.SuspendRequest(r.chain.RequestId(), sjc)
		},
		func(err error) {
			r.logger.Errorf("problem sending suspended job chain to RM: %s. Retrying...", err)
		},
	)
	if err != nil {
		r.logger.Errorf("failed to send suspended job chain to Request Manager")
		return
	}
	r.logger.Infof("suspended job chain sent to Request Manager")
	r.chainRepo.Remove(r.chain.RequestId())
}

// prepareSequenceRetry prepares a sequence to be retried:
//   1. Mark failed job and all previously completed jobs in sequence as PENDING.
//   2. Decrement the number of jobs that were previously run (failed job +
//   previously completed jobs in sequence) from the chain job count.
//   3. Increment retry count for the sequence.
func (r *reaper) prepareSequenceRetry(failedJob proto.Job) {
	sequenceStartJob := r.chain.SequenceStartJob(failedJob.Id)

	// sequenceJobsToRetry is a list containing the failed job and all previously
	// completed jobs in the sequence. For example, if job C of A -> B -> C -> D
	// fails, then A and B are the previously completed jobs and C is the failed
	// job. So, jobs A, B, and C will be added to sequenceJobsToRetry. D will not be
	// added because it was never run.
	var sequenceJobsToRetry []proto.Job
	sequenceJobsToRetry = append(sequenceJobsToRetry, failedJob)
	sequenceJobsCompleted := r.sequenceJobsCompleted(sequenceStartJob)
	sequenceJobsToRetry = append(sequenceJobsToRetry, sequenceJobsCompleted...)

	// Mark all jobs to retry to PENDING state
	for _, job := range sequenceJobsToRetry {
		r.chain.SetJobState(job.Id, proto.STATE_PENDING)
	}
	r.chainRepo.Set(r.chain)
}

// sequenceJobsCompleted does a BFS to find all jobs in the sequence that have
// completed. You can read how BFS works here:
// https://en.wikipedia.org/wiki/Breadth-first_search.
func (r *reaper) sequenceJobsCompleted(sequenceStartJob proto.Job) []proto.Job {
	// toVisit is a map of job id->job to visit
	toVisit := map[string]proto.Job{}
	// visited is a map of job id->job visited
	visited := map[string]proto.Job{}

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
