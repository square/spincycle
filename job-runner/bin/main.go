// Copyright 2017-2018, Square, Inc.

package main

import (
	"log"

	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/server"
)

func main() {
	s := server.NewServer(app.Defaults())
	err := s.Run()
	log.Fatalf("Job Runner stopped: %s", err)
}
