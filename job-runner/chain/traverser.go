// Copyright 2017, Square, Inc.

package chain

import (
	"strconv"

	"github.com/square/spincycle/job-runner/cache"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"

	log "github.com/Sirupsen/logrus"
)

const CACHE_DB_PREFIX = "TRAVERSER"

// A traverser provides the ability to run a job chain while respecting the
// dependencies between the jobs.
type Traverser interface {
	// Run traverses a job chain and runs all of the jobs in it. It starts by
	// running the first job in the chain, and then, if the job completed,
	// successfully, running its adjacent jobs. This process continues until there
	// or no more jobs to run, or until the Stop method is called on the traverser.
	//
	// It returns an error if it fails to start.
	Run() error

	// Stop makes a traverser stop traversing its job chain. It also sends a stop
	// signal to all of the jobs that a traverser is running.
	//
	// It returns an error if it fails to stop running jobs.
	Stop() error

	// Status gets the status of all running and failed jobs. Since a job can only
	// run when all of its ancestors have completed, the state of the entire chain
	// can be inferred from this information - every job in the chain before a
	// running or failed job must be complete, and every job in the chain after a
	// running or failed job must be pending.
	Status() proto.JobChainStatus
}

// A traverser represents a job chain and everything needed to traverse it.
type traverser struct {
	// The chain that will be traversed.
	chain *chain

	// Repo for acessing/updating chains.
	chainRepo Repo

	// Factory for creating Runners.
	rf runner.RunnerFactory

	// Used to stop a running traverser.
	stopChan chan struct{}

	// Queue for processing jobs that need to run.
	runJobChan chan proto.Job

	// Queue for processing jobs that are done running.
	doneJobChan chan proto.Job

	// Cache for storing Runners.
	cache cache.Cacher
}

// NewTraverser creates a new traverser for a job chain.
func NewTraverser(repo Repo, rf runner.RunnerFactory, chain *chain, cache cache.Cacher) *traverser {
	return &traverser{
		chain:       chain,
		chainRepo:   repo,
		rf:          rf,
		stopChan:    make(chan struct{}),
		runJobChan:  make(chan proto.Job),
		doneJobChan: make(chan proto.Job),
		cache:       cache,
	}
}

func (t *traverser) Run() error {
	log.Infof("[chain=%d]: Starting the chain traverser.", t.chain.RequestId())
	firstJob, err := t.chain.FirstJob()
	if err != nil {
		return err
	}

	// Set the starting state of the chain.
	t.chain.SetStart()
	t.chainRepo.Set(t.chain)

	// Start a goroutine to run jobs. This consumes from the runJobChan. When
	// jobs are done, they will be sent to the doneJobChan, which gets consumed
	// from right below this.
	go t.runJobs()

	// Set the state of the first job in the chain to RUNNING.
	t.chain.SetJobState(firstJob.Name, proto.STATE_RUNNING)
	t.chainRepo.Set(t.chain)

	// Add the first job in the chain to the runJobChan.
	log.Infof("[chain=%d]: Sending the first job (%s) to runJobChan.",
		t.chain.RequestId(), firstJob.Name)
	t.runJobChan <- firstJob

	// When a job finishes, update the state of the chain and figure out what
	// to do next (check to see if the entire chain is done running, and
	// enqueue the next jobs if there are any).
	for job := range t.doneJobChan {
		// Set the final state of the job in the chain.
		t.chain.SetJobState(job.Name, job.State)
		t.chainRepo.Set(t.chain)

		// Check to see if the entire chain is done. If it is, break out of
		// the loop on doneJobChan because there is no more work for us to do.
		//
		// A chain is done if no more jobs in it can run. A chain is
		// complete if every job in it completed successfully.
		done, complete := t.chain.IsDone()
		if done {
			close(t.runJobChan)
			if complete {
				t.chain.SetComplete()
			} else {
				t.chain.SetIncomplete()
			}
			break
		}

		// If the job completed successfully, add its next jobs to
		// runJobChan. If it didn't complete successfully, do nothing.
		switch job.State {
		case proto.STATE_COMPLETE:
			for _, nextJob := range t.chain.NextJobs(job.Name) {
				// Check to make sure the job is ready to run.
				if t.chain.JobIsReady(nextJob.Name) {
					log.Infof("[chain=%d,job=%s]: Next job %s is ready to run. Enqueuing it.",
						t.chain.RequestId(), job.Name, nextJob.Name)
					// Set the state of the job in the chain to "Running".
					t.chain.SetJobState(nextJob.Name, proto.STATE_RUNNING)

					t.runJobChan <- nextJob // add the job to the run queue
				} else {
					log.Infof("[chain=%d,job=%s]: Next job %s is not ready to run. "+
						"Not enqueuing it.", t.chain.RequestId(), job.Name, nextJob.Name)
				}
			}
		default:
			log.Infof("[chain=%d,job=%s]: Job did not complete successfully, so not "+
				"enqueuing its next jobs.", t.chain.RequestId(), job.Name)
		}
	}

	return nil
}

func (t *traverser) Stop() error {
	log.Infof("[chain=%d]: Stopping the traverser and all jobs.", t.chain.RequestId())

	// Get all of the runners for this traverser from the cache. Only runners that are
	// in the cache will be stopped.
	cachedRunners := t.cache.GetAll(t.cacheDb())

	// Call Stop on each runner, and then remove it from the cache.
	for jobName, i := range cachedRunners {
		r, ok := i.(runner.Runner) // make sure we got a Runner from the cache
		if !ok {
			log.Errorf("[chain=%d,job=%s]: Error getting runner from the cache.",
				t.chain.RequestId(), jobName)
			continue
		}
		r.Stop() // this should return quickly
		t.cache.Delete(t.cacheDb(), jobName)
	}

	// Stop the traverser (i.e., stop running new jobs).
	close(t.stopChan)
	return nil
}

func (t *traverser) Status() proto.JobChainStatus {
	log.Infof("[chain=%d]: Getting the status of all running jobs.", t.chain.RequestId())

	// Get all of the runners for this traverser from the cache. Only runners that are
	// in the cache will have their statuses queried.
	cachedRunners := t.cache.GetAll(t.cacheDb())

	var jobStatuses []proto.JobStatus
	// Get the Status of each runner, as well as the state of the job it represents.
	for jobName, i := range cachedRunners {
		r, ok := i.(runner.Runner) // make sure we got a Runner from the cache
		if !ok {
			log.Errorf("[chain=%d,job=%s]: Error getting runner from the cache.",
				t.chain.RequestId(), jobName)
			continue
		}

		jobStatus := proto.JobStatus{
			Name:   jobName,
			Status: r.Status(),                // get the job status. this should return quickly
			State:  t.chain.JobState(jobName), // get the state of the job
		}
		jobStatuses = append(jobStatuses, jobStatus)
	}

	return proto.JobChainStatus{
		RequestId:   t.chain.RequestId(),
		JobStatuses: jobStatuses,
	}
}

// -------------------------------------------------------------------------- //

// runJobs loops on the runJobChannel and runs each job that comes through it in
// a goroutine. When it is done, it sends the job out through the doneJobChannel.
func (t *traverser) runJobs() {
	for job := range t.runJobChan {
		go func(j proto.Job) {
			defer func() { t.doneJobChan <- j }() // send the job to doneJobChan when done

			// Create a job runner.
			jr, err := t.rf.Make(j.Type, j.Name, j.Bytes, t.chain.RequestId())
			if err != nil {
				log.Errorf("[chain=%d,job=%s]: Error creating runner (error: %s).",
					t.chain.RequestId(), j.Name, err)
				j.State = proto.STATE_FAIL
				return
			}

			// Add the runner to the cache. Cached runners are used by the Status and
			// methods on the traverser.
			err = t.cache.Add(t.cacheDb(), j.Name, jr)
			if err != nil {
				log.Errorf("[chain=%d,job=%s]: Error adding runner to the cache (error: %s).",
					t.chain.RequestId(), j.Name, err)
				j.State = proto.STATE_FAIL
				return
			}

			// Bail out if the traverser was stopped. It is important that we check this
			// AFTER the runner is added to the cache, because traverser.Stop only stops
			// runners in the cache. If we do not add this extra check, the following
			// sequence would cause a problem: 1) runner A gets created, 2) traverser.Stop
			// gets called, 3) runner A gets added to the cache, 4) runner A runs
			// unbounded even though we want to stop the traverser.
			select {
			case <-t.stopChan:
				log.Errorf("[chain=%d,job=%s]: stopChan was closed. Bailing out before running.",
					t.chain.RequestId(), j.Name)
				j.State = proto.STATE_FAIL
				return
			default:
			}

			// Get a copy of the current jobData.
			currentJobData := t.chain.CurrentJobData()

			// Run the job. This is a blocking operation that could take a long time.
			completed := jr.Run(currentJobData)

			// Merge the current jobData (which at this point it has potentially been
			// modified by the job) bach into the chain's jobData.
			t.chain.AddJobData(currentJobData)

			if completed {
				j.State = proto.STATE_COMPLETE

				// Remove the runner from the cache.
				//
				// Since the runner cache is used by the traverser's Status method,
				// and that method cares about failed runners, only remove from the
				// cache if the runner completed (i.e., leave failed runners in it).
				t.cache.Delete(t.cacheDb(), j.Name)
			} else {
				j.State = proto.STATE_FAIL
			}
		}(job)
	}
}

// cacheDb returns a string unique for this traverser that can be used as the
// key for storing stuff in the cache.
func (t *traverser) cacheDb() string {
	return CACHE_DB_PREFIX + strconv.FormatUint(uint64(t.chain.RequestId()), 10)
}
