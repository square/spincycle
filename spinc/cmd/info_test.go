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

func TestInfoRunning(t *testing.T) {
	output := &bytes.Buffer{}
	ts, _ := time.Parse("2006-01-02 15:04:05", "2019-03-27 11:30:00")
	ago := time.Now().Sub(ts).Round(time.Second)
	request := proto.Request{
		Id:           "b9uvdi8tk9kahl8ppvbg",
		Type:         "requestname",
		State:        proto.STATE_RUNNING,
		User:         "owner",
		Args:         args,
		TotalJobs:    9,
		FinishedJobs: 1,
		CreatedAt:    ts,
		StartedAt:    &ts,
		JobRunnerURL: "http://localhost",
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
			Cmd:  "info",
			Args: []string{request.Id},
		},
	}
	info := cmd.NewInfo(ctx)

	err := info.Prepare()
	if err != nil {
		t.Error(err)
	}

	err = info.Run()
	if err != nil {
		t.Error(err)
	}

	expectOutput := fmt.Sprintf(`      id: b9uvdi8tk9kahl8ppvbg
 request: requestname
  caller: owner
 created: 2019-03-27 11:30:00 UTC (%s ago)
 started: 2019-03-27 11:30:00 UTC (%s ago)
finished: 
   state: RUNNING
    host: http://localhost
    jobs: 9 (1 complete)
    args: key=value key2=val2 opt=not-shown
`, ago, ago)
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestInfoFinished(t *testing.T) {
	output := &bytes.Buffer{}
	ts, _ := time.Parse("2006-01-02 15:04:05", "2019-03-27 11:30:00")
	ago := time.Now().Sub(ts).Round(time.Second)

	finished, _ := time.Parse("2006-01-02 15:04:05", "2019-03-27 12:00:59")
	finishedAgo := time.Now().Sub(finished).Round(time.Second)

	request := proto.Request{
		Id:           "b9uvdi8tk9kahl8ppvbg",
		Type:         "requestname",
		State:        proto.STATE_COMPLETE,
		User:         "owner",
		Args:         args,
		TotalJobs:    9,
		FinishedJobs: 9,
		CreatedAt:    ts,
		StartedAt:    &ts,
		FinishedAt:   &finished,
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
			Cmd:  "info",
			Args: []string{request.Id},
		},
	}
	info := cmd.NewInfo(ctx)

	err := info.Prepare()
	if err != nil {
		t.Error(err)
	}

	err = info.Run()
	if err != nil {
		t.Error(err)
	}

	expectOutput := fmt.Sprintf(`      id: b9uvdi8tk9kahl8ppvbg
 request: requestname
  caller: owner
 created: 2019-03-27 11:30:00 UTC (%s ago)
 started: 2019-03-27 11:30:00 UTC (%s ago)
finished: 2019-03-27 12:00:59 UTC (%s ago)
   state: COMPLETE
    host: 
    jobs: 9 (9 complete)
    args: key=value key2=val2 opt=not-shown
`, ago, ago, finishedAgo)
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}
