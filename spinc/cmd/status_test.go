// Copyright 2019, Square, Inc.

package cmd_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/cmd"
	"github.com/square/spincycle/v2/spinc/config"
	"github.com/square/spincycle/v2/test/mock"
)

func TestStatusRunning(t *testing.T) {
	output := &bytes.Buffer{}
	createdAt := time.Now().Add(-5 * time.Second)
	startedAt := time.Now().Add(-5 * time.Second)
	request := proto.Request{
		Id:           "b9uvdi8tk9kahl8ppvbg",
		Type:         "requestname",
		State:        proto.STATE_RUNNING,
		User:         "owner",
		Args:         args,
		TotalJobs:    9,
		FinishedJobs: 1,
		CreatedAt:    createdAt,
		StartedAt:    &startedAt,
	}
	rmc := &mock.RMClient{
		GetRequestFunc: func(id string) (proto.Request, error) {
			if id == request.Id {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options:  config.Options{},
		Command: config.Command{
			Cmd:  "status",
			Args: []string{request.Id},
		},
	}
	status := cmd.NewStatus(ctx)

	err := status.Prepare()
	if err != nil {
		t.Error(err)
	}

	err = status.Run()
	if err != nil {
		t.Error(err)
	}

	expectOutput := `   state: RUNNING
progress: 11%
 runtime: 5s
 request: requestname
  caller: owner
    args: key=value key2=val2
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestStatusFinished(t *testing.T) {
	output := &bytes.Buffer{}
	createdAt := time.Now().Add(-120 * time.Minute)
	startedAt := time.Now().Add(-120 * time.Minute)
	finishedAt := time.Now().Add(-1 * time.Second)
	request := proto.Request{
		Id:           "b9uvdi8tk9kahl8ppvbg",
		Type:         "requestname",
		State:        proto.STATE_COMPLETE,
		User:         "owner",
		Args:         args,
		TotalJobs:    9,
		FinishedJobs: 9,
		CreatedAt:    createdAt,
		StartedAt:    &startedAt,
		FinishedAt:   &finishedAt,
	}
	rmc := &mock.RMClient{
		GetRequestFunc: func(id string) (proto.Request, error) {
			if id == request.Id {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options:  config.Options{},
		Command: config.Command{
			Cmd:  "status",
			Args: []string{request.Id},
		},
	}
	status := cmd.NewStatus(ctx)

	err := status.Prepare()
	if err != nil {
		t.Error(err)
	}

	err = status.Run()
	if err != nil {
		t.Error(err)
	}

	expectOutput := `   state: COMPLETE
progress: 100%
 runtime: 1h59m59s
 request: requestname
  caller: owner
    args: key=value key2=val2
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestStatusArgValueQuoting(t *testing.T) {
	var args []proto.RequestArg = []proto.RequestArg{
		{
			Name:  "a",
			Type:  proto.ARG_TYPE_REQUIRED,
			Value: "foo bar",
		},
		{
			Name:  "b",
			Type:  proto.ARG_TYPE_REQUIRED,
			Value: `has "inner" quote`,
		},
	}
	output := &bytes.Buffer{}
	createdAt := time.Now().Add(-5 * time.Second)
	startedAt := time.Now().Add(-5 * time.Second)
	request := proto.Request{
		Id:           "b9uvdi8tk9kahl8ppvbg",
		Type:         "requestname",
		State:        proto.STATE_RUNNING,
		User:         "owner",
		Args:         args,
		TotalJobs:    9,
		FinishedJobs: 1,
		CreatedAt:    createdAt,
		StartedAt:    &startedAt,
	}
	rmc := &mock.RMClient{
		GetRequestFunc: func(id string) (proto.Request, error) {
			if id == request.Id {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options:  config.Options{},
		Command: config.Command{
			Cmd:  "status",
			Args: []string{request.Id},
		},
	}
	status := cmd.NewStatus(ctx)

	err := status.Prepare()
	if err != nil {
		t.Error(err)
	}

	err = status.Run()
	if err != nil {
		t.Error(err)
	}

	expectOutput := `   state: RUNNING
progress: 11%
 runtime: 5s
 request: requestname
  caller: owner
    args: a="foo bar" b="has \"inner\" quote"
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}
