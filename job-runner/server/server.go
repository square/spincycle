// Copyright 2017-2018, Square, Inc.

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

// Run runs the Job Runner API in the foreground. It returns when the API stops.
func Run(appCtx app.Context) error {
	if err := loadConfig(&appCtx); err != nil {
		return err
	}
	api, err := makeAPI(appCtx)
	if err != nil {
		return err
	}
	return api.Run()
}

type Server struct {
	appCtx app.Context
	api    *api.API
}

func NewServer(appCtx app.Context) *Server {
	return &Server{
		appCtx: appCtx,
	}
}

func (s *Server) Boot() error {
	if s.api != nil {
		return nil
	}
	if err := loadConfig(&s.appCtx); err != nil {
		return err
	}
	api, err := makeAPI(s.appCtx)
	if err != nil {
		return err
	}
	s.api = api
	return nil
}

func (s *Server) API() *api.API {
	return s.api
}

// --------------------------------------------------------------------------

func loadConfig(appCtx *app.Context) error {
	var err error
	var cfg config.JobRunner
	if appCtx.Hooks.LoadConfig != nil {
		cfg, err = appCtx.Hooks.LoadConfig(*appCtx)
	} else {
		cfg, err = appCtx.Hooks.LoadConfig(*appCtx)
	}
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}
	appCtx.Config = cfg
	return nil
}

func makeAPI(appCtx app.Context) (*api.API, error) {
	var err error

	// //////////////////////////////////////////////////////////////////////
	// Request Manager Client
	// //////////////////////////////////////////////////////////////////////
	rmc, err := appCtx.Factories.MakeRequestManagerClient(appCtx)
	if err != nil {
		return nil, fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Chain repo
	// //////////////////////////////////////////////////////////////////////
	chainRepo, err := appCtx.Factories.MakeChainRepo(appCtx)
	if err != nil {
		return nil, fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	stat := status.NewManager(chainRepo)
	rf := runner.NewFactory(jobs.Factory, rmc)
	trFactory := chain.NewTraverserFactory(chainRepo, rf, rmc)
	trRepo := cmap.New()

	return api.NewAPI(appCtx, trFactory, trRepo, stat), nil
}
