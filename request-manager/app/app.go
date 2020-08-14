// Copyright 2017-2019, Square, Inc.

package app

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/square/spincycle/v2/config"
	jr "github.com/square/spincycle/v2/job-runner"
	"github.com/square/spincycle/v2/request-manager/auth"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/joblog"
	"github.com/square/spincycle/v2/request-manager/request"
	"github.com/square/spincycle/v2/request-manager/spec"
	"github.com/square/spincycle/v2/request-manager/status"
)

// Context represents the config, core service singletons, and 3rd-party extensions.
// There is one immutable context shared by many packages, created in Server.Boot,
// called api.appCtx.
type Context struct {
	// User-provided config from config file
	Config config.RequestManager
	Specs  spec.Specs

	// Core service singletons, not user-configurable
	RM     request.Manager
	RR     request.Resumer
	Status status.Manager
	Auth   auth.Manager
	JLS    joblog.Store

	// Closed to initiate RM shutdown
	ShutdownChan chan struct{}

	// 3rd-party extensions, all optional
	Hooks     Hooks
	Factories Factories
	Plugins   Plugins
}

// Factories make objects at runtime. All factories are optional; the defaults
// are sufficient to run the Request Manager. Users can provide custom factories
// to modify behavior. For example, make Job Runner clients with custom TLS certs.
type Factories struct {
	// MakeIDGeneratorFactory makes a factory of (U)ID generators for jobs within a
	// request. Generators should be able to generate at least as many IDs as jobs
	// in the largest possible request.
	MakeIDGeneratorFactory func(Context) (id.GeneratorFactory, error)

	// Makes list of check factories, which create checks run on request specs on
	// startup. Checks may appear in multiple factories, since checks should not modify
	// the specs at all.  spec.BaseCheckFactory is automatically included by caller
	// (and does not need to be included here).
	MakeCheckFactories func(Context) ([]spec.CheckFactory, error)

	MakeJobRunnerClient func(Context) (jr.Client, error)
	MakeDbConnPool      func(Context) (*sql.DB, error)
}

// Hooks allow users to modify system behavior at certain points. All hooks are
// optional; the defaults are sufficient to run the Request Manager. For example,
// the LoadConfig hook allows the user to load and parse the config file, completely
// overriding the built-in code.
type Hooks struct {
	// LoadConfig loads the Request Manager config file. This hook overrides
	// the default function. Spin Cycle fails to start if it returns an error.
	LoadConfig func(Context) (config.RequestManager, error)

	// LoadSpecs loads the request specification files (specs). This hook overrides
	// the default function. Spin Cycle fails to start if it returns an error.
	LoadSpecs func(Context) (spec.Specs, error)

	// SetUsername sets proto.Request.User. The auth.Plugin.Authenticate method is
	// called first which sets the username to Caller.Name. This hook is called after
	// and overrides the username. The request fails and returns HTTP 500 if it
	// returns an error.
	SetUsername func(*http.Request) (string, error)

	// RunAPI runs the Request Manager API. It should block until the API is
	// stopped via a call to StopAPI. If this hook is provided, it is called
	// instead of api.Run(). If you provide this hook, you need to provide StopAPI
	// as well.
	RunAPI func() error

	// StopAPI stops running the Request Manager API - it's called after RunAPI
	// when the Request Manager is shutting down, and it should cause RunAPI to
	// return. If you provide this hook, you need to provide RunAPI as well.
	StopAPI func() error
}

// Plugins allow users to provide custom components. All plugins are optional;
// the defaults are sufficient to run the Request Manager. Whereas hooks are single,
// specific calls, plugins are complete components with more extensive functionality
// defined by an interface. A user plugin, if provided, must implement the interface
// completely. For example, the Auth plugin allows the user to provide a complete
// and custom system of authentication and authorization.
type Plugins struct {
	Auth auth.Plugin
}

// Defaults returns a Context with default (built-in) 3rd-party extensions.
// The default context is not sufficient to run the Request Manager, but it
// provides the starting point for user customization by overriding the default
// hooks, factories, and plugins. See documentation for details.
//
// After customizing the default context, it is used to boot the server (see server
// package) which loads the configs and creates the core service singleton.
func Defaults() Context {
	return Context{
		ShutdownChan: make(chan struct{}),
		Factories: Factories{
			MakeIDGeneratorFactory: MakeIDGeneratorFactory,
			MakeCheckFactories:     MakeCheckFactories,
			MakeJobRunnerClient:    MakeJobRunnerClient,
			MakeDbConnPool:         MakeDbConnPool,
		},
		Hooks: Hooks{
			LoadConfig: LoadConfig,
			LoadSpecs:  LoadSpecs,
		},
		Plugins: Plugins{
			Auth: auth.AllowAll{},
		},
	}
}

// LoadConfig is the default LoadConfig hook. It loads the config file specified
// on the command line, or from config/ENVIRONMENT.yaml where ENVIRONMENT is an
// environment variable specifing "staging" or "production", else it defaults to
// "development".
func LoadConfig(ctx Context) (config.RequestManager, error) {
	cfg, _ := config.Defaults()
	if err := config.Load("", &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// LoadSpecs is the default LoadSpecs hook.
func LoadSpecs(ctx Context) (spec.Specs, error) {
	return spec.ParseSpecsDir(ctx.Config.Specs.Dir, log.Printf)
}

// MakeIDGeneratorFactory is the default MakeIDGeneratorFactory factory.
func MakeIDGeneratorFactory(ctx Context) (id.GeneratorFactory, error) {
	return id.NewGeneratorFactory(4, 100), nil // generates 4-character ids for jobs
}

// MakeCheckFactories is the default MakeCheckFactories factory.
func MakeCheckFactories(ctx Context) ([]spec.CheckFactory, error) {
	return []spec.CheckFactory{spec.DefaultCheckFactory{}}, nil
}

// MakeJobRunnerClient is the default MakeJobRunnerClient factory.
func MakeJobRunnerClient(ctx Context) (jr.Client, error) {
	httpClient := &http.Client{}
	jrcfg := ctx.Config.JRClient
	if jrcfg.TLS.CertFile != "" && jrcfg.TLS.KeyFile != "" && jrcfg.TLS.CAFile != "" {
		tlsConfig, err := config.NewTLSConfig(jrcfg.TLS.CAFile, jrcfg.TLS.CertFile, jrcfg.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading JR client TLS config: %s", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}
	}
	jrc := jr.NewClient(httpClient)
	return jrc, nil
}

// MakeDbConnPool is the default MakeDbConnPool factory.
func MakeDbConnPool(ctx Context) (*sql.DB, error) {
	// @todo: validate dsn
	dbcfg := ctx.Config.MySQL
	dsn := dbcfg.DSN + "?parseTime=true" // always needs to be set
	if dbcfg.TLS.CAFile != "" && dbcfg.TLS.CertFile != "" && dbcfg.TLS.KeyFile != "" {
		tlsConfig, err := config.NewTLSConfig(dbcfg.TLS.CAFile, dbcfg.TLS.CertFile, dbcfg.TLS.KeyFile)
		if err != nil {
			log.Fatalf("error loading database TLS config: %s", err)
		}
		mysql.RegisterTLSConfig("custom", tlsConfig)
		dsn += "&tls=custom"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error creating sql.DB: %s", err)
	}
	// @todo: make this configurable
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(12 * time.Hour)
	return db, nil
}
