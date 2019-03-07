// Copyright 2019, Square, Inc.

package spinc_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/square/spincycle/spinc"
	"github.com/square/spincycle/spinc/app"
)

func TestArgsNoCommand(t *testing.T) {
	ctx := app.Context{
		In:        os.Stdin,
		Out:       &bytes.Buffer{},
		Hooks:     app.Hooks{},
		Factories: app.Factories{},
	}
	os.Args = []string{"spinc", "--env", "staging", "--addr", "http://localhost"}
	err := spinc.Run(ctx)
	if err != app.ErrHelp {
		t.Errorf("got error '%v', expected ErrHelp", err)
	}
}
