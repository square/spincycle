// Copyright 2020, Square, Inc.

package main

import (
	"os"

	"github.com/square/spincycle/v2/linter"
)

func main() {
	if ok := linter.Run(); !ok {
		os.Exit(1)
	}
}
