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
	Make(job proto.Job, requestId string, prevTries uint, prevSeqTries uint, sequenceRetry uint) (Runner, error)
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
func (f *factory) Make(pJob proto.Job, requestId string, prevTries uint, prevSeqTries uint, sequenceRetry uint) (Runner, error) {
	// Subtract the number of times the job was already tried within this
	// sequence try (i.e. before being stopped). Used when resuming a
	// previously stopped job.
	pJob.Retry -= prevSeqTries

	// Instantiate a "blank" job of the given type.
	realJob, err := f.jf.Make(job.NewIdWithRequestId(pJob.Type, pJob.Name, pJob.Id, requestId))
	if err != nil {
		return nil, err
	}

	// Have the job re-create itself so it's no longer blank but rather
	// what it was when first created in the Request Manager
	if err := realJob.Deserialize(pJob.Bytes); err != nil {
		return nil, err
	}

	// Job should be ready to run. Create and return a runner for it.
	return NewRunner(pJob, realJob, requestId, prevTries, sequenceRetry, f.rmc), nil
}
