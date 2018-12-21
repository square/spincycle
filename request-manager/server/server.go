// Copyright 2017-2018, Square, Inc.

// Package server bootstraps and runs the Request Manager.
package server

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/square/spincycle/request-manager/api"
	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/auth"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
)

var (
	// How often the request resumer is run.
	ResumerInterval = 10 * time.Second

	// How long Suspended Job Chains have to be resumed before they're deleted.
	SJCTTL = 1 * time.Hour
)

type Server struct {
	appCtx app.Context
	api    *api.API

	shutdownChan   chan struct{}
	resumerStopped chan struct{}
	apiStopped     chan struct{}
	stopped        bool
	stopMux        sync.Mutex
}

func NewServer(appCtx app.Context) *Server {
	return &Server{
		appCtx:         appCtx,
		resumerStopped: make(chan struct{}),
		apiStopped:     make(chan struct{}),
		shutdownChan:   make(chan struct{}),
		stopMux:        sync.Mutex{},
	}
}

// Run runs the Request Manager API and Request Resumer. It returns when the API
// stops running (either from an error, or after a call to Stop). If a custom RunAPI
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

	// Run the request resumer in a goroutine, so we can block on running the api.
	go func() {
		defer close(s.resumerStopped) // indicate the resumer is done running

		// Every 10 seconds until the server is stopped, resume all Suspended Job
		// Chains and clean up any that are in a bad state.
		ticker := time.NewTicker(ResumerInterval)
	RESUMER:
		for {
			select {
			case <-s.shutdownChan:
				break RESUMER
			case <-ticker.C:
				s.appCtx.RR.ResumeAll()
				s.appCtx.RR.Cleanup()
			}
		}
		ticker.Stop()
	}()

	// If stopOnSignal = true, watch for TERM + INT signals from the OS and shut
	// down the Request Manager when we receive them.
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
	// to make sure it's done shutting down the API and resumer before returning.
	if s.stopped {
		<-s.apiStopped
		<-s.resumerStopped
	}

	if err != nil {
		return fmt.Errorf("error from API: %s", err)
	}
	return nil
}

// Stop stops the running Request Resumer and API. It signals the resumer to shut
// down and then stops the API (using either the default api.Stop or the StopAPI
// hook if provided). Once Stop has been called, the server cannot be reused -
// future calls to Run will return an error.
//
// If stopOnSignal was set when calling Run, Stop will automatically be called by
// the server on receiving a TERM or INT signal from the OS. Otherwise, you must
// call Stop when you want to shut down the Request Manager.
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

	log.Infof("Stopping Request Manager server")

	// Stops the request resumer loop. The API will also begin refusing to start
	// running new requests.
	close(s.shutdownChan)

	// Stop the API, using the StopAPI hook if provided and api.Stop otherwise.
	var err error
	if s.appCtx.Hooks.StopAPI != nil {
		err = s.appCtx.Hooks.StopAPI()
	} else {
		err = s.api.Stop()
	}
	close(s.apiStopped) // indicate to Run that the API is done shutting down

	// Wait to return until the resumer has been stopped.
	<-s.resumerStopped

	if err != nil {
		return fmt.Errorf("error stopping API: %s", err)
	}
	return nil
}

// Boot sets up the server. It must be called before calling Run.
func (s *Server) Boot() error {
	// Load config file
	cfg, err := s.appCtx.Hooks.LoadConfig(s.appCtx)
	if err != nil {
		return fmt.Errorf("error loading config: %s", err)
	}
	s.appCtx.Config = cfg

	// Load requests specification files (specs)
	specs, err := s.appCtx.Hooks.LoadSpecs(s.appCtx)
	if err != nil {
		return fmt.Errorf("error loading specs: %s", err)
	}
	s.appCtx.Specs = specs

	// Grapher: load, parse, and validate specs. Done only once on startup.
	grf, err := s.appCtx.Factories.MakeGrapher(s.appCtx)
	if err != nil {
		return fmt.Errorf("MakeGrapher: %s", err)
	}

	// Job Runner Client: how the Request Manager talks to Job Runners
	jrc, err := s.appCtx.Factories.MakeJobRunnerClient(s.appCtx)
	if err != nil {
		return fmt.Errorf("MakeJobRunnerClient: %s", err)
	}

	// Db connection pool: for requests, job chains, etc. (pretty much everything)
	dbc, err := s.appCtx.Factories.MakeDbConnPool(s.appCtx)
	if err != nil {
		return fmt.Errorf("MakeDbConnPool: %s", err)
	}

	// Request Manager: core logic and coordination
	s.appCtx.RM = request.NewManager(grf, dbc, jrc, s.shutdownChan)

	// Request Resumer: suspend + resume requests
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting hostname: %s", err)
	}
	resumerConfig := request.ResumerConfig{
		RequestManager:       s.appCtx.RM,
		DBConnector:          dbc,
		JRClient:             jrc,
		RMHost:               hostname,
		ShutdownChan:         s.shutdownChan,
		SuspendedJobChainTTL: SJCTTL,
	}
	s.appCtx.RR = request.NewResumer(resumerConfig)

	// Status: figure out request status using db and Job Runners (real-time)
	s.appCtx.Status = status.NewManager(dbc, jrc)

	// Job log store: save job log entries (JLE) from Job Runners
	s.appCtx.JLS = joblog.NewStore(dbc)

	// Auth Manager: request authorization (pre- (built-in) and post- using plugin)
	s.appCtx.Auth = auth.NewManager(s.appCtx.Plugins.Auth, mapACL(specs), cfg.Auth.AdminRoles, cfg.Auth.Strict)

	// API: endpoints and controllers, also handles auth via auth plugin
	s.api = api.NewAPI(s.appCtx)

	return nil
}

// API returns the Request Manager API created in Boot.
func (s *Server) API() *api.API {
	return s.api
}

// --------------------------------------------------------------------------

// Catch TERM and INT signals to gracefully shut down the Request Manager
func (s *Server) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	err := s.Stop()
	if err != nil {
		log.Errorf("error shutting down server: %s", err)
	}
}

// MapACL maps spec file ACL to auth.ACL structure.
func mapACL(specs grapher.Config) map[string][]auth.ACL {
	acl := map[string][]auth.ACL{}
	for name, spec := range specs.Sequences {
		if len(spec.ACL) == 0 {
			acl[name] = nil
			continue
		}
		acl[name] = make([]auth.ACL, len(spec.ACL))
		for i, sa := range spec.ACL {
			acl[name][i] = auth.ACL{
				Role:  sa.Role,
				Admin: sa.Admin,
				Ops:   sa.Ops,
			}
		}
	}
	return acl
}
