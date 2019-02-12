// Copyright 2017-2019, Square, Inc.

package app

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/square/spincycle/config"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/jobs"
	"github.com/square/spincycle/request-manager/auth"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
	"github.com/square/spincycle/util"
)

// Context represents the config, core service singletons, and 3rd-party extensions.
// There is one immutable context shared by many packages, created in Server.Boot,
// called api.appCtx.
type Context struct {
	// User-provided config from config file
	Config config.RequestManager
	Specs  grapher.Config

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
	MakeGrapher         func(Context) (grapher.GrapherFactory, error) // @fixme
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
	LoadSpecs func(Context) (grapher.Config, error)

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
			MakeGrapher:         MakeGrapher,
			MakeJobRunnerClient: MakeJobRunnerClient,
			MakeDbConnPool:      MakeDbConnPool,
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
	var cfgFile string
	if len(os.Args) > 1 {
		cfgFile = os.Args[1]
	} else {
		switch os.Getenv("ENVIRONMENT") {
		case "staging":
			cfgFile = "config/staging.yaml"
		case "production":
			cfgFile = "config/production.yaml"
		default:
			cfgFile = "config/development.yaml"
		}
	}
	var cfg config.RequestManager
	err := config.Load(cfgFile, &cfg)
	return cfg, err
}

// LoadSpecs is the default LoadSpecs hook.
func LoadSpecs(ctx Context) (grapher.Config, error) {
	specs := grapher.Config{
		Sequences: map[string]*grapher.SequenceSpec{},
	}
	// For each config in the cfg.SpecFileDir directory, read the file and
	// then aggregate all of the resulting configs into a single struct.
	specFiles, err := ioutil.ReadDir(ctx.Config.SpecFileDir)
	if err != nil {
		return specs, err
	}
	for _, f := range specFiles {
		spec, err := grapher.ReadConfig(filepath.Join(ctx.Config.SpecFileDir, f.Name()))
		if err != nil {
			return specs, fmt.Errorf("error reading spec file %s: %s", f.Name(), err)
		}
		for name, spec := range spec.Sequences {
			specs.Sequences[name] = spec
		}
	}
	return specs, nil
}

// MakeGrapher is the default MakeGrapher factory.
func MakeGrapher(ctx Context) (grapher.GrapherFactory, error) {
	idf := id.NewGeneratorFactory(4, 100) // generate 4-character ids for jobs
	grf := grapher.NewGrapherFactory(jobs.Factory, ctx.Specs, idf)
	return grf, nil
}

// MakeJobRunnerClient is the default MakeJobRunnerClient factory.
func MakeJobRunnerClient(ctx Context) (jr.Client, error) {
	httpClient := &http.Client{}
	jrcfg := ctx.Config.JRClient
	if jrcfg.TLS.CertFile != "" && jrcfg.TLS.KeyFile != "" && jrcfg.TLS.CAFile != "" {
		tlsConfig, err := util.NewTLSConfig(jrcfg.TLS.CAFile, jrcfg.TLS.CertFile, jrcfg.TLS.KeyFile)
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
	dbcfg := ctx.Config.Db
	dsn := dbcfg.DSN + "?parseTime=true" // always needs to be set
	if dbcfg.TLS.CAFile != "" && dbcfg.TLS.CertFile != "" && dbcfg.TLS.KeyFile != "" {
		tlsConfig, err := util.NewTLSConfig(dbcfg.TLS.CAFile, dbcfg.TLS.CertFile, dbcfg.TLS.KeyFile)
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
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(12 * time.Hour)
	return db, nil
}
