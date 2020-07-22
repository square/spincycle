// Copyright 2017-2019, Square, Inc.

package main

import (
	"fmt"
	"os"

	"github.com/square/spincycle/v2/linter"
	"github.com/square/spincycle/v2/linter/app"
)

func main() {
	defaultContext := app.Context{
		Hooks: app.Hooks{},
	}
	if err := linter.Run(defaultContext); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("No errors")
}
