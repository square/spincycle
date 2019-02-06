package cmd_test

import (
	"bytes"
	"testing"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/cmd"
	"github.com/square/spincycle/spinc/config"
	"github.com/square/spincycle/test/mock"
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
