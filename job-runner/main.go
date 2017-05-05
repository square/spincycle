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
	// Make a chain repo.
	chainRepo := chain.NewMemoryRepo()
	// We could alternatively make a redis-backed chain repo with something
	// like the following:
	//
	// redisConf := chain.RedisRepoConfig{
	// 	Server:      "localhost",
	// 	Port:        6379,
	// 	Prefix:      "SpinCycle::ChainRepo",
	// 	IdleTimeout: 240 * time.Second,
	// }
	// chainRepo := chain.NewRedisRepo(redisConf)

	// Make the API
	runnerFactory := runner.NewRunnerFactory(external.JobFactory)
	api := api.NewAPI(&router.Router{}, chainRepo, runnerFactory)

	// Make an HTTP server using API
	h := http.NewServeMux()
	h.Handle("/api/", api.Router)

	// Listen and serve
	err := http.ListenAndServe(":9999", h)
	log.Fatal(err)
}
