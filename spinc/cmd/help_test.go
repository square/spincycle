// Copyright 2019, Square, Inc.

package cmd_test

import (
	"bytes"
	//	"strings"
	"testing"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/cmd"
	"github.com/square/spincycle/v2/spinc/config"
	"github.com/square/spincycle/v2/test/mock"
)

func TestHelpRequestHelp(t *testing.T) {
	// spinc help req
	out := &bytes.Buffer{}
	ctx := app.Context{
		Out: out,
		Factories: app.Factories{
			Command: &cmd.DefaultFactory{},
		},
		RMClient: &mock.RMClient{
			RequestListFunc: func() ([]proto.RequestSpec, error) {
				req := []proto.RequestSpec{
					{
						Name: "req1",
						Args: []proto.RequestArg{
							{
								Name: "foo",
								Desc: "foo arg",
								Type: "required",
							},
							{
								Name:    "bar",
								Desc:    "bar arg",
								Type:    "optional",
								Default: "abc",
							},
						},
					},
				}
				return req, nil
			},
		},
		Options: config.Options{
			Addr: "http://localhost",
		},
		Command: config.Command{
			Cmd:  "help",
			Args: []string{"req1"},
		},
	}
	help := cmd.NewHelp(ctx)
	err := help.Prepare()
	if err != nil {
		t.Error(err)
	}
	err = help.Run()
	if err != app.ErrHelp {
		t.Errorf("got error '%v', expected app.ErrHelp", err)
	}

	expectOutput := `Request Manager address: http://localhost

req1 request args (* required)

  * foo  foo arg
    bar  bar arg (default: abc)

To start a req1 request, run 'spinc start req1'
`
	gotOutput := out.String()
	if gotOutput != expectOutput {
		t.Logf("   got: %s", gotOutput)
		t.Logf("expect: %s", expectOutput)
		t.Error("output not correct, see above")
	}
}
