// Copyright 2020, Square, Inc.

// Package app provides app-wide data structs and functions.
package app

import (
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Context represents how to run linter. A context is passed to linter.Run().
// A default context is created in main.go. Wrapper code can integrate with
// linter by passing a custom context to linter.Run(). Integration is done
// primarily with hooks.
type Context struct {
	Hooks Hooks // for integration with other code
}

type Hooks struct {
	LoadSpecs func(string, func(string, ...interface{})) (spec.Specs, error)
}
