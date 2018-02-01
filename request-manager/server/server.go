// Copyright 2017-2018, Square, Inc.

// Package server bootstraps the Request Manager.
package server

import (
	"fmt"

	"github.com/square/spincycle/config"
	"github.com/square/spincycle/request-manager/api"
	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
)

// Run runs the Request Manager API in the foreground. It returns when the API stops.
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

func NewServer(appCtx app.Context) Server {
	return Server{
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
	var cfg config.RequestManager
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
	// Grapher Factory
	// //////////////////////////////////////////////////////////////////////
	grf, err := appCtx.Factories.MakeGrapher(appCtx)
	if err != nil {
		return nil, fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Job Runner Client
	// //////////////////////////////////////////////////////////////////////
	jrc, err := appCtx.Factories.MakeJobRunnerClient(appCtx)
	if err != nil {
		return nil, fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// DB Connection Pool
	// //////////////////////////////////////////////////////////////////////
	dbc, err := appCtx.Factories.MakeDbConnPool(appCtx)
	if err != nil {
		return nil, fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Request Manager, Job Log Store, and Job Chain Store
	// //////////////////////////////////////////////////////////////////////
	rm := request.NewManager(grf, dbc, jrc)

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	stat := status.NewManager(dbc, jrc)
	jls := joblog.NewStore(dbc)

	return api.NewAPI(appCtx, rm, jls, stat), nil
}
