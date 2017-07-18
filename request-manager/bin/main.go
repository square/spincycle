// Copyright 2017, Square, Inc.

package main

import (
	"crypto/tls"
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/square/spincycle/config"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/job/external"
	"github.com/square/spincycle/request-manager/api"
	"github.com/square/spincycle/request-manager/grapher"
	"github.com/square/spincycle/request-manager/request"
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
	// Request Resolver
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
		if grapherCfg.NoopNode != nil {
			allGrapherCfgs.NoopNode = grapherCfg.NoopNode
		}
	}
	rr := grapher.NewGrapher(external.JobFactory, &allGrapherCfgs)

	// //////////////////////////////////////////////////////////////////////
	// Job Runner Client
	// //////////////////////////////////////////////////////////////////////
	httpClient := &http.Client{}
	var tlsConfig *tls.Config
	if cfg.JRClient.TLS.CertFile != "" && cfg.JRClient.TLS.KeyFile != "" && cfg.JRClient.TLS.CAFile != "" {
		tlsConfig, err = util.NewTLSConfig(cfg.JRClient.TLS.CAFile,
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
	// Request Manager and its DB Accessor
	// //////////////////////////////////////////////////////////////////////
	params := "?parseTime=true" // always needs to be set
	if cfg.Db.TLS.CAFile != "" && cfg.Db.TLS.CertFile != "" && cfg.Db.TLS.KeyFile != "" {
		tlsConfig, err := util.NewTLSConfig(cfg.Db.TLS.CAFile,
			cfg.Db.TLS.CertFile, cfg.Db.TLS.KeyFile)
		if err != nil {
			log.Fatalf("error loading DB Accessor TLS config: %s", err)
		}
		mysql.RegisterTLSConfig("custom", tlsConfig)
		params += "&tls=custom"
	}
	dsn := cfg.Db.DSN + params
	db, err := sql.Open(cfg.Db.Type, dsn)
	if err != nil {
		log.Fatalf("error opening sql db: %s", err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatalf("error connecting to sql db: %s", err)
	}
	dbAccessor := request.NewDBAccessor(db)

	rm := request.NewManager(rr, dbAccessor, jrClient)

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	api := api.NewAPI(rm)

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
