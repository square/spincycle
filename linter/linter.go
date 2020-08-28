// Copyright 2020, Square, Inc.

package linter

import (
	"fmt"

	"github.com/alexflint/go-arg"

	"github.com/square/spincycle/v2/linter/app"
	"github.com/square/spincycle/v2/request-manager/spec"
	"github.com/square/spincycle/v2/request-manager/template"
)

var cmd struct {
	SpecsDir string `arg:"positional,required" help:"path to spin cycle requests directory"`
}

func Run(ctx app.Context) error {
	/* Setup. */
	arg.MustParse(&cmd)
	printf := func(s string, args ...interface{}) { fmt.Printf(s+"\n", args...) }

	/* Static checks. */
	allSpecs, err := ctx.Hooks.LoadSpecs(cmd.SpecsDir, printf)
	if err != nil {
		return err
	}
	spec.ProcessSpecs(&allSpecs)

	checkFactories, err := ctx.Factories.MakeCheckFactories(allSpecs)
	if err != nil {
		return fmt.Errorf("MakeCheckFactories: %s", err)
	}
	checkFactories = append(checkFactories, spec.BaseCheckFactory{allSpecs})
	checker, err := spec.NewChecker(checkFactories, printf)
	if err != nil {
		return err
	}
	ok := checker.RunChecks(allSpecs)
	if !ok {
		return fmt.Errorf("static check failed") // checker prints details for us
	}

	/* Graph checks. */
	idgen, err := ctx.Factories.MakeIDGeneratorFactory()
	if err != nil {
		return fmt.Errorf("MakeIDGeneratorFactory: %s", err)
	}
	templateG := template.NewGrapher(allSpecs, idgen, printf)
	_, ok = templateG.CreateTemplates()
	if !ok {
		return fmt.Errorf("graph check failed") // grapher prints details for us
	}

	return nil
}
