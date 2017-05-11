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
	// //////////////////////////////////////////////////////////////////////
	// Chain repo
	// //////////////////////////////////////////////////////////////////////
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
	chainRepo := chain.NewMemoryRepo()

	// //////////////////////////////////////////////////////////////////////
	// Runner factory and repo
	// //////////////////////////////////////////////////////////////////////
	runnerFactory := runner.NewFactory(external.JobFactory)
	runnerRepo := runner.NewRepo()

	// //////////////////////////////////////////////////////////////////////
	// Traverser repo and factory
	// //////////////////////////////////////////////////////////////////////
	trRepo := chain.NewTraverserRepo()
	trFactory := chain.NewTraverserFactory(chainRepo, runnerFactory, runnerRepo)

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	api := api.NewAPI(&router.Router{}, trFactory, trRepo)
	h := http.NewServeMux()
	h.Handle("/api/", api.Router)
	err := http.ListenAndServe(":9999", h)
	log.Fatal(err)
}
