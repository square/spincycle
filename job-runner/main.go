// Copyright 2017, Square, Inc.

package main

import (
	"log"
	"net/http"

	"github.com/square/spincycle/job-runner/api"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/job/external"
	"github.com/square/spincycle/router"
)

func main() {
	// Make the API
	runnerFactory := &runner.RealRunnerFactory{
		JobFactory: external.JobFactory,
	}
	chainRepo := chain.NewMemoryRepo()
	api := api.NewAPI(&router.Router{}, chainRepo, runnerFactory)

	// Make an HTTP server using API
	h := http.NewServeMux()
	h.Handle("/api/", api.Router)

	// Listen and serve
	err := http.ListenAndServe(":9999", h)
	log.Fatal(err)
}
