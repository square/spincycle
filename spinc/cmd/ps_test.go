// Copyright 2017-2019, Square, Inc.

package cmd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/cmd"
	"github.com/square/spincycle/v2/spinc/config"
	"github.com/square/spincycle/v2/test/mock"
)

func TestPs(t *testing.T) {
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "jobname",
				StartedAt: time.Now().Add(-3 * time.Second).UnixNano(),
				Status:    "jobstatus",
				Try:       1,
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:           "b9uvdi8tk9kahl8ppvbg",
				TotalJobs:    9,
				Type:         "requestname",
				User:         "owner",
				FinishedJobs: 1,
			},
		},
	}
	request := proto.Request{
		Id:   "b9uvdi8tk9kahl8ppvbg",
		Args: args,
	}
	rmc := &mock.RMClient{
		RunningFunc: func(f proto.StatusFilter) (proto.RunningStatus, error) {
			return status, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			if strings.Compare(id, "b9uvdi8tk9kahl8ppvbg") == 0 {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options:  config.Options{},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}
	expectOutput := `REQUEST              ID                    PRG  USER      RUNTIME  TRY JOB                    STATUS
requestname          b9uvdi8tk9kahl8ppvbg  11%  owner     3s         1 jobname                jobstatus
`

	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestPsVerbose(t *testing.T) {
	// ps -v was removed. This test is just normal output, slightly different than
	// previous test.
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "jobname",
				StartedAt: time.Now().Add(-3 * time.Second).UnixNano(),
				Status:    "jobstatus",
				Try:       2,
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:           "b9uvdi8tk9kahl8ppvbg",
				TotalJobs:    9,
				Type:         "requestname",
				User:         "owner",
				FinishedJobs: 2,
			},
		},
	}
	request := proto.Request{
		Id:   "b9uvdi8tk9kahl8ppvbg",
		Args: args,
	}
	rmc := &mock.RMClient{
		RunningFunc: func(f proto.StatusFilter) (proto.RunningStatus, error) {
			return status, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			if strings.Compare(id, "b9uvdi8tk9kahl8ppvbg") == 0 {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options:  config.Options{},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	// There's a trailing space after "val2 ". Only required args in list order
	// because that matches spec order.
	expectOutput := `REQUEST              ID                    PRG  USER      RUNTIME  TRY JOB                    STATUS
requestname          b9uvdi8tk9kahl8ppvbg  22%  owner     3s         2 jobname                jobstatus
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestPsLongNames(t *testing.T) {
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "this-is-a-pretty-long-job-name",
				StartedAt: time.Now().Add(-65 * time.Minute).UnixNano(),
				Status:    "but job status has no length so we should see this whole string, nothing truncated...",
				Try:       999,
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:           "b9uvdi8tk9kahl8ppvbg",
				TotalJobs:    9,
				Type:         "this-is-a-rather-long-request-name",
				User:         "michael.engineering.manager.finch",
				FinishedJobs: 9,
			},
		},
	}
	request := proto.Request{
		Id:   "b9uvdi8tk9kahl8ppvbg",
		Args: args,
	}
	rmc := &mock.RMClient{
		RunningFunc: func(f proto.StatusFilter) (proto.RunningStatus, error) {
			return status, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			if strings.Compare(id, "b9uvdi8tk9kahl8ppvbg") == 0 {
				return request, nil
			}
			return proto.Request{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Options:  config.Options{},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	expectOutput := `REQUEST              ID                    PRG  USER      RUNTIME  TRY JOB                    STATUS
this-is-a..uest-name b9uvdi8tk9kahl8ppvbg 100%  mich..nch 1h5m0s   999 this-is-a-..g-job-name but job status has no length so we should see this whole string, nothing truncated...
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
		t.Error("wrong output, see above")
	}
}

func TestPsFilterByRequestId(t *testing.T) {
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "jobname",
				StartedAt: time.Now().Add(-3 * time.Second).UnixNano(),
				Status:    "jobstatus",
				Try:       1,
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:           "b9uvdi8tk9kahl8ppvbg",
				TotalJobs:    9,
				Type:         "requestname",
				User:         "owner",
				FinishedJobs: 1,
			},
		},
	}
	request := proto.Request{
		Id:   "b9uvdi8tk9kahl8ppvbg",
		Args: args,
	}
	var gotFilter proto.StatusFilter
	rmc := &mock.RMClient{
		RunningFunc: func(f proto.StatusFilter) (proto.RunningStatus, error) {
			gotFilter = f
			return status, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			if strings.Compare(id, "b9uvdi8tk9kahl8ppvbg") == 0 {
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
			Cmd:  "ps",
			Args: []string{request.Id},
		},
	}
	ps := cmd.NewPs(ctx)

	// Must call Prepare, that's where ps checks/saves the optional request ID arg
	err := ps.Prepare()
	if err != nil {
		t.Fatal(err)
	}

	// Then it uses the optional request ID arg in Run
	err = ps.Run()
	if err != nil {
		t.Fatal(err)
	}
	if gotFilter.RequestId != request.Id {
		t.Errorf("RequestId not set in filter")
	}
}
