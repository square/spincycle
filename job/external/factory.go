// Copyright 2017, Square, Inc.

package external

// Change the important path, but do not change "myJobs"
import myJobs "github.com/square/spincycle/job/internal"

// Do not change these lines
import "github.com/square/spincycle/job"    // do not change
var JobFactory job.Factory = myJobs.Factory // do not change
