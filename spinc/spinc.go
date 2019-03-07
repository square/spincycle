// Copyright 2017-2019, Square, Inc.

// Package spinc provides a framework for integration with other programs.
package spinc

import (
	"fmt"
	"net/http"
	"os"
	"time"

	rm "github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/cmd"
	"github.com/square/spincycle/spinc/config"
)

// Run runs spinc and exits when done. When using a standard spinc bin, Run is
// called by spinc/bin/main.go. When spinc is wrapped by custom code, that code
// imports this pkg then call spinc.Run() with its custom factories. If a factory
// is not set (nil), then the default/standard factory is used.
func Run(ctx app.Context) error {
	// //////////////////////////////////////////////////////////////////////
	// Config and command line
	// //////////////////////////////////////////////////////////////////////

	// Options are set in this order: config -> env var -> cmd line option.
	// So first we must apply config files, then do cmd line parsing which
	// will apply env vars and cmd line options.

	// Parse cmd line to get --config files
	cmdLine := config.ParseCommandLine(config.Options{})

	// --config files override defaults if given
	configFiles := config.DEFAULT_CONFIG_FILES
	if cmdLine.Config != "" {
		configFiles = cmdLine.Config
	}

	// Parse default options from config files
	def := config.ParseConfigFiles(configFiles, cmdLine.Debug)

	// Parse env vars and cmd line options, override default config
	cmdLine = config.ParseCommandLine(def)

	// Final options and commands
	var o config.Options = cmdLine.Options
	var c config.Command = cmdLine.Command

	// Apply defaults
	if o.Timeout == 0 {
		o.Timeout = config.DEFAULT_TIMEOUT
	}
	if o.Addr == "" {
		o.Addr = config.DEFAULT_ADDR
	}

	// Let hook modify options, if set
	if ctx.Hooks.AfterParseOptions != nil {
		if o.Debug {
			app.Debug("calling hook AfterParseOptions")
		}
		ctx.Hooks.AfterParseOptions(&o)
	}

	// This is a little hack to make spinc -> quick help work, i.e. print
	// quick help when there is no command. We can't check os.Args because
	// it'll be >0 if any flag, like --debug, is specified but we ignore
	// flags. And we can't check c.Cmd == "" because we set c.Cmd = "help".
	ctx.Nargs = len(c.Args) + 1
	if c.Cmd == "" {
		ctx.Nargs -= 1
	}

	// spinc with no args or --help = spinc help
	if len(os.Args) == 1 || o.Help || c.Cmd == "" {
		c.Cmd = "help"
	}

	// --version = spinc version
	if o.Version {
		c.Cmd = "version"
	}

	ctx.Options = o
	ctx.Command = c
	if o.Debug {
		app.Debug("command: %#v\n", c)
		app.Debug("options: %#v\n", o)
	}

	// Use default, built-in command factory if not set by user
	if ctx.Factories.Command == nil {
		ctx.Factories.Command = &cmd.DefaultFactory{}
	}

	// //////////////////////////////////////////////////////////////////////
	// Request Manager Client
	// //////////////////////////////////////////////////////////////////////
	var err error
	ctx.RMClient, err = makeRMC(ctx)
	if err != nil {
		if o.Debug {
			app.Debug("error making RM client: %s", err)
		}
		// All cmds except help and version require an RM client
		if c.Cmd != "help" && c.Cmd != "version" {
			return err
		}
	}

	// //////////////////////////////////////////////////////////////////////
	// Commands
	// //////////////////////////////////////////////////////////////////////

	spincCmd, err := ctx.Factories.Command.Make(c.Cmd, ctx)
	if err != nil {
		switch err {
		case cmd.ErrNotExist:
			return fmt.Errorf("Unknown command: %s. Run 'spinc help' to list commands.", c.Cmd)
		default:
			return fmt.Errorf("Command factory error: %s", err)
		}
	}

	// Let command prepare to run. The start command makes heavy use of this.
	if err := spincCmd.Prepare(); err != nil {
		if o.Debug {
			app.Debug("%s Prepare error: %s", c.Cmd, err)
		}
		switch err {
		case app.ErrUnknownRequest:
			reqName := c.Args[0]
			return fmt.Errorf("Unknown request: %s. Run spinc (no arguments) to list all requests.", reqName)
		default:
			return err
		}
	}

	err = spincCmd.Run()
	if o.Debug {
		app.Debug("%s Run error: %s", c.Cmd, err)
	}
	return err
}

func makeRMC(ctx app.Context) (rm.Client, error) {
	if ctx.Options.Addr == "" {
		return nil, fmt.Errorf("Request Manager API address is not set."+
			" It is best to specify addr in a config file (%s). Or, specify"+
			" --addr on the command line option or set the ADDR environment"+
			" variable. Use --ping to test addr when set.", config.DEFAULT_CONFIG_FILES)
	}
	if ctx.Options.Debug {
		app.Debug("addr: %s", ctx.Options.Addr)
	}
	var httpClient *http.Client
	var err error
	if ctx.Factories.HTTPClient != nil {
		httpClient, err = ctx.Factories.HTTPClient.Make(ctx)
	} else {
		httpClient = &http.Client{
			Timeout: time.Duration(ctx.Options.Timeout) * time.Millisecond,
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Error making http.Client: %s", err)
	}
	rmc := rm.NewClient(httpClient, ctx.Options.Addr)
	return rmc, nil
}
