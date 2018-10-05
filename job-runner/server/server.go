// Copyright 2017-2018, Square, Inc.

// Package server bootstraps the Job Runner.
package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/orcaman/concurrent-map"

	"github.com/square/spincycle/config"
	"github.com/square/spincycle/job-runner/api"
	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/job-runner/status"
	"github.com/square/spincycle/jobs"
)

type Server struct {
	appCtx       app.Context
	api          *api.API
	shutdownChan chan struct{}
}

func NewServer(appCtx app.Context) *Server {
	return &Server{
		appCtx:       appCtx,
		shutdownChan: make(chan struct{}),
	}
}

// Run runs the Job Runner API in the foreground. It returns when the API stops.
func (s *Server) Run() error {
	if err := s.Boot(); err != nil {
		return err
	}
	go s.waitForShutdown()
	return s.api.Run() // returns after API is shut down
}

func (s *Server) Boot() error {
	if s.api != nil {
		return nil
	}
	if err := s.loadConfig(); err != nil {
		return err
	}
	if err := s.makeAPI(); err != nil {
		return err
	}
	return nil
}

func (s *Server) API() *api.API {
	return s.api
}

// --------------------------------------------------------------------------

// Catch TERM and INT signals to gracefully shut down the Job Runner
func (s *Server) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	// API + traversers watch shutdownChan
	close(s.shutdownChan)
}

func (s *Server) loadConfig() error {
	var err error
	var cfg config.JobRunner
	if s.appCtx.Hooks.LoadConfig != nil {
		cfg, err = s.appCtx.Hooks.LoadConfig(s.appCtx)
	} else {
		cfg, err = s.appCtx.Hooks.LoadConfig(s.appCtx)
	}
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}
	s.appCtx.Config = cfg
	return nil
}

func (s *Server) makeAPI() error {
	var err error

	// //////////////////////////////////////////////////////////////////////
	// Request Manager Client
	// //////////////////////////////////////////////////////////////////////
	rmc, err := s.appCtx.Factories.MakeRequestManagerClient(s.appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Chain repo
	// //////////////////////////////////////////////////////////////////////
	chainRepo, err := s.appCtx.Factories.MakeChainRepo(s.appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	stat := status.NewManager(chainRepo)
	rf := runner.NewFactory(jobs.Factory, rmc)
	trFactory := chain.NewTraverserFactory(chainRepo, rf, rmc, s.shutdownChan)
	trRepo := cmap.New()

	s.api = api.NewAPI(s.appCtx, trFactory, trRepo, stat, s.shutdownChan)
	return nil
}
