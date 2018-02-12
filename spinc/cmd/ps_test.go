package cmd_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/cmd"
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
			},
		},
		Requests: map[string]proto.Request{
			"b9uvdi8tk9kahl8ppvbg": proto.Request{
				Id:        "b9uvdi8tk9kahl8ppvbg",
				TotalJobs: 9,
			},
		},
	}
	rmc := &mock.RMClient{
		SysStatRunningFunc: func() (proto.RunningStatus, error) {
			return status, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
	}
	ps := cmd.NewPs(ctx)
	err := ps.Run()
	if err != nil {
		t.Errorf("got err '%s', exepcted nil", err)
	}

	expectOutput := `ID                       N  NJOBS    TIME  JOB
b9uvdi8tk9kahl8ppvbg     1      9     3.0  jobname
`
	if output.String() != expectOutput {
		t.Errorf("got output:\n%s\nexpected:\n%s\n", output, expectOutput)
	}
}
