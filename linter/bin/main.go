// Copyright 2019, Square, Inc.

package main

import (
	"os"

	"github.com/square/spincycle/v2/linter"
	"github.com/square/spincycle/v2/linter/app"
)

func main() {
	defaultContext := app.Defaults()
	if ok := linter.Run(defaultContext); !ok {
		os.Exit(1)
	}
}
