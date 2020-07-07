package cmd_test

import (
	"bytes"
	"testing"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/cmd"
	"github.com/square/spincycle/v2/spinc/config"
	"github.com/square/spincycle/v2/test/mock"
)

func TestStartSplitArgBug(t *testing.T) {
	ctx := app.Context{
		Out: &bytes.Buffer{},
		RMClient: &mock.RMClient{
			RequestListFunc: func() ([]proto.RequestSpec, error) {
				req := []proto.RequestSpec{{Name: "r1"}}
				return req, nil
			},
		},
		Options: config.Options{Debug: true},
		Command: config.Command{
			Cmd:  "start",
			Args: []string{"r1", "foo=a=b"}, // testing this
		},
	}
	start := cmd.NewStart(ctx)
	err := start.Prepare()
	if err == nil {
		t.Fatal("no error, expected ErrUnknownArgs")
	}
	switch v := err.(type) {
	case cmd.ErrUnknownArgs:
	default:
		t.Errorf("got error type %v, expected ErrUnknownArgs", v)
	}
}

func TestStartTestRequest(t *testing.T) {
	specs := []proto.RequestSpec{
		{
			Name: "test",
			Args: []proto.RequestArg{
				{
					Name: "foo",
					Desc: "foo is required",
					Type: proto.ARG_TYPE_REQUIRED,
				},
				{
					Name:    "bar",
					Desc:    "bar is optional",
					Default: "brr",
					Type:    proto.ARG_TYPE_OPTIONAL,
				},
			},
		},
	}
	ctx := app.Context{
		Out: &bytes.Buffer{},
		RMClient: &mock.RMClient{
			RequestListFunc: func() ([]proto.RequestSpec, error) {
				return specs, nil
			},
		},
		Options: config.Options{Debug: true},
		Command: config.Command{
			Cmd:  "start",
			Args: []string{"test", "foo=val"},
		},
	}
	start := cmd.NewStart(ctx)
	err := start.Prepare()
	if err != nil {
		t.Fatal(err)
	}

	// Optional values are given unless user explicitly gives a value
	expectCmd := "start test foo=val"
	gotCmd := start.Cmd()
	if expectCmd != gotCmd {
		t.Errorf("got cmd '%s', expected '%s'", gotCmd, expectCmd)
	}
}
