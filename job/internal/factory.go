// Copyright 2017, Square, Inc.

package internal

import (
	"github.com/square/spincycle/job"
)

var Factory job.Factory = factory{}

type factory struct {
}

func (f factory) Make(jobType, jobName string) (job.Job, error) {
	switch jobType {
	case "shell-command":
		return NewShellCommand(jobName), nil
	}
	return nil, job.ErrUnknownJobType
}
