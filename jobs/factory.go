// Copyright 2018, Square, Inc.

// Package jobs implements all SpinCycle jobs and a factory to create them. This package
// is intended to be overwritten with a custom jobs package with factory and job
// implementations that make sense for your domain. For an example jobs package, look at
// the package github.com/square/spincycle/dev/jobs.
package jobs

import (
	"github.com/square/spincycle/job"
)

// Factory is a package variable referenced from other modules. It is the main entry point
// to domain-specific jobs. Refer to the job.Factory type for usage documentation.
var Factory job.Factory = empty{}

type empty struct{}

func (empty) Make(_ job.Id) (job.Job, error) {
	panic("replace the spincycle jobs package with your own implementation")
}
