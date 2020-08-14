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
	GeneratorFactory id.GeneratorFactory
	CheckFactories   []spec.CheckFactory // All additional check factories to run
}

type Hooks struct {
	LoadSpecs func(specsDir string, logFunc func(string, ...interface{})) (spec.Specs, error)
}

func Defaults() Context {
	return Context{
		Factories: Factories{
			GeneratorFactory: id.NewGeneratorFactory(4, 100),
			CheckFactories:   []spec.CheckFactory{spec.DefaultCheckFactory{}},
		},
		Hooks: Hooks{
			LoadSpecs: spec.ParseSpecsDir,
		},
	}
}
