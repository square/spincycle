// Copyright 2017, Square, Inc.

// Package internal provides an example job and job factory. The job type is
// "shell-command", and the factory only build this job type. By default, this
// factory is imported in ../external/factory.go, which allows Spin Cycle to be
// build without external jobs.
package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"sync"

	"github.com/square/spincycle/job"
)

// Factory is a job.Factory that makes "shell-command" type jobs.
var Factory job.Factory = factory{}

type factory struct {
}

// Make makes a job of the given type, with the given name. This factory only
// makes "shell-command" type jobs. If jobType is any other value, a job.ErrUnknownJobType
// error is returned.
func (f factory) Make(jobType, jobName string) (job.Job, error) {
	switch jobType {
	case "shell-command":
		return NewShellCommand(jobName), nil
	}
	return nil, job.ErrUnknownJobType
}

// ShellCommand is a job.Job that runs a single shell command with arguments.
type ShellCommand struct {
	// Internal data (serialized)
	Cmd  string   // command to execute
	Args []string // args to cmd

	// While running
	status string
	*sync.RWMutex

	// Meta
	jobName string
	jobType string
}

// NewShellCommand instantiates a new ShellCommand job. This should only be called
// by the Factory. jobName must be unique within a job chain.
func NewShellCommand(jobName string) *ShellCommand {
	return &ShellCommand{
		jobName: jobName,
		jobType: "shell-command",
		RWMutex: &sync.RWMutex{},
	}
}

// Create is a job.Job interface method.
func (j *ShellCommand) Create(jobArgs map[string]string) error {
	cmd := jobArgs[j.jobName+"_cmd"]
	if cmd == "" {
		return errors.New(j.jobName + "_cmd is not set")
	}
	j.Cmd = cmd

	args := jobArgs[j.jobName+"_args"]
	if args != "" {
		j.Args = strings.Split(args, ",")
	}

	return nil
}

// Serialize is a job.Job interface method.
func (j *ShellCommand) Serialize() ([]byte, error) {
	return json.Marshal(j)
}

// Deserialize is a job.Job interface method.
func (j *ShellCommand) Deserialize(bytes []byte) error {
	var d ShellCommand
	if err := json.Unmarshal(bytes, &d); err != nil {
		return err
	}
	j.Cmd = d.Cmd
	j.Args = d.Args
	j.setStatus("ready to run")
	return nil
}

// Run is a job.Job interface method.
func (j *ShellCommand) Run(jobData map[string]interface{}) (job.Return, error) {
	// Set status before and after
	j.setStatus("runnning " + j.Cmd)
	defer j.setStatus("done running " + j.Cmd)

	// Create the cmd to run
	cmd := exec.Command(j.Cmd, j.Args...)

	// Capture STDOUT and STDERR
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Run the cmd and wait for it to return
	exit := int64(0)
	err := cmd.Run()
	if err != nil {
		exit = 1
	}

	ret := job.Return{
		Exit:   exit,
		Error:  err,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	return ret, nil
}

// Stop is a job.Job interface method.
func (j *ShellCommand) Stop() error {
	return nil
}

// Status is a job.Job interface method.
func (j *ShellCommand) Status() string {
	j.RLock()
	defer j.RUnlock()
	return j.status
}

// Name is a job.Job interface method.
func (j *ShellCommand) Name() string {
	return j.jobName
}

// Type is a job.Job interface method.
func (j *ShellCommand) Type() string {
	return j.jobType
}

// setStatus is a private method, not a job.Job interface method.
func (j *ShellCommand) setStatus(msg string) {
	j.Lock()
	defer j.Unlock()
	j.status = msg
}
