// Copyright 2017, Square, Inc.

package chain

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
)

// A Traverser provides the ability to run a job chain while respecting the
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
	// It returns an error if it fails to stop all running jobs.
	Stop() error

	// Status gets the status of all running and failed jobs. Since a job can only
	// run when all of its ancestors have completed, the state of the entire chain
	// can be inferred from this information - every job in the chain before a
	// running or failed job must be complete, and every job in the chain after a
	// running or failed job must be pending.
	//
	// It returns an error if it fails to get the status of all running jobs.
	Status() (proto.JobChainStatus, error)
}

// A TraverserFactory makes new Traverser.
type TraverserFactory interface {
	Make(proto.JobChain) (Traverser, error)
}

type traverserFactory struct {
	chainRepo     Repo
	runnerFactory runner.Factory
	runnerRepo    runner.Repo
}

func NewTraverserFactory(cr Repo, rf runner.Factory, rr runner.Repo) TraverserFactory {
	return &traverserFactory{
		chainRepo:     cr,
		runnerFactory: rf,
		runnerRepo:    rr,
	}
}

// Make makes a Traverser for the given job chain. The chain is first validated
// and saved to the chain repo.
func (f *traverserFactory) Make(jobChain proto.JobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object
	chain := NewChain(&jobChain)

	// Validate the chain
	log.Infof("[chain=%s]: Validating the chain.", chain.RequestId())
	err := chain.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid chain: %s", err)
	}

	// Save the chain. If this JR instance dies, another can recover the chain
	// from the repo.
	log.Infof("[chain=%s]: Saving the chain to the repo.", chain.RequestId())
	err = f.chainRepo.Set(chain)
	if err != nil {
		return nil, fmt.Errorf("cannot save chain to repo: %s", err)
	}

	// Create and return a traverser for the chain. The traverser is responsible
	// for the chain: running, cleaning up, removing from repo when done, etc.
	// And traverser and chain have the same lifespan: traverser is done when
	// chain is done.
	return NewTraverser(chain, f.chainRepo, f.runnerFactory, f.runnerRepo)
}

// A traverser represents a job chain and everything needed to traverse it.
type traverser struct {
	// The chain that will be traversed.
	chain *chain

	// Repo for acessing/updating chains.
	chainRepo Repo

	// Factory for creating Runners.
	runnerFactory runner.Factory

	// Repo for keeping track of active Runners.
	runnerRepo runner.Repo

	// Used to stop a running traverser.
	stopChan chan struct{}

	// Queue for processing jobs that need to run.
	runJobChan chan proto.Job

	// Queue for processing jobs that are done running.
	doneJobChan chan proto.Job
}

// NewTraverser creates a new traverser for a job chain.
func NewTraverser(chain *chain, cr Repo, rf runner.Factory, rr runner.Repo) (*traverser, error) {
	return &traverser{
		chain:         chain,
		chainRepo:     cr,
		runnerFactory: rf,
		runnerRepo:    rr,
		stopChan:      make(chan struct{}),
		runJobChan:    make(chan proto.Job),
		doneJobChan:   make(chan proto.Job),
	}, nil
}

// Run runs all jobs in the chain and blocks until all jobs complete or a job fails.
func (t *traverser) Run() error {
	log.Infof("[chain=%s]: Starting the chain traverser.", t.chain.RequestId())
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
	log.Infof("[chain=%s]: Sending the first job (%s) to runJobChan.",
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
				log.Infof("[chain=%s]: Chain is done, all jobs finished successfully.", t.chain.RequestId())
				t.chain.SetComplete()
			} else {
				log.Infof("[chain=%s]: Chain is done, some jobs failed.", t.chain.RequestId())
				t.chain.SetIncomplete()
			}
			break
		}

		// If the job completed successfully, add its next jobs to
		// runJobChan. If it didn't complete successfully, do nothing.
		// Also, pass a copy of the job's jobData to all next jobs.
		//
		// It is important to note that when a job has multiple parent
		// jobs, it will get its jobData from whichever parent finishes
		// last. Therefore, a job should never rely on jobData that was
		// created during an unrelated sequence at any time earlier in
		// the chain.
		switch job.State {
		case proto.STATE_COMPLETE:
			for _, nextJob := range t.chain.NextJobs(job.Name) {
				// Check to make sure the job is ready to run.
				if t.chain.JobIsReady(nextJob.Name) {
					log.Infof("[chain=%s,job=%s]: Next job %s is ready to run. Enqueuing it.",
						t.chain.RequestId(), job.Name, nextJob.Name)
					// Set the state of the job in the chain to "Running".
					t.chain.SetJobState(nextJob.Name, proto.STATE_RUNNING)

					// Copy the jobData from the job that just finished to the next job.
					for k, v := range job.Data {
						nextJob.Data[k] = v
					}

					t.runJobChan <- nextJob // add the job to the run queue
				} else {
					log.Infof("[chain=%s,job=%s]: Next job %s is not ready to run. "+
						"Not enqueuing it.", t.chain.RequestId(), job.Name, nextJob.Name)
				}
			}
		default:
			log.Infof("[chain=%s,job=%s]: Job did not complete successfully, so not "+
				"enqueuing its next jobs.", t.chain.RequestId(), job.Name)
		}
	}

	return nil
}

// Stop stops the traverser if it's running.
func (t *traverser) Stop() error {
	log.Infof("[chain=%s]: Stopping the traverser and all jobs.", t.chain.RequestId())

	// Get all of the runners for this traverser from the repo. Only runners that are
	// in the repo will be stopped.
	activeRunners, err := t.runnerRepo.GetAll()
	if err != nil {
		return err
	}

	// Call Stop on each runner, and then remove it from the repo.
	for jobName, runner := range activeRunners {
		runner.Stop() // this should return quickly
		t.runnerRepo.Remove(jobName)
	}

	// Stop the traverser (i.e., stop running new jobs).
	close(t.stopChan)
	return nil
}

// Status returns the status of currently running jobs in the chain.
func (t *traverser) Status() (proto.JobChainStatus, error) {
	log.Infof("[chain=%s]: Getting the status of all running jobs.", t.chain.RequestId())
	var jobChainStatus proto.JobChainStatus
	var jobStatuses []proto.JobStatus

	// Get all of the runners for this traverser from the repo. Only runners that are
	// in the repo will have their statuses queried.
	activeRunners, err := t.runnerRepo.GetAll()
	if err != nil {
		return jobChainStatus, err
	}

	// Get the Status of each runner, as well as the state of the job it represents.
	for jobName, runner := range activeRunners {
		jobStatus := proto.JobStatus{
			Name:   jobName,
			Status: runner.Status(),           // get the job status. this should return quickly
			State:  t.chain.JobState(jobName), // get the state of the job
		}
		jobStatuses = append(jobStatuses, jobStatus)
	}

	return proto.JobChainStatus{
		RequestId:   t.chain.RequestId(),
		JobStatuses: jobStatuses,
	}, nil
}

// -------------------------------------------------------------------------- //

// runJobs loops on the runJobChannel and runs each job that comes through it in
// a goroutine. When it is done, it sends the job out through the doneJobChannel.
func (t *traverser) runJobs() {
	for job := range t.runJobChan {
		go func(j proto.Job) {
			defer func() { t.doneJobChan <- j }() // send the job to doneJobChan when done

			// Create a job runner.
			jr, err := t.runnerFactory.Make(j.Type, j.Name, j.Bytes, t.chain.RequestId())
			if err != nil {
				log.Errorf("[chain=%s,job=%s]: Error creating runner (error: %s).",
					t.chain.RequestId(), j.Name, err)
				j.State = proto.STATE_FAIL
				return
			}

			// Add the runner to the repo. Runners in the repo are used by the Status and
			// methods on the traverser.
			err = t.runnerRepo.Add(j.Name, jr)
			if err != nil {
				log.Errorf("[chain=%s,job=%s]: Error adding runner to the repo (error: %s).",
					t.chain.RequestId(), j.Name, err)
				j.State = proto.STATE_FAIL
				return
			}

			// Bail out if the traverser was stopped. It is important that we check this
			// AFTER the runner is added to the repo, because traverser.Stop only stops
			// runners in the repo. If we do not add this extra check, the following
			// sequence would cause a problem: 1) runner A gets created, 2) traverser.Stop
			// gets called, 3) runner A gets added to the repo, 4) runner A runs
			// unbounded even though we want to stop the traverser.
			select {
			case <-t.stopChan:
				log.Errorf("[chain=%s,job=%s]: stopChan was closed. Bailing out before running.",
					t.chain.RequestId(), j.Name)
				j.State = proto.STATE_FAIL
				return
			default:
			}

			// Run the job. This is a blocking operation that could take a long time.
			completed := jr.Run(j.Data)

			if completed {
				j.State = proto.STATE_COMPLETE

				// Remove the runner from the repo.
				//
				// Since the runner repo is used by the traverser's Status method,
				// and that method cares about failed runners, only remove from the
				// repo if the runner completed (i.e., leave failed runners in it).
				t.runnerRepo.Remove(j.Name)
			} else {
				j.State = proto.STATE_FAIL
			}
		}(job)
	}
}
