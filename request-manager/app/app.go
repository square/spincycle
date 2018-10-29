// Copyright 2017-2018, Square, Inc.

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

	myconn "github.com/go-mysql/conn"
	"github.com/go-sql-driver/mysql"

	"github.com/square/spincycle/config"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/jobs"
	"github.com/square/spincycle/request-manager/auth"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/util"
)

type Context struct {
	Hooks     Hooks
	Factories Factories
	Plugins   Plugins

	Config config.RequestManager
}

type Factories struct {
	MakeGrapher         func(Context) (grapher.GrapherFactory, error) // @fixme
	MakeJobRunnerClient func(Context) (jr.Client, error)
	MakeDbConnPool      func(Context) (myconn.Connector, error)
}

type Hooks struct {
	LoadConfig  func(Context) (config.RequestManager, error)
	SetUsername func(*http.Request) (string, error)
}

type Plugins struct {
	Auth auth.Auth
}

func Defaults() Context {
	return Context{
		Factories: Factories{
			MakeGrapher:         MakeGrapher,
			MakeJobRunnerClient: MakeJobRunnerClient,
			MakeDbConnPool:      MakeDbConnPool,
		},
		Hooks: Hooks{
			LoadConfig: LoadConfig,
			SetUsername: (func(ireq *http.Request) (string, error) {
				return "admin", nil
			}),
		},
		Plugins: Plugins{
			Auth: auth.AllowAll{},
		},
	}
}

func LoadConfig(ctx Context) (config.RequestManager, error) {
	var cfgFile string
	if len(os.Args) > 1 {
		cfgFile = os.Args[1]
	} else {
		switch os.Getenv("ENVIRONMENT") {
		case "staging":
			cfgFile = "config/staging.yaml"
		case "production":
			cfgFile = "config/staging.yaml"
		default:
			cfgFile = "config/development.yaml"
		}
	}
	var cfg config.RequestManager
	err := config.Load(cfgFile, &cfg)
	return cfg, err
}

func MakeGrapher(ctx Context) (grapher.GrapherFactory, error) {
	specs := grapher.Config{
		Sequences: map[string]*grapher.SequenceSpec{},
	}
	// For each config in the cfg.SpecFileDir directory, read the file and
	// then aggregate all of the resulting configs into a single struct.
	specFiles, err := ioutil.ReadDir(ctx.Config.SpecFileDir)
	if err != nil {
		return nil, err
	}
	for _, f := range specFiles {
		spec, err := grapher.ReadConfig(filepath.Join(ctx.Config.SpecFileDir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("error reading spec file %s: %s", f.Name(), err)
		}
		for name, spec := range spec.Sequences {
			specs.Sequences[name] = spec
		}
	}
	idf := id.NewGeneratorFactory(4, 100) // generate 4-character ids for jobs
	grf := grapher.NewGrapherFactory(jobs.Factory, &specs, idf)
	return grf, nil
}

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
	jrc := jr.NewClient(httpClient, jrcfg.ServerURL)
	return jrc, nil
}

func MakeDbConnPool(ctx Context) (myconn.Connector, error) {
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
	dbc := myconn.NewPool(db)
	return dbc, nil
}
