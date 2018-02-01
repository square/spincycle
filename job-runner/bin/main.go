// Copyright 2017, Square, Inc.

package main

import (
	"log"

	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/server"
)

func main() {
	err := server.Run(app.Defaults())
	log.Fatal("Job Runner stopped: %s", err)
}
