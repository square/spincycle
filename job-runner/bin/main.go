// Copyright 2017, Square, Inc.

package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/middleware"
	"github.com/orcaman/concurrent-map"
	"github.com/square/spincycle/config"
	"github.com/square/spincycle/job-runner/api"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/job/external"
	rm "github.com/square/spincycle/request-manager"
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
	var cfg config.JobRunner
	err := config.Load(cfgFile, &cfg)
	if err != nil {
		log.Fatalf("error loading config at %s: %s", cfgFile, err)
	}

	// //////////////////////////////////////////////////////////////////////
	// Chain repo
	// //////////////////////////////////////////////////////////////////////
	var chainRepo chain.Repo
	switch cfg.ChainRepoType {
	case "memory":
		chainRepo = chain.NewMemoryRepo()
	case "redis":
		redisConf := chain.RedisRepoConfig{
			Network:     cfg.Redis.Network,
			Address:     cfg.Redis.Address,
			Prefix:      cfg.Redis.Prefix,
			IdleTimeout: time.Duration(cfg.Redis.IdleTimeout) * time.Second,
			MaxIdle:     cfg.Redis.MaxIdle,
		}
		chainRepo, err = chain.NewRedisRepo(redisConf)
		if err != nil {
			log.Fatalf("error setting up redis chain repo: %s", err)
		}
	default:
		log.Fatalf("invalid chain repo type (%s). valid options are 'memory' and 'redis'", cfg.ChainRepoType)
	}

	// //////////////////////////////////////////////////////////////////////
	// Request Manager Client
	// //////////////////////////////////////////////////////////////////////
	httpClient := &http.Client{}
	var tlsConfig *tls.Config
	if cfg.RMClient.TLS.CertFile != "" && cfg.RMClient.TLS.KeyFile != "" && cfg.RMClient.TLS.CAFile != "" {
		tlsConfig, err = util.NewTLSConfig(cfg.RMClient.TLS.CAFile,
			cfg.RMClient.TLS.CertFile, cfg.RMClient.TLS.KeyFile)
		if err != nil {
			log.Fatalf("error loading RM client TLS config: %s", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}
	}
	rmClient := rm.NewClient(httpClient, cfg.RMClient.ServerURL)

	// //////////////////////////////////////////////////////////////////////
	// Runner factory
	// //////////////////////////////////////////////////////////////////////
	rf := runner.NewFactory(external.JobFactory, rmClient)

	// //////////////////////////////////////////////////////////////////////
	// Traverser repo and factory
	// //////////////////////////////////////////////////////////////////////
	trRepo := cmap.New()
	trFactory := chain.NewTraverserFactory(chainRepo, rf, rmClient)

	// //////////////////////////////////////////////////////////////////////
	// API
	// //////////////////////////////////////////////////////////////////////
	api := api.NewAPI(trFactory, trRepo)

	// If you want to add custom middleware for authentication, authorization,
	// etc., you should do that here. See https://echo.labstack.com/middleware
	// for more details.
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
