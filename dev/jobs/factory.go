// Copyright 2017-2018, Square, Inc.

// Package example provides an example job and job factory. The job type is
// "shell-command", and the factory only build this job type. By default, this

///
// factory is imported in ../external/factory.go, which allows Spin Cycle to be
// build without external jobs.
package jobs

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"

	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
)

// Factory is a job.Factory that makes "shell-command" type jobs.
var Factory job.Factory = factory{}

type factory struct {
}

// Make makes a job of the given type, with the given name. This factory only
// makes "shell-command" type jobs. If jobType is any other value, a job.ErrUnknownJobType
// error is returned.
func (f factory) Make(jid job.Id) (job.Job, error) {
	switch jid.Type {
	case "noop":
		return NewNop(jid), nil
	case "shell-command":
		return NewShellCommand(jid), nil
	}
	return nil, job.ErrUnknownJobType
}

// ShellCommand is a job.Job that runs a single shell command with arguments.
type ShellCommand struct {
	// Internal data (serialized)
	Cmd  string   `json:"cmd"`            // command to execute
	Args []string `json:"args,omitempty"` // args to cmd

	// While running
	status string
	*sync.RWMutex

	// Meta
	id job.Id
}

// NewShellCommand instantiates a new ShellCommand job. This should only be called
// by the Factory. jobName must be unique within a job chain.
func NewShellCommand(jid job.Id) *ShellCommand {
	return &ShellCommand{
		id:      jid,
		RWMutex: &sync.RWMutex{},
	}
}

// Create is a job.Job interface method.
func (j *ShellCommand) Create(jobArgs map[string]interface{}) error {
	cmd, ok := jobArgs["cmd"]
	if !ok {
		return job.ErrArgNotSet{"cmd"}
	}
	j.Cmd = cmd.(string)

	args, ok := jobArgs["args"]
	if ok {
		j.Args = strings.Fields(args.(string))
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
	ret := job.Return{
		Exit:   exit,
		Error:  err,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		ret.Exit = 1
		ret.State = proto.STATE_FAIL
	} else {
		ret.State = proto.STATE_COMPLETE
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
func (j *ShellCommand) Id() job.Id {
	return j.id
}

// setStatus is a private method, not a job.Job interface method.
func (j *ShellCommand) setStatus(msg string) {
	j.Lock()
	defer j.Unlock()
	j.status = msg
}

// Nop is a no-op job that does nothing and always returns success. It's used in
// place of jobs that we want to include in a job chain but haven't implemented yet.
type Nop struct {
	id job.Id
}

func NewNop(jid job.Id) *Nop {
	n := &Nop{
		id: jid,
	}
	return n
}

func (j *Nop) Create(jobArgs map[string]interface{}) error {
	return nil
}

func (j *Nop) Serialize() ([]byte, error) {
	return nil, nil
}

func (j *Nop) Deserialize(bytes []byte) error {
	return nil
}

func (j *Nop) Run(jobData map[string]interface{}) (job.Return, error) {
	ret := job.Return{
		Exit:   0,
		Error:  nil,
		Stdout: "",
		Stderr: "",
		State:  proto.STATE_COMPLETE,
	}
	return ret, nil
}

func (j *Nop) Status() string {
	return "nop"
}

func (j *Nop) Stop() error {
	return nil
}

func (j *Nop) Id() job.Id {
	return j.id
}
