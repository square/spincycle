// Copyright 2017-2018, Square, Inc.

package runner

import (
	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
)

// A Factory takes a proto.Job, creates a corresponding job.Job interface for
// it, and passes both to NewRunner to make a Runner.
type Factory interface {
	Make(job proto.Job, requestId string, prevTryNo uint, triesToSkip uint, sequenceRetry uint) (Runner, error)
}
type factory struct {
	jf  job.Factory
	rmc rm.Client
}

// NewRunnerFactory makes a RunnerFactory.
func NewFactory(jf job.Factory, rmc rm.Client) Factory {
	return &factory{
		jf:  jf,
		rmc: rmc,
	}
}

// Make a runner for a new job.
func (f *factory) Make(pJob proto.Job, requestId string, prevTryNo uint, triesToSkip uint, sequenceRetry uint) (Runner, error) {
	// Remove a number of retries from the job. Useful when resuming a
	// previously stopped job.
	pJob.Retry -= triesToSkip

	// Instantiate a "blank" job of the given type.
	realJob, err := f.jf.Make(job.NewId(pJob.Type, pJob.Name, pJob.Id))
	if err != nil {
		return nil, err
	}

	// Have the job re-create itself so it's no longer blank but rather
	// what it was when first created in the Request Manager
	if err := realJob.Deserialize(pJob.Bytes); err != nil {
		return nil, err
	}

	// Job should be ready to run. Create and return a runner for it.
	return NewRunner(pJob, realJob, requestId, prevTryNo, sequenceRetry, f.rmc), nil
}
