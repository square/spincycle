// Copyright 2017, Square, Inc.

// Package external provides the hook for your job factory. See the source
// code for the single line to modify. After setting your job factory, Spin Cycle
// must be rebuilt.
package external

// Change only this important path, but do not change "myJobs". For example,
// if your jobs are in "github.com/my/jobs", change the line below to:
//
//   import myJobs "github.com/my/jobs"
//
// Then rebuild Spin Cycle.
import myJobs "github.com/square/spincycle/job/example"

// Do not change these lines
import "github.com/square/spincycle/job"    // do not change
var JobFactory job.Factory = myJobs.Factory // do not change
