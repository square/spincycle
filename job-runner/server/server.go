// Copyright 2017, Square, Inc.

// Package server bootstraps the Job Runner.
package server

import (
	"fmt"

	"github.com/orcaman/concurrent-map"

	"github.com/square/spincycle/config"
	"github.com/square/spincycle/job-runner/api"
	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/job-runner/status"
	"github.com/square/spincycle/jobs"
)

func Run(appCtx app.Context) error {
	var err error

	// //////////////////////////////////////////////////////////////////////
	// Config
	// //////////////////////////////////////////////////////////////////////
	var cfg config.JobRunner
	if appCtx.Hooks.LoadConfig != nil {
		cfg, err = appCtx.Hooks.LoadConfig(appCtx)
	} else {
		cfg, err = appCtx.Hooks.LoadConfig(appCtx)
	}
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}
	appCtx.Config = cfg

	// //////////////////////////////////////////////////////////////////////
	// Request Manager Client
	// //////////////////////////////////////////////////////////////////////
	rmc, err := appCtx.Factories.MakeRequestManagerClient(appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Chain repo
	// //////////////////////////////////////////////////////////////////////
	chainRepo, err := appCtx.Factories.MakeChainRepo(appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	stat := status.NewManager(chainRepo)
	rf := runner.NewFactory(jobs.Factory, rmc)
	trFactory := chain.NewTraverserFactory(chainRepo, rf, rmc)
	trRepo := cmap.New()
	api := api.NewAPI(appCtx, trFactory, trRepo, stat)

	return api.Run()
}
