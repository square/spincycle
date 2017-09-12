// Copyright 2017, Square, Inc.

package main

import (
	"os"

	"github.com/square/spincycle/spinc"
	"github.com/square/spincycle/spinc/app"
)

func main() {
	defaultContext := app.Context{
		In:        os.Stdin,
		Out:       os.Stdout,
		Hooks:     app.Hooks{},
		Factories: app.Factories{},
	}
	spinc.Run(defaultContext)
}
