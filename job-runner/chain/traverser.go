// Copyright 2017, Square, Inc.

package chain

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/orcaman/concurrent-map"
	job "github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
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
	chainRepo  Repo
	jobFactory job.Factory
	rmClient   rm.Client
}

func NewTraverserFactory(cr Repo, jf job.Factory, rmc rm.Client) TraverserFactory {
	return &traverserFactory{
		chainRepo:  cr,
		jobFactory: jf,
		rmClient:   rmc,
	}
}

// Make makes a Traverser for the given job chain. The chain is first validated
// and saved to the chain repo.
func (f *traverserFactory) Make(jobChain proto.JobChain) (Traverser, error) {
	// Convert/wrap chain from proto to Go object.
	chain := NewChain(&jobChain)

	// Validate the chain
	log.Infof("[chain=%s]: Validating the chain.", chain.RequestId())
	err := chain.Validate()
	if err != nil {
		return nil, err
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
	tr := NewTraverser(chain, f.chainRepo, f.jobFactory, f.rmClient)
	return tr, nil
}

// A traverser represents a job chain and everything needed to traverse it.
type traverser struct {
	// The chain that will be traversed.
	chain *chain

	// Repo for acessing/updating chains.
	chainRepo Repo

	// Factory for creating runnable jobs. These are the actual jobs that get
	// run and satisfy the SpinCyle Job interface.
	jobFactory job.Factory

	// Repo for keeping track of active jobs.
	jobRepo cmap.ConcurrentMap

	// Used to stop a running traverser.
	stopChan chan struct{}

	// Queue for processing jobs that need to run.
	runJobChan chan proto.Job

	// Queue for processing jobs that are done running.
	doneJobChan chan proto.Job

	// Client for communicating with the request manager.
	rmClient rm.Client
}

// NewTraverser creates a new traverser for a job chain.
func NewTraverser(chain *chain, cr Repo, jf job.Factory, rmc rm.Client) *traverser {
	return &traverser{
		chain:       chain,
		chainRepo:   cr,
		jobFactory:  jf,
		jobRepo:     cmap.New(),
		stopChan:    make(chan struct{}),
		runJobChan:  make(chan proto.Job),
		doneJobChan: make(chan proto.Job),
		rmClient:    rmc,
	}
}

// Run runs all jobs in the chain and blocks until all jobs complete or a job fails.
func (t *traverser) Run() error {
	log.Infof("[chain=%s]: Starting the chain traverser.", t.chain.RequestId())
	firstJob, err := t.chain.FirstJob()
	if err != nil {
		return err
	}

	// Set the starting state of the chain.
	t.chain.SetState(proto.STATE_RUNNING)
	t.chainRepo.Set(t.chain)

	// Start a goroutine to run jobs. This consumes from the runJobChan. When
	// jobs are done, they will be sent to the doneJobChan, which gets consumed
	// from right below this.
	go t.runJobs()

	// Set the state of the first job in the chain to RUNNING.
	t.chain.SetJobState(firstJob.Id, proto.STATE_RUNNING)
	t.chainRepo.Set(t.chain)

	// Add the first job in the chain to the runJobChan.
	log.Infof("[chain=%s]: Sending the first job (%s) to runJobChan.",
		t.chain.RequestId(), firstJob.Id)
	t.runJobChan <- firstJob

	// When a job finishes, update the state of the chain and figure out what
	// to do next (check to see if the entire chain is done running, and
	// enqueue the next jobs if there are any).
	for doneJ := range t.doneJobChan {
		// Set the final state of the job in the chain.
		t.chain.SetJobState(doneJ.Id, doneJ.State)
		t.chainRepo.Set(t.chain)

		// Check to see if the entire chain is done. If it is, break out of
		// the loop on doneJobChan because there is no more work for us to do.
		//
		// A chain is done if no more jobs in it can run. A chain is
		// complete if every job in it completed successfully.
		done, complete := t.chain.IsDone()
		if done {
			var finalState byte
			close(t.runJobChan)
			if complete {
				log.Infof("[chain=%s]: Chain is done, all jobs finished successfully.", t.chain.RequestId())
				finalState = proto.STATE_COMPLETE
			} else {
				log.Infof("[chain=%s]: Chain is done, some jobs failed.", t.chain.RequestId())
				finalState = proto.STATE_INCOMPLETE
			}
			t.chain.SetState(finalState)

			// Mark the request as finished in the Request Manager.
			err = t.rmClient.FinishRequest(t.chain.RequestId(), finalState)
			if err != nil {
				log.Errorf("[chain=%s]: Could not mark the request as finished in the RM (error: %s).",
					t.chain.RequestId(), err)
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
		switch doneJ.State {
		case proto.STATE_COMPLETE:
			log.Infof("[chain=%s,job=%s]: Job completed successfully.", t.chain.RequestId(), doneJ.Id)
			for _, nextJ := range t.chain.NextJobs(doneJ.Id) {
				// Check to make sure the job is ready to run.
				if t.chain.JobIsReady(nextJ.Id) {
					log.Infof("[chain=%s,job=%s]: Next job %s is ready to run. Enqueuing it.",
						t.chain.RequestId(), doneJ.Id, nextJ.Id)
					// Set the state of the job in the chain to "Running".
					t.chain.SetJobState(nextJ.Id, proto.STATE_RUNNING)

					// Copy the jobData from the job that just finished to the next job.
					for k, v := range doneJ.Data {
						nextJ.Data[k] = v
					}

					t.runJobChan <- nextJ // add the job to the run queue
				} else {
					log.Infof("[chain=%s,job=%s]: Next job %s is not ready to run. "+
						"Not enqueuing it.", t.chain.RequestId(), doneJ.Id, nextJ.Id)
				}
			}
		default:
			log.Infof("[chain=%s,job=%s]: Job did not complete successfully, so not "+
				"enqueuing its next jobs.", t.chain.RequestId(), doneJ.Id)
		}
	}

	return nil
}

// Stop stops the traverser if it's running.
func (t *traverser) Stop() error {
	log.Infof("[chain=%s]: Stopping the traverser and all jobs.", t.chain.RequestId())

	// Stop the traverser (i.e., stop running new jobs).
	close(t.stopChan)

	// Get all of the jobs for this traverser from the repo. Only jobs that are
	// in the repo will be stopped.
	activeJobs := t.jobRepo.Items()

	// Call Stop on each job, and then remove it from the repo.
	for jobId, val := range activeJobs {
		realJob, ok := val.(job.Job)
		if !ok {
			log.Errorf("[chain=%s,job=%s]: Got an invalid job from the repo, skipping.",
				t.chain.RequestId(), jobId)
			continue
		}

		err := realJob.Stop() // this should return quickly
		if err != nil {
			log.Errorf("[chain=%s,job=%s]: Problem stopping the Job (error: %s).",
				t.chain.RequestId(), jobId, err)
		}
		t.jobRepo.Remove(jobId)
	}

	return nil
}

// Status returns the status of currently running jobs in the chain.
func (t *traverser) Status() (proto.JobChainStatus, error) {
	log.Infof("[chain=%s]: Getting the status of all running jobs.", t.chain.RequestId())
	var jobStatuses []proto.JobStatus

	// Get all of the jobs for this traverser from the repo. Only jobs that are
	// in the repo will have their statuses queried.
	activeJobs := t.jobRepo.Items()

	// Get the status and state of each job.
	for jobId, val := range activeJobs {
		realJob, ok := val.(job.Job)
		if !ok {
			log.Errorf("[chain=%s,job=%s]: Got an invalid job from the repo, skipping.",
				t.chain.RequestId(), jobId)
			continue
		}

		jobStatus := proto.JobStatus{
			Id:     jobId,
			Status: realJob.Status(),        // get the job status. this should return quickly
			State:  t.chain.JobState(jobId), // get the state of the job
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
// a goroutine. When it is done, it sends a jog log to the request manager,
// and then it sends the job out through the doneJobChan.
func (t *traverser) runJobs() {
	for runJ := range t.runJobChan {
		go func(j proto.Job) {
			// Send the job to the doneJobChan when done.
			defer func() { t.doneJobChan <- j }()

			// Send a JL to the RM when done. This deferred function
			// will use the latest values of jobRet and err as
			// arguments to the snedJL function, so it might look
			// like we're throwing away errors when we return early,
			// but in reality the err var is getting used here.
			var jobRet job.Return   // the struct that job.Run returns
			var err error           // any error around running a job
			startedAt := time.Now() // the start time of the job
			defer func(startedAt time.Time) {
				finishedAt := time.Now()
				// Call sendJL with all relevant info.
				t.sendJL(j, jobRet, err, startedAt, finishedAt)
			}(startedAt)

			// Construct a real job (a thing that satisfies the
			// fundamental SpinCycle Job interface) that will be run.
			realJob, err := t.jobFactory.Make(j.Type, j.Id)
			if err != nil {
				j.State = proto.STATE_FAIL
				err = fmt.Errorf("error making job: %s", err)
				return // both defer funcs will be called
			}

			// Have the real job re-create itself so it's no longer blank but
			// rather what it was when first created in the Request Manager.
			if err := realJob.Deserialize(j.Bytes); err != nil {
				j.State = proto.STATE_FAIL
				err = fmt.Errorf("error deserializing job from bytes: %s", err)
				return // both defer funcs will be called
			}

			// Add the real job to the repo. Jobs in the repo are used by the
			// Status and Stop methods on the traverser.
			t.jobRepo.Set(j.Id, realJob)
			defer t.jobRepo.Remove(j.Id) // remove the job from the repo

			// Bail out if the traverser was stopped. It is important that we check this
			// AFTER the job is added to the repo, because traverser.Stop only stops
			// jobs in the repo. If we do not add this extra check, the following
			// sequence would cause a problem: 1) job A gets created, 2) traverser.Stop
			// gets called, 3) job A gets added to the repo, 4) job A runs
			// unbounded even though we want to stop the traverser.
			select {
			case <-t.stopChan:
				j.State = proto.STATE_FAIL
				err = fmt.Errorf("chain was stopped before job started")
				return // all three defer funcs will be called
			default:
			}

			// Run the job. This is a blocking operation that could take a long time.
			jobRet, err = realJob.Run(j.Data)
			if err != nil {
				j.State = proto.STATE_FAIL
				err = fmt.Errorf("error running job: %s", err)
				return // all three defer funcs will be called
			}

			j.State = jobRet.State
			return // all three defer funcs will be called
		}(runJ)
	}
}

// sendJL takes all of the information related to the running of a job, and
// turns it into a jog log (JL) that gets sent to the Request Manager.
func (t *traverser) sendJL(job proto.Job, jobRet job.Return, runErr error, start, finish time.Time) {
	// Figure out what the error message in the JL should be. A run error
	// in the traverser takes precedence, followed by an error returned
	// in the job.Return struct from the job itself.
	var errMsg string
	if runErr != nil {
		errMsg = runErr.Error()
	} else if jobRet.Error != nil {
		errMsg = jobRet.Error.Error()
	}

	if runErr != nil || jobRet.Error != nil {
		log.Errorf("[chain=%s,job=%s]: Error running job (error: %s).",
			t.chain.RequestId(), job.Id, errMsg)
	}

	// Create the proto.JobLog.
	jl := proto.JobLog{
		RequestId:  t.chain.RequestId(),
		JobId:      job.Id,
		Type:       job.Type,
		StartedAt:  &start,
		FinishedAt: &finish,
		State:      job.State,
		Exit:       jobRet.Exit,
		Error:      errMsg,
		Stdout:     jobRet.Stdout,
		Stderr:     jobRet.Stderr,
	}

	// Send the JL to the RequestManager. If it fails, there's not much we
	// can do outside of logging the failure.
	err := t.rmClient.CreateJL(t.chain.RequestId(), jl)
	if err != nil {
		log.Errorf("[chain=%s,job=%s]: Error sending JL to the RM (error: %s).",
			t.chain.RequestId(), job.Id, err)
	}
}
