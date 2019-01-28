// Copyright 2017-2019, Square, Inc.

// Package spinc provides a framework for integration with other programs.
package spinc

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	rm "github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/cmd"
	"github.com/square/spincycle/spinc/config"
)

var (
	ErrHelp = errors.New("print help")
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
	if o.Debug {
		app.Debug("command: %#v\n", c)
		app.Debug("options: %#v\n", o)
	}

	if ctx.Hooks.AfterParseOptions != nil {
		if o.Debug {
			app.Debug("calling hook AfterParseOptions")
		}
		ctx.Hooks.AfterParseOptions(&o)

		// Dump options again to see if hook changed them
		if o.Debug {
			app.Debug("options: %#v\n", o)
		}
	}
	ctx.Options = o
	ctx.Command = c

	// //////////////////////////////////////////////////////////////////////
	// Help and version
	// //////////////////////////////////////////////////////////////////////

	// Help uses a Request Manager client to fetch the list of all requests.
	// If addr is set, then this works; else, ignore and always print help.
	rmc, _ := makeRMC(&ctx)

	// spinc with no args (Args[0] = "spinc" itself). Print short request help
	// because Ryan is very busy.
	if len(os.Args) == 1 {
		config.Help(false, rmc, ctx.Out)
		return ErrHelp
	}

	// spinc --help or spinc help (full help)
	if o.Help || c.Cmd == "help" || c.Cmd == "" {
		config.Help(true, rmc, ctx.Out)
		return ErrHelp
	}

	// spinc help <command>
	if c.Cmd == "help" && len(c.Args) > 0 {
		// Need rm client for this
		if rmc == nil {
			var err error
			rmc, err = makeRMC(&ctx)
			if err != nil {
				return err
			}
		}
		reqName := c.Args[0]
		if err := config.RequestHelp(reqName, rmc); err != nil {
			switch err {
			case config.ErrUnknownRequest:
				return fmt.Errorf("Unknown request: %s. Run spinc (no arguments) to list all requests.", reqName)
			default:
				return fmt.Errorf("API error: %s. Use --ping to test the API connection.", err)
			}
		}
		return nil
	}

	// spinc --version or spinc version
	if o.Version || c.Cmd == "version" {
		fmt.Println("spinc v0.0.0")
		return nil
	}

	// //////////////////////////////////////////////////////////////////////
	// Request Manager Client
	// //////////////////////////////////////////////////////////////////////
	if rmc == nil {
		var err error
		rmc, err = makeRMC(&ctx)
		if err != nil {
			return err
		}
	}

	// //////////////////////////////////////////////////////////////////////
	// Ping
	// //////////////////////////////////////////////////////////////////////
	if o.Ping {
		if _, err := rmc.RequestList(); err != nil {
			return fmt.Errorf("Ping failed: %s", err)
		}
		fmt.Printf("%s OK\n", o.Addr)
		return nil
	}

	// //////////////////////////////////////////////////////////////////////
	// Commands
	// //////////////////////////////////////////////////////////////////////
	cmdFactory := &cmd.DefaultFactory{}

	var err error
	var run app.Command
	if ctx.Factories.Command != nil {
		run, err = ctx.Factories.Command.Make(c.Cmd, ctx)
		if err != nil {
			switch err {
			case cmd.ErrNotExist:
				if o.Debug {
					app.Debug("user cmd factory cannot make a %s cmd, trying default factory", c.Cmd)
				}
			default:
				return fmt.Errorf("User command factory error: %s", err)
			}
		}
	}
	if run == nil {
		if o.Debug {
			app.Debug("using default factory to make a %s cmd", c.Cmd)
		}
		run, err = cmdFactory.Make(c.Cmd, ctx)
		if err != nil {
			switch err {
			case cmd.ErrNotExist:
				return fmt.Errorf("Unknown command: %s. Run 'spinc help' to list commands.", c.Cmd)
			default:
				return fmt.Errorf("Command factory error: %s", err)
			}
		}
	}

	if err := run.Prepare(); err != nil {
		if o.Debug {
			app.Debug("%s Prepare error: %s", c.Cmd, err)
		}
		switch err {
		case config.ErrUnknownRequest:
			reqName := c.Args[0]
			return fmt.Errorf("Unknown request: %s. Run spinc (no arguments) to list all requests.", reqName)
		default:
			return err
		}
	}

	if err := run.Run(); err != nil {
		if o.Debug {
			app.Debug("%s Run error: %s", c.Cmd, err)
		}
		return err
	}

	return nil
}

func makeRMC(ctx *app.Context) (rm.Client, error) {
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
		httpClient, err = ctx.Factories.HTTPClient.Make(*ctx)
	} else {
		httpClient = &http.Client{}
	}
	if err != nil {
		return nil, fmt.Errorf("Error making http.Client: %s", err)
	}
	ctx.RMClient = rm.NewClient(httpClient, ctx.Options.Addr)
	return ctx.RMClient, nil
}
