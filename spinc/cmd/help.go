// Copyright 2019, Square, Inc.

package cmd

import (
	"fmt"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
	"github.com/square/spincycle/spinc/config"
)

type Help struct {
	ctx      app.Context
	helpType string
}

func NewHelp(ctx app.Context) *Help {
	return &Help{
		ctx: ctx,
	}
}

func (c *Help) Prepare() error {
	return nil
}

func (c *Help) Run() error {
	// Return app.ErrHelp on success. This is a little hack to make a test ing
	// ../spinc_test.go work.

	cmdlineArgs := c.ctx.Nargs
	commandArgs := len(c.ctx.Command.Args)

	if c.ctx.Options.Help { // spinc --help
		c.Usage()
	} else if cmdlineArgs == 0 && commandArgs == 0 { // spinc
		c.QuickHelp()
	} else if cmdlineArgs == 1 && commandArgs == 0 { // spinc help
		c.Usage()
	} else { // spinc help <cmd|req>
		arg := c.ctx.Command.Args[0]

		// Is arg a command? If yes, print cmd.Help().
		spincCmd, err := c.ctx.Factories.Command.Make(arg, c.ctx)
		if c.ctx.Options.Debug {
			app.Debug("Factories.Command.Make: %s", err)
		}
		if err == nil {
			fmt.Fprintf(c.ctx.Out, spincCmd.Help())
			return app.ErrHelp
		}

		// Is arg a request? If yes, print request args.
		if err := c.RequestHelp(arg); err != nil {
			switch err {
			case app.ErrUnknownRequest:
				return fmt.Errorf("'%s' is not a valid command or request. Run 'spinc help' to list commands, or 'spinc' to list requests.", arg)
			default:
				return fmt.Errorf("Cannot get help for request '%s' because the Request Manager at address %s returned an error: %s. "+
					"Verify that --addr is correct and the Request Manager is running.", arg, c.ctx.Options.Addr, err)
			}
		}
	}
	return app.ErrHelp
}

func (c *Help) Cmd() string {
	return "help"
}

func (c *Help) Help() string {
	return "Run 'spinc help' for usage, or 'spinc help <command>' for command help, or 'spinc help <request>' for request help.\n"
}

// --------------------------------------------------------------------------

func (c *Help) Usage() {
	fmt.Fprintf(c.ctx.Out, "Usage: spinc [flags] command [request|id] [args]\n\n"+
		"Flags:\n"+
		"  --addr     Request Manager address (default: %s)\n"+
		"  --config   Config files (default: %s)\n"+
		"  --debug    Print debug to stderr\n"+
		"  --env      Environment (dev, staging, production)\n"+
		"  --help     Print help\n"+
		"  --timeout  API timeout, milliseconds (default: %d ms)\n"+
		"  --version  Print version\n"+
		"Commands:\n"+
		"  help    <cmd|req>  Print command or request help\n"+
		"  info    <ID>       Print complete request information\n"+
		"  log     <ID>       Print job log (tip: pipe output to less)\n"+
		"  ps      [ID]       Show running requests and jobs (request ID optional)\n"+
		"  running <ID>       Exit 0 if request is pending or running, else exit 1\n"+
		"  start   <request>  Start new request\n"+
		"  status  <ID>       Print request status and basic information\n"+
		"  stop    <ID>       Stop request\n"+
		"  version            Print Spin Cycle version\n",
		config.DEFAULT_ADDR, config.DEFAULT_CONFIG_FILES, config.DEFAULT_TIMEOUT)
	fmt.Fprintf(c.ctx.Out, "\nRun spinc (no command) to lists requests\n")
}

func (c *Help) QuickHelp() {
	if c.ctx.RMClient == nil {
		fmt.Fprintf(c.ctx.Out, "Specify --addr or SPINC_ADDR environment variable to list all requests\n")
		fmt.Fprintf(c.ctx.Out, "Run 'spinc help' for usage\n")
	} else {
		fmt.Fprintf(c.ctx.Out, "Request Manager address: %s\n\n", c.ctx.Options.Addr)
		fmt.Fprintf(c.ctx.Out, "Requests:\n")
		req, err := c.ctx.RMClient.RequestList()
		if err != nil {
			fmt.Fprintf(c.ctx.Out, "  Error getting request list: %s. Verify that --addr is correct and the Request Manager is running.\n", err)
		} else {
			for _, r := range req {
				fmt.Fprintf(c.ctx.Out, "  "+r.Name+"\n")
			}
		}
		fmt.Fprintf(c.ctx.Out, "\nspinc help  <request>\n")
		fmt.Fprintf(c.ctx.Out, "spinc start <request>\n")
	}
}

func (c *Help) RequestHelp(reqName string) error {
	reqList, err := c.ctx.RMClient.RequestList()
	if err != nil {
		return err
	}

	var req *proto.RequestSpec
	for _, r := range reqList {
		if r.Name != reqName {
			continue
		}
		req = &r
		break
	}

	if req == nil {
		return app.ErrUnknownRequest
	}

	fmt.Fprintf(c.ctx.Out, "Request Manager address: %s\n\n", c.ctx.Options.Addr)
	fmt.Fprintf(c.ctx.Out, "%s request args (* required)\n\n", req.Name)
	l := 0
	for _, a := range req.Args {
		if len(a.Name) > l {
			l = len(a.Name)
		}
	}
	line := fmt.Sprintf("  %%s %%-%ds  %%s\n", l)
	for _, a := range req.Args {
		help := a.Desc
		star := " "
		if a.Type == proto.ARG_TYPE_REQUIRED {
			star = "*"
		} else {
			help += " (default: " + a.Default.(string) + ")"
		}
		fmt.Fprintf(c.ctx.Out, line, star, a.Name, help)
	}
	fmt.Fprintf(c.ctx.Out, "\nTo start a %s request, run 'spinc start %s'\n", reqName, reqName)
	return nil
}
