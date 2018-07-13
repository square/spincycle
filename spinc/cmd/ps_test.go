package cmd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/cmd"
	"github.com/square/spincycle/spinc/config"
	"github.com/square/spincycle/test/mock"
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
				N:         1,
				Status:    "jobstatus",
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:        "b9uvdi8tk9kahl8ppvbg",
				TotalJobs: 9,
				Type:      "requestname",
				User:      "owner",
			},
		},
	}
	args := make(map[string]interface{})
	args["key"] = "value"
	request := proto.Request{
		Id:     "b9uvdi8tk9kahl8ppvbg",
		Params: args,
	}
	rmc := &mock.RMClient{
		SysStatRunningFunc: func() (proto.RunningStatus, error) {
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
		Options: config.Options{
			Verbose: false,
		},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	expectOutput := `ID                  	   N	NJOBS	  TIME	OWNER	JOB	
b9uvdi8tk9kahl8ppvbg	   1	    9	   3.0	owner	jobname
`

	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
	}
}

func TestPsVerbose(t *testing.T) {
	output := &bytes.Buffer{}
	status := proto.RunningStatus{
		Jobs: []proto.JobStatus{
			{
				RequestId: "b9uvdi8tk9kahl8ppvbg",
				JobId:     "jid1",
				Type:      "jobtype",
				Name:      "jobname",
				StartedAt: time.Now().Add(-3 * time.Second).UnixNano(),
				N:         1,
				Status:    "jobstatus",
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:        "b9uvdi8tk9kahl8ppvbg",
				TotalJobs: 9,
				Type:      "requestname",
				User:      "owner",
			},
		},
	}
	args := make(map[string]interface{})
	args["key"] = "value"
	request := proto.Request{
		Id:     "b9uvdi8tk9kahl8ppvbg",
		Params: args,
	}
	rmc := &mock.RMClient{
		SysStatRunningFunc: func() (proto.RunningStatus, error) {
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
		Options: config.Options{
			Verbose: true,
		},
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	expectOutput := `ID                  	   N	NJOBS	  TIME	OWNER	JOB	REQUEST
b9uvdi8tk9kahl8ppvbg	   1	    9	   3.0	owner	jobname	requestname  key=value
`
	if output.String() != expectOutput {
		fmt.Printf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
	}
}
