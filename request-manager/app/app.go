// Copyright 2017, Square, Inc.

package app

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	myconn "github.com/go-mysql/conn"
	"github.com/go-sql-driver/mysql"

	"github.com/square/spincycle/config"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/jobs"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/util"
)

type Context struct {
	Hooks     Hooks
	Factories Factories

	Config config.RequestManager
}

type Factories struct {
	MakeGrapher         func(Context) (grapher.GrapherFactory, error) // @fixme
	MakeJobRunnerClient func(Context) (jr.Client, error)
	MakeDbConnPool      func(Context) (myconn.Connector, error)
}

type Hooks struct {
	LoadConfig  func(Context) (config.RequestManager, error)
	Auth        func(*http.Request) (bool, error)
	SetUsername func(*http.Request) (string, error)
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
	allGrapherCfgs := grapher.Config{
		Sequences: map[string]*grapher.SequenceSpec{},
	}
	// For each config in the cfg.SpecFileDir directory, read the file and
	// then aggregate all of the resulting configs into a single struct.
	files, _ := ioutil.ReadDir(ctx.Config.SpecFileDir) // add your specs to this dir
	for _, f := range files {
		grapherCfg, err := grapher.ReadConfig(ctx.Config.SpecFileDir + "/" + f.Name())
		if err != nil {
			return nil, fmt.Errorf("error reading grapher config file %s: %s", f.Name(), err)
		}
		for k, v := range grapherCfg.Sequences {
			allGrapherCfgs.Sequences[k] = v
		}
	}
	idf := id.NewGeneratorFactory(4, 100) // generate 4-character ids for jobs
	grf := grapher.NewGrapherFactory(jobs.Factory, &allGrapherCfgs, idf)
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
