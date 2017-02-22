// Copyright 2017, Square, Inc.

package app

import (
	"net/http"

	"github.com/square/spincycle/job-runner/api"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/db"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/router"
)

// External "view".
type App struct {
	api *api.API
}

func (app *App) Router() *router.Router {
	return app.api.Router
}

// Incoming config object.
type Config struct {
	HTTPServer    *http.ServeMux
	RunnerFactory *runner.RealRunnerFactory
	ChainRepo     chain.Repo
	Cache         db.Driver
}

func New(cfg *Config) *App {
	api := api.NewAPI(&router.Router{}, cfg.ChainRepo, cfg.RunnerFactory, cfg.Cache)

	// If an http server is provided, register the API Router.
	if cfg.HTTPServer != nil {
		cfg.HTTPServer.Handle("/api/", api.Router)
	}

	return &App{api}
}
