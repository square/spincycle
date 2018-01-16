// Copyright 2017, Square, Inc.

package main

import (
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	myconn "github.com/go-mysql/conn"
	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/square/spincycle/config"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/job/external"
	"github.com/square/spincycle/request-manager/api"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/request-manager/joblog"
	"github.com/square/spincycle/request-manager/request"
	"github.com/square/spincycle/request-manager/status"
	"github.com/square/spincycle/util"
)

func main() {
	// //////////////////////////////////////////////////////////////////////
	// Config
	// //////////////////////////////////////////////////////////////////////
	var cfgFile string
	switch os.Getenv("ENVIRONMENT") {
	case "staging":
		cfgFile = "config/staging.yaml"
	case "production":
		cfgFile = "config/staging.yaml"
	default:
		cfgFile = "config/development.yaml"
	}
	var cfg config.RequestManager
	err := config.Load(cfgFile, &cfg)
	if err != nil {
		log.Fatalf("error loading config at %s: %s", cfgFile, err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Grapher Factory
	// //////////////////////////////////////////////////////////////////////
	allGrapherCfgs := grapher.Config{
		Sequences: map[string]*grapher.SequenceSpec{},
	}
	// For each config in the cfg.SpecFileDir directory, read the file and
	// then aggregate all of the resulting configs into a single struct.
	files, _ := ioutil.ReadDir(cfg.SpecFileDir) // add your specs to this dir
	for _, f := range files {
		grapherCfg, err := grapher.ReadConfig(cfg.SpecFileDir + "/" + f.Name())
		if err != nil {
			log.Fatalf("error reading grapher config file %s: %s", f.Name(), err)
		}
		for k, v := range grapherCfg.Sequences {
			allGrapherCfgs.Sequences[k] = v
		}
	}
	idf := id.NewGeneratorFactory(4, 100) // generate 4-character ids for jobs
	grf := grapher.NewGrapherFactory(external.JobFactory, &allGrapherCfgs, idf)

	// //////////////////////////////////////////////////////////////////////
	// Job Runner Client
	// //////////////////////////////////////////////////////////////////////
	httpClient := &http.Client{}
	if cfg.JRClient.TLS.CertFile != "" && cfg.JRClient.TLS.KeyFile != "" && cfg.JRClient.TLS.CAFile != "" {
		tlsConfig, err := util.NewTLSConfig(cfg.JRClient.TLS.CAFile,
			cfg.JRClient.TLS.CertFile, cfg.JRClient.TLS.KeyFile)
		if err != nil {
			log.Fatalf("error loading JR client TLS config: %s", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}
	}
	jrClient := jr.NewClient(httpClient, cfg.JRClient.ServerURL)

	// //////////////////////////////////////////////////////////////////////
	// DB Connection Pool
	// //////////////////////////////////////////////////////////////////////
	dsn := cfg.Db.DSN + "?parseTime=true" // always needs to be set
	if cfg.Db.TLS.CAFile != "" && cfg.Db.TLS.CertFile != "" && cfg.Db.TLS.KeyFile != "" {
		tlsConfig, err := util.NewTLSConfig(cfg.Db.TLS.CAFile, cfg.Db.TLS.CertFile, cfg.Db.TLS.KeyFile)
		if err != nil {
			log.Fatalf("error loading database TLS config: %s", err)
		}
		mysql.RegisterTLSConfig("custom", tlsConfig)
		dsn += "&tls=custom"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("error creating sql.DB: %s", err)
	}
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(12 * time.Hour)
	dbc := myconn.NewPool(db)

	// //////////////////////////////////////////////////////////////////////
	// Request Manager, Job Log Store, and Job Chain Store
	// //////////////////////////////////////////////////////////////////////
	rm := request.NewManager(grf, dbc, jrClient)
	jls := joblog.NewStore(dbc)

	// //////////////////////////////////////////////////////////////////////
	// System Status
	// //////////////////////////////////////////////////////////////////////
	stat := status.NewManager(
		dbc,
		jrClient,
	)

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	api := api.NewAPI(rm, jls, stat)

	// If you want to add custom middleware for authentication, authorization,
	// etc., you should do that here. See https://echo.labstack.com/middleware
	// for more details.
	api.Use((func(h echo.HandlerFunc) echo.HandlerFunc {
		// This middleware will always set the username of the request to be
		// "admin". You can change this as necessary.
		return func(c echo.Context) error {
			c.Set("username", "admin")
			return h(c)
		}
	}))
	api.Use(middleware.Recover())
	api.Use(middleware.Logger())

	// Start the web server.
	if cfg.Server.TLS.CertFile != "" && cfg.Server.TLS.KeyFile != "" {
		err = http.ListenAndServeTLS(cfg.Server.ListenAddress,
			cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile, api)
	} else {
		err = http.ListenAndServe(cfg.Server.ListenAddress, api)
	}
	log.Fatalf("error running the web server: %s", err)
}
