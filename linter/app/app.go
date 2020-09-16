// Copyright 2020, Square, Inc.

// Package app provides app-wide data structs and functions.
package app

import (
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Context represents how to run linter. A context is passed to linter.Run().
// A default context is created in main.go. Wrapper code can integrate with
// linter by passing a custom context to linter.Run(). Integration is done
// primarily with hooks and factories.
type Context struct {
	// for integration with other code
	Factories Factories
	Hooks     Hooks
}

type Factories struct {
	// MakeIDGeneratorFactory makes a factory of (U)ID generators for nodes within a
	// sequence. Generators should be able to generate at least as many IDs as nodes
	// in the largest sequence.
	MakeIDGeneratorFactory func() (id.GeneratorFactory, error)

	// Makes list of check factories, which create checks run on request specs in
	// linter. Checks may appear in multiple factories, since checks should not modify
	// the specs at all.  spec.BaseCheckFactory is automatically included by caller
	// (and does not need to be included here).
	MakeCheckFactories func(allSpecs spec.Specs) ([]spec.CheckFactory, error)
}

type Hooks struct {
	// Parse specs
	LoadSpecs func(specsDir string) (specs spec.Specs, fileErrors, fileWarnings map[string][]error, traversalError error)
}

func Defaults() Context {
	return Context{
		Factories: Factories{
			MakeIDGeneratorFactory: MakeIDGeneratorFactory,
			MakeCheckFactories:     MakeCheckFactories,
		},
		Hooks: Hooks{
			LoadSpecs: spec.ParseSpecsDir,
		},
	}
}

// MakeIDGeneratorFactory is the default MakeIDGeneratorFactory factory.
func MakeIDGeneratorFactory() (id.GeneratorFactory, error) {
	return id.NewGeneratorFactory(4, 100), nil // generates 4-character ids for jobs
}

// MakeCheckFactories is the default MakeCheckFactories factory.
func MakeCheckFactories(allSpecs spec.Specs) ([]spec.CheckFactory, error) {
	return []spec.CheckFactory{spec.DefaultCheckFactory{allSpecs}}, nil
}
