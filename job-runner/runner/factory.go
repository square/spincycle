// Copyright 2017-2019, Square, Inc.

package runner

import (
	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	rm "github.com/square/spincycle/request-manager"
)

// A Factory makes a Runner for one job. There are two try counts: prevTries and
// totalTries. prevTries is a gauge from [0, 1+retry], where retry is the retry
// count from the request spec. The prevTries count is per-sequence try, which is
// why it can reset to zero on sequence retry (handled by a chain.Reaper).
// On suspend/resume, jobs are stopped and the try on which it's stopped doesn't count,
// so prevTries is decremented by 1 on resume to retry. The totalTries count is
// a monotonically increasing global counter of how many times the job was run.
// This count is used for the proto.JobLog.Try field which cannot repeat a number
// because the job_log table primary key is <request_id, job_id, try>.
type Factory interface {
	Make(job proto.Job, requestId string, prevTries, totalTries uint) (Runner, error)
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
func (f *factory) Make(pJob proto.Job, requestId string, prevTries, totalTries uint) (Runner, error) {
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
	return NewRunner(pJob, realJob, requestId, prevTries, totalTries, f.rmc), nil
}
