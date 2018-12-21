// Copyright 2017-2018, Square, Inc.

// Package server bootstraps and runs the Job Runner.
package server

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
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
	appCtx        app.Context
	api           *api.API
	traverserRepo cmap.ConcurrentMap

	shutdownChan chan struct{}
	apiStopped   chan struct{}
	stopMux      sync.Mutex
	stopped      bool
}

func NewServer(appCtx app.Context) *Server {
	return &Server{
		appCtx:       appCtx,
		stopMux:      sync.Mutex{},
		apiStopped:   make(chan struct{}),
		shutdownChan: make(chan struct{}),
	}
}

// Run runs the Job Runner API in the foreground. It returns when the API stops
// running (either from an error, or after a call to Stop). If a custom RunAPI
// hook has been provided, it will be called to run the API instead of the default
// api.Run.
//
// If stopOnSignal = true, the server will listen for TERM and INT signals from the
// OS and call Stop to shut itself down when those signals are received. Else, the
// caller must call Stop to shut down the server.
func (s *Server) Run(stopOnSignal bool) error {
	if s.api == nil {
		panic("Server.Run called before Server.Boot")
	}
	if s.stopped {
		return fmt.Errorf("server stopped")
	}

	// If stopOnSignal = true, watch for TERM + INT signals from the OS and shut
	// down the Job Runner when we receive them.
	if stopOnSignal {
		go s.waitForShutdown()
	}

	// Run the API - this will block until the API is stopped (or encounters
	// some fatal error). If the RunAPI hook has been provided, call that instead
	// of the default api.Run.
	var err error
	if s.appCtx.Hooks.RunAPI != nil {
		err = s.appCtx.Hooks.RunAPI()
	} else {
		err = s.api.Run()
	}

	// If the server was stopped (as opposed to some error within the API), wait
	// to make sure it's done shutting down the API before returning.
	if s.stopped {
		<-s.apiStopped
	}

	if err != nil {
		return fmt.Errorf("error from API: %s", err)
	}
	return nil
}

// Boot sets up the server. It must be called before calling Run.
func (s *Server) Boot() error {
	// Only run Boot once.
	if s.api != nil {
		return nil
	}

	// Either both or neither RunAPI and StopAPI hooks must be provided - can't
	// have just one.
	if (s.appCtx.Hooks.RunAPI == nil) != (s.appCtx.Hooks.StopAPI == nil) {
		return fmt.Errorf("Only one of RunAPI and StopAPI hooks provided - either both or neither must be provided.")
	}
	if err := s.loadConfig(); err != nil {
		return err
	}
	if err := s.makeAPI(); err != nil {
		return err
	}
	return nil
}

// Stop stops the server. It signals running traversers to shut down and then
// stops the API (using either the default api.Stop or the StopAPI hook if
// provided). Once Stop has been called, the server cannot be reused - future calls
// to Run will return an error.
//
// If stopOnSignal was set when calling Run, Stop will automatically be called by
// the server on receiving a TERM or INT signal from the OS. Otherwise, you must
// call Stop when you want to shut down the Job Runner.
func (s *Server) Stop() error {
	// Only stop once. We lock the whole Stop call, so that, if Stop is called
	// multiple times in quick succession, no calls will return before the server
	// has actually been shut down.
	s.stopMux.Lock()
	defer s.stopMux.Unlock()
	if s.stopped {
		return nil
	}
	s.stopped = true

	log.Infof("Stopping Job Runner server")

	// Running traversers watch shutdownChan - closing this tells them to shut down.
	// The API will also begin refusing to start running new job chains.
	close(s.shutdownChan)

	// Wait for all traversers to shut down. Timeout if they aren't done
	// within 20 seconds, and continue to shutting down the API.
	timeout := time.After(20 * time.Second)
WAIT_FOR_TRAVERSERS:
	for !s.traverserRepo.IsEmpty() {
		select {
		case <-time.After(10 * time.Millisecond):
			// Check again if traversers are all done.
		case <-timeout:
			break WAIT_FOR_TRAVERSERS
		}
	}

	// Stop the API, using the StopAPI hook if provided and api.Stop otherwise.
	var err error
	if s.appCtx.Hooks.StopAPI != nil {
		err = s.appCtx.Hooks.StopAPI()
	} else {
		err = s.api.Stop()
	}
	close(s.apiStopped) // indicate to Run that the API is done shutting down

	if err != nil {
		return fmt.Errorf("error stopping API: %s", err)
	}
	return nil
}

// API returns the Job Runner API created in Boot.
func (s *Server) API() *api.API {
	return s.api
}

// --------------------------------------------------------------------------

// Catch TERM and INT signals to gracefully shut down the Job Runner
func (s *Server) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	err := s.Stop()
	if err != nil {
		log.Errorf("error shutting down server: %s", err)
	}
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
	s.traverserRepo = cmap.New()

	apiCfg := api.Config{
		AppCtx:           s.appCtx,
		TraverserFactory: trFactory,
		TraverserRepo:    s.traverserRepo,
		StatusManager:    stat,
		ShutdownChan:     s.shutdownChan,
	}
	s.api = api.NewAPI(apiCfg)
	return nil
}
