// Copyright 2017-2019, Square, Inc.

package main

import (
	"fmt"
	"os"

	"github.com/square/spincycle/v2/spinc"
	"github.com/square/spincycle/v2/spinc/app"
)

func main() {
	defaultContext := app.Context{
		In:        os.Stdin,
		Out:       os.Stdout,
		Hooks:     app.Hooks{},
		Factories: app.Factories{},
	}
	if err := spinc.Run(defaultContext); err != nil {
		if err != app.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
