// Copyright 2017-2018, Square, Inc.

package main

import (
	"log"

	"github.com/square/spincycle/v2/job-runner/app"
	"github.com/square/spincycle/v2/job-runner/server"
)

func main() {
	s := server.NewServer(app.Defaults())
	if err := s.Boot(); err != nil {
		log.Fatalf("Error starting Job Runner: %s", err)
	}
	err := s.Run(true)
	log.Fatalf("Job Runner stopped: %s", err)
}
