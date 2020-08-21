// Copyright 2020, Square, Inc.

package cmd_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/cmd"
	"github.com/square/spincycle/v2/spinc/config"
	"github.com/square/spincycle/v2/test/mock"
)

func TestFindPrepare1(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"user=owner", "since=2006-01-02 15:04:05 UTC",
		"until=2006-01-02 15:04:05 UTC", "limit=5", "offset=3"}

	command := config.Command{
		Args: args,
	}

	ctx := app.Context{
		Command: command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error in 'Prepare': %s", err)
	}
}

func TestFindPrepare2(t *testing.T) {
	args := []string{"type=requestname",
		"user=owner", "until=2006-01-02 15:04:05 UTC", "limit=5"}

	command := config.Command{
		Args: args,
	}

	ctx := app.Context{
		Command: command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error in 'Prepare': %s", err)
	}
}

func TestFailFindPrepare1(t *testing.T) {
	args := []string{"type", "states=RUNNING,PENDING,FAIL",
		"user=owner", "since=2006-01-02 15:04:05 UTC",
		"until=2006-01-02 15:04:05 UTC", "limit=5", "offset=3"}

	command := config.Command{
		Args: args,
	}

	ctx := app.Context{
		Command: command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err == nil {
		t.Fatal("No error in 'Prepare' with invalid input (no arg supplied)")
	}
	t.Log(err)
}

func TestFailFindPrepare2(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"user=owner", "since=2006-01-02",
		"until=2006-01-02 15:04:05 UTC", "limit=5", "offset=3"}

	command := config.Command{
		Args: args,
	}

	ctx := app.Context{
		Command: command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err == nil {
		t.Fatal("No error in 'Prepare' with invalid input (invalid date format)")
	}
	t.Log(err)
}

func TestFailFindPrepare3(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,not-a-state,FAIL",
		"user=owner", "since=2006-01-02 15:04:05 UTC",
		"until=2006-01-02 15:04:05 UTC", "limit=5", "offset=3"}

	command := config.Command{
		Args: args,
	}

	ctx := app.Context{
		Command: command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err == nil {
		t.Fatal("No error in 'Prepare' with invalid input (invalid state)")
	}
	t.Log(err)
}

func TestFindRun(t *testing.T) {
	ts, _ := time.Parse("2006-01-02 15:04:05", "2019-03-27 11:30:00")
	createdAt := ts
	startedAt := ts
	finishedAt := ts
	requests := []proto.Request{
		proto.Request{
			Id:    "b9uvdi8tk9kahl8ppvbg",
			Type:  "requestname",
			State: proto.STATE_RUNNING,
			User:  "owner",

			CreatedAt:  createdAt,
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,

			TotalJobs:    304,
			FinishedJobs: 68,

			JobRunnerURL: "http://localhost",
		},
		proto.Request{
			Id:    "b9uvdi8tk9kahl8ppvbh",
			Type:  "requestname",
			State: proto.STATE_RUNNING,
			User:  "owner",

			CreatedAt: createdAt,

			TotalJobs:    304,
			FinishedJobs: 68,

			JobRunnerURL: "http://localhost",
		},
	}

	output := &bytes.Buffer{}
	rmc := &mock.RMClient{
		FindRequestsFunc: func(proto.RequestFilter) ([]proto.Request, error) {
			return requests, nil
		},
	}
	command := config.Command{
		Args: []string{},
	}

	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command:  command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error in 'Prepare': %s", err)
	}
	err = find.Run()
	if err != nil {
		t.Fatalf("Unexpected error in 'Run': %s", err)
	}

	expectedOutput := `REQUEST              ID                   USER      STATE     CREATED                 STARTED                 FINISHED                JOBS            HOST
requestname          b9uvdi8tk9kahl8ppvbg owner     RUNNING   2019-03-27 11:30:00 UTC 2019-03-27 11:30:00 UTC 2019-03-27 11:30:00 UTC 68 / 304        http://localhost
requestname          b9uvdi8tk9kahl8ppvbh owner     RUNNING   2019-03-27 11:30:00 UTC N/A                     N/A                     68 / 304        http://localhost
`

	if output.String() != expectedOutput {
		t.Errorf("Wrong output:\nactual output:\n%s\nexpected:\n%s\n", output, expectedOutput)
	}
}

func TestFindRunNoRequests(t *testing.T) {
	requests := []proto.Request{}

	output := &bytes.Buffer{}
	rmc := &mock.RMClient{
		FindRequestsFunc: func(proto.RequestFilter) ([]proto.Request, error) {
			return requests, nil
		},
	}
	command := config.Command{
		Args: []string{},
	}

	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command:  command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err != nil {
		t.Fatalf("Unexpected error in 'Prepare': %s", err)
	}
	err = find.Run()
	if err != nil {
		t.Fatalf("Unexpected error in 'Run': %s", err)
	}

	expectedOutput := ""

	if output.String() != expectedOutput {
		t.Errorf("Wrong output:\nactual output:\n%s\nexpected:\n%s\n", output, expectedOutput)
	}
}
