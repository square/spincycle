// Copyright 2020, Square, Inc.

package linter

import (
	"fmt"

	"github.com/alexflint/go-arg"

	"github.com/square/spincycle/v2/linter/app"
	"github.com/square/spincycle/v2/request-manager/spec"
)

var cmd struct {
	SpecsDir string `arg:"positional,required" help:"path to spin cycle requests directory"`
}

func Run(ctx app.Context) error {
	arg.MustParse(&cmd)

	printf := func(s string, args ...interface{}) { fmt.Printf(s, args...) }

	var specs spec.Specs
	var err error
	if ctx.Hooks.LoadSpecs == nil {
		specs, err = spec.Parse(cmd.SpecsDir, printf)
	} else {
		specs, err = ctx.Hooks.LoadSpecs(cmd.SpecsDir, printf)
	}
	if err != nil {
		return err
	}

	err = spec.RunChecks(specs, printf)
	if err != nil {
		return err
	}

	return nil
}
