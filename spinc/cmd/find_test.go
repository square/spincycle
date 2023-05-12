// Copyright 2020, Square, Inc.

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

func TestFindPrepare1(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"args=arg1=val1,arg2=val2",
		"user=owner", "since=2006-01-02 15:04:05 UTC",
		"until=2006-01-02 15:04:05 UTC", "limit=5", "offset=3", "timezone=local"}

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
		"user=owner", "until=2006-01-02 15:04:05 UTC", "limit=5", "timezone=utc"}

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

func TestFailFindPrepare4(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"user=owner", "since=2006-01-02 15:04:05 PST",
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
		t.Fatal("No error in 'Prepare' with invalid input (date must be in UTC)")
	}
	t.Log(err)
}

func TestFailFindPrepare5(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"user=owner", "since=2006-01-02 15:04:05 UTC",
		"until=2006-01-02 15:04:05 UTC", "limit=5", "offset=3", "timezone=pst"}

	command := config.Command{
		Args: args,
	}

	ctx := app.Context{
		Command: command,
	}

	find := cmd.NewFind(ctx)
	err := find.Prepare()
	if err == nil {
		t.Fatalf("No error in 'Prepare' with invalid input (invalid timezone)")
	}
}

func TestFailFindPrepare6(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"user=owner", "since=2006-01-02 15:04:05 UTC", "invalidarg=value",
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
		t.Fatalf("No error in 'Prepare' with invalid input (invalid arg)")
	}
}

func TestFailFindPrepare7(t *testing.T) {
	args := []string{"type=requestname", "states=RUNNING,PENDING,FAIL",
		"args=arg1",
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
		t.Fatalf("No error in 'Prepare' with invalid input (invalid args input)")
	}
}

func TestFindRunUTC(t *testing.T) {
	tsutc := "2020-08-02 15:00:00 UTC"
	ts, _ := time.Parse("2006-01-02 15:04:05 MST", tsutc)
	tslocal := ts.UTC().Format("2006-01-02 15:04:05 MST")
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
		},
		proto.Request{
			Id:    "b9uvdi8tk9kahl8ppvbh",
			Type:  "requestname",
			State: proto.STATE_RUNNING,
			User:  "owner",

			CreatedAt: createdAt,

			TotalJobs:    304,
			FinishedJobs: 68,
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
		Options:  config.Options{Debug: true},
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

	expectedOutput := fmt.Sprintf(`ID                   REQUEST                                  USER             STATE     CREATED                 STARTED                 FINISHED                JOBS
b9uvdi8tk9kahl8ppvbg requestname                              owner            RUNNING   %s %s %s 68 / 304
b9uvdi8tk9kahl8ppvbh requestname                              owner            RUNNING   %s N/A                     N/A                     68 / 304
`, tslocal, tslocal, tslocal, tslocal)

	if output.String() != expectedOutput {
		t.Errorf("Wrong output:\nactual output:\n%s\nexpected:\n%s\n", output, expectedOutput)
	}
}

func TestFindRunLocal(t *testing.T) {
	tsutc := "2020-08-02 15:00:00 UTC"
	ts, _ := time.Parse("2006-01-02 15:04:05 MST", tsutc)
	tslocal := ts.Local().Format("2006-01-02 15:04:05 MST")
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
		},
		proto.Request{
			Id:    "b9uvdi8tk9kahl8ppvbh",
			Type:  "requestname",
			State: proto.STATE_RUNNING,
			User:  "owner",

			CreatedAt: createdAt,

			TotalJobs:    304,
			FinishedJobs: 68,
		},
	}

	output := &bytes.Buffer{}
	rmc := &mock.RMClient{
		FindRequestsFunc: func(proto.RequestFilter) ([]proto.Request, error) {
			return requests, nil
		},
	}
	command := config.Command{
		Args: []string{"timezone=local"},
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

	expectedOutput := fmt.Sprintf(`ID                   REQUEST                                  USER             STATE     CREATED                 STARTED                 FINISHED                JOBS
b9uvdi8tk9kahl8ppvbg requestname                              owner            RUNNING   %s %s %s 68 / 304
b9uvdi8tk9kahl8ppvbh requestname                              owner            RUNNING   %s N/A                     N/A                     68 / 304
`, tslocal, tslocal, tslocal, tslocal)

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
