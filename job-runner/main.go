// Copyright 2017, Square, Inc.

package main

import (
	"log"
	"net/http"

	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/cache"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/job/external"
)

func main() {
	runnerFactory := &runner.RealRunnerFactory{
		JobFactory: external.JobFactory,
	}

	h := http.NewServeMux()
	app.New(&app.Config{
		HTTPServer:    h,
		RunnerFactory: runnerFactory,
		ChainRepo:     &chain.FakeRepo{},
		Cache:         cache.NewLocalCache(),
	})

	err := http.ListenAndServe(":9999", h)
	log.Fatal(err)
}
