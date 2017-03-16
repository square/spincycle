// Copyright 2017, Square, Inc.

package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"sync"

	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
)

// //////////////////////////////////////////////////////////////////////////
// ShellCommand
// //////////////////////////////////////////////////////////////////////////

// ShellCommand runs a command with arguments.
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

func NewShellCommand(jobName string) *ShellCommand {
	return &ShellCommand{
		jobName: jobName,
		jobType: "shell-command",
		RWMutex: &sync.RWMutex{},
	}
}

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

func (j *ShellCommand) Serialize() ([]byte, error) {
	return json.Marshal(j)
}

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

func (j *ShellCommand) Run(jobData map[string]interface{}) job.Return {
	// Set status before and after
	j.setStatus("runnning " + j.Cmd)
	defer j.setStatus("done running " + j.Cmd)
	var ret job.Return

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
		ret = job.Return{
			State:  proto.STATE_FAIL,
			Exit:   1,
			Error:  err,
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}
	}

	ret = job.Return{
		State:  proto.STATE_COMPLETE,
		Exit:   exit,
		Error:  err,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	return ret
}

func (j *ShellCommand) Stop() error {
	return nil
}

func (j *ShellCommand) Status() string {
	j.RLock()
	defer j.RUnlock()
	return j.status
}

func (j *ShellCommand) Name() string {
	return j.jobName
}

func (j *ShellCommand) Type() string {
	return j.jobType
}

func (j *ShellCommand) setStatus(msg string) {
	j.Lock()
	defer j.Unlock()
	j.status = msg
}
