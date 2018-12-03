// Copyright 2017-2018, Square, Inc.

// Package server bootstraps and runs the Request Manager.
package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/square/spincycle/request-manager/api"
	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/auth"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
)

type Server struct {
	appCtx  app.Context
	api     *api.API
	sigChan chan os.Signal
}

func NewServer(appCtx app.Context) *Server {
	return &Server{
		appCtx: appCtx,
	}
}

// Run runs the Request Manager API in the foreground. It returns when the API stops.
func (s *Server) Run() error {
	if s.api == nil {
		panic("Server.Run called before Server.Boot")
	}

	// Run the request resumer.
	resumerDone := make(chan struct{})
	go func() {
		defer close(resumerDone)

		// Every 10 seconds until shutdown, resume all SJCs and clean up any
		// SJCs in a bad state.
		ticker := time.NewTicker(10 * time.Second)
	RESUMER:
		for {
			select {
			case <-s.appCtx.ShutdownChan:
				break RESUMER
			case <-ticker.C:
				s.appCtx.RR.ResumeAll()
				s.appCtx.RR.Cleanup(1 * time.Hour)
			}
		}
		ticker.Stop()
	}()

	go s.waitForShutdown()

	// Run the RM API.
	err := s.api.Run()

	// If api returned because the RM is shutting down, wait for the resumer to
	// stop running as well.
	select {
	case <-s.appCtx.ShutdownChan:
		<-resumerDone
	default:
	}

	return err
}

// Catch TERM and INT signals to gracefully shut down the Request Manager
func (s *Server) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	close(s.appCtx.ShutdownChan)
}

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
	s.appCtx.RM = request.NewManager(grf, dbc, jrc, s.appCtx.ShutdownChan)

	// Request Resumer: suspend + resume requests
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting hostname: %s", err)
	}
	resumerConfig := request.ResumerConfig{
		RequestManager: s.appCtx.RM,
		DBConnector:    dbc,
		JRClient:       jrc,
		RMHost:         hostname,
		ShutdownChan:   s.appCtx.ShutdownChan,
	}
	s.appCtx.RR = request.NewResumer(resumerConfig)

	// Status: figure out request status using db and Job Runners (real-time)
	s.appCtx.Status = status.NewManager(dbc, jrc)

	// Job log store: save job log entries (JLE) from Job Runners
	s.appCtx.JLS = joblog.NewStore(dbc)

	// Auth Manager: request authorization (pre- (built-in) and post- using plugin)
	s.appCtx.Auth = auth.NewManager(s.appCtx.Plugins.Auth, MapACL(specs), cfg.Auth.AdminRoles, cfg.Auth.Strict)

	// API: endpoints and controllers, also handles auth via auth plugin
	s.api = api.NewAPI(s.appCtx)

	return nil
}

func (s *Server) API() *api.API {
	return s.api
}

// MapACL maps spec file ACL to auth.ACL structure.
func MapACL(specs grapher.Config) map[string][]auth.ACL {
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
