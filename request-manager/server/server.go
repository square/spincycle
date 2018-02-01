// Copyright 2017, Square, Inc.

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
	var err error

	// //////////////////////////////////////////////////////////////////////
	// Config
	// //////////////////////////////////////////////////////////////////////
	var cfg config.RequestManager
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
	// Grapher Factory
	// //////////////////////////////////////////////////////////////////////
	grf, err := appCtx.Factories.MakeGrapher(appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Job Runner Client
	// //////////////////////////////////////////////////////////////////////
	jrc, err := appCtx.Factories.MakeJobRunnerClient(appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
	}

	// //////////////////////////////////////////////////////////////////////
	// DB Connection Pool
	// //////////////////////////////////////////////////////////////////////
	dbc, err := appCtx.Factories.MakeDbConnPool(appCtx)
	if err != nil {
		return fmt.Errorf("error loading config at %s", err)
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
	api := api.NewAPI(appCtx, rm, jls, stat)

	return api.Run()
}
