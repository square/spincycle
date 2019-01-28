// Copyright 2017-2018, Square, Inc.

package app

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/square/spincycle/config"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/util"
)

type Context struct {
	Hooks     Hooks
	Factories Factories

	Config config.JobRunner
}

type Factories struct {
	MakeRequestManagerClient func(Context) (rm.Client, error)
	MakeChainRepo            func(Context) (chain.Repo, error)
}

type Hooks struct {
	LoadConfig  func(Context) (config.JobRunner, error)
	Auth        func(*http.Request) (bool, error)
	SetUsername func(*http.Request) (string, error)

	// RunAPI runs the Job Runner API. It should block until the API is stopped
	// via a call to StopAPI. If this hook is provided, it is called instead of
	// api.Run, and StopAPI must be provided as well.
	RunAPI func() error

	// StopAPI stops running the Job Runner API. It's called when the server is
	// stopped, and it should cause RunAPI to return. If this hook is provided, it
	// is called instead of api.Stop, and RunAPI must be provided as well.
	StopAPI func() error

	// ServerURL returns the base URL to be used for querying this Job Runner's API.
	// This URL is returned to the Request Manager when a job chain is run, so the
	// Request Manager may direct status/stop queries for the request to the
	// correct Job Runner instance. The default ServerURL hook must be overriden if
	// a ListenAddress (and TLS config) is not provided in the Job Runner config
	// file. This is typical if a RunAPI hook has been provided, as ListenAddress
	// and TLS config files are used only in the default api.Run.
	ServerURL func(Context) (string, error)
}

func Defaults() Context {
	return Context{
		Factories: Factories{
			MakeRequestManagerClient: MakeRequestManagerClient,
			MakeChainRepo:            MakeChainRepo,
		},
		Hooks: Hooks{
			LoadConfig: LoadConfig,
			SetUsername: (func(ireq *http.Request) (string, error) {
				return "admin", nil
			}),
			ServerURL: ServerURL,
		},
	}
}

func LoadConfig(appCtx Context) (config.JobRunner, error) {
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
	var cfg config.JobRunner
	err := config.Load(cfgFile, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("error loading config at %s: %s", cfgFile, err)
	}
	return cfg, nil
}

// Default ServerURL Hook. Uses config's ListenAddress as the host and sets scheme
// based on the presence of a TLS config.
func ServerURL(appCtx Context) (string, error) {
	var serverURL url.URL

	address := appCtx.Config.ListenAddress
	if address == "" {
		return "", fmt.Errorf("listen_address not set in config")
	}
	serverURL.Host = address

	// If config has TLS info, use https; else http.
	if appCtx.Config.TLS.CertFile != "" && appCtx.Config.TLS.KeyFile != "" {
		serverURL.Scheme = "https"
	} else {
		serverURL.Scheme = "http"
	}

	return serverURL.String(), nil
}

func MakeRequestManagerClient(appCtx Context) (rm.Client, error) {
	cfg := appCtx.Config
	httpClient := &http.Client{}
	if cfg.RMClient.TLS.CertFile != "" && cfg.RMClient.TLS.KeyFile != "" && cfg.RMClient.TLS.CAFile != "" {
		tlsConfig, err := util.NewTLSConfig(
			cfg.RMClient.TLS.CAFile,
			cfg.RMClient.TLS.CertFile,
			cfg.RMClient.TLS.KeyFile,
		)
		if err != nil {
			return nil, fmt.Errorf("error loading RM client TLS config: %s", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}
	}
	rmc := rm.NewClient(httpClient, cfg.RMClient.ServerURL)
	return rmc, nil
}

func MakeChainRepo(appCtx Context) (chain.Repo, error) {
	cfg := appCtx.Config
	switch cfg.ChainRepoType {
	case "memory":
		return chain.NewMemoryRepo(), nil
	case "redis":
		redisConf := chain.RedisRepoConfig{
			Network:     cfg.Redis.Network,
			Address:     cfg.Redis.Address,
			Prefix:      cfg.Redis.Prefix,
			IdleTimeout: time.Duration(cfg.Redis.IdleTimeout) * time.Second,
			MaxIdle:     cfg.Redis.MaxIdle,
		}
		chainRepo, err := chain.NewRedisRepo(redisConf)
		if err != nil {
			return nil, fmt.Errorf("error setting up redis chain repo: %s", err)
		}
		return chainRepo, nil
	}
	return nil, fmt.Errorf("invalid chain repo type (%s). valid options are 'memory' and 'redis'", cfg.ChainRepoType)
}
