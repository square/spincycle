// Copyright 2017-2019, Square, Inc.

package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/prompt"
)

type Start struct {
	ctx app.Context
	// --
	reqName      string
	requiredArgs []prompt.Item
	optionalArgs []prompt.Item
	debug        bool
	args         map[string]interface{}
	fullCmd      string
}

func NewStart(ctx app.Context) *Start {
	return &Start{
		ctx:   ctx,
		debug: ctx.Options.Debug, // brevity
	}
}

func (c *Start) Prepare() error {
	cmd := c.ctx.Command

	if len(cmd.Args) == 0 {
		return fmt.Errorf("Usage: spinc start <request> [args]\n'spinc' for request list")
	}
	c.reqName = cmd.Args[0]
	cmd.Args = cmd.Args[1:] // shift request name

	// Get request list from API
	reqList, err := c.ctx.RMClient.RequestList()
	if err != nil {
		return fmt.Errorf("Cannot get request list from API: %s", err)
	}

	// Find this request in the request list
	var req *proto.RequestSpec
	for _, r := range reqList {
		if r.Name != c.reqName {
			continue
		}
		req = &r
		break
	}
	if req == nil {
		return app.ErrUnknownRequest
	}

	// Split and save request args given on cmd line
	given := map[string]string{}
	for _, keyval := range cmd.Args {
		p := strings.SplitN(keyval, "=", 2)
		if len(p) != 2 {
			return fmt.Errorf("Invalid command arg: %s: split on = produced %d values, expected 2 (key=val)", keyval, len(p))
		}
		given[p[0]] = p[1]
		if c.debug {
			app.Debug("given '%s'='%s'", p[0], p[1])
		}
	}
	// If no args are given, then it'll be a full prompt: required and all
	// optional args. But if any args are given, then we presume user knows
	// what they're doing and we skip all optional args (let them use default
	// values) and only prompt for missing required args.
	argsGiven := len(given) > 0

	// Group request args by required. We prompt for required first, then optional,
	// both in the order as listed in the request spec because, normally, we list
	// args with some reason. E.g. if request is "shutdown-host", the first required
	// arg is probably "host".  Chances are the optional args have default values,
	// so after entering required args, the user can just hit enter to speed through
	// the optional args.
	c.requiredArgs = []prompt.Item{}
	c.optionalArgs = []prompt.Item{}
	for _, a := range req.Args {
		defaultValue := ""
		if a.Default != nil {
			if s, ok := a.Default.(string); ok {
				defaultValue = s
			}
		}

		// Map all args to prompt items
		i := prompt.Item{
			Name:     a.Name,
			Desc:     a.Desc,
			Required: a.Type == proto.ARG_TYPE_REQUIRED,
			Default:  defaultValue,
		}

		// Always skip given vars. Presume the user knows what they're doing.
		// Note: skip != save. We store the arg/item, we just don't prompt for it.
		if val, ok := given[a.Name]; ok {
			i.Value = val
			i.Skip = true // don't prompt
		}

		// Save the arg/item
		if i.Required {
			c.requiredArgs = append(c.requiredArgs, i)
		} else {
			// Optional arg

			if argsGiven {
				// Skip optional args when any args are given
				i.Skip = true

				// If optional arg not given, use its default value
				if _, ok := given[a.Name]; !ok {
					i.IsDefault = true
					i.Value = a.Default.(string)
					if c.debug {
						app.Debug("optional arg %s using default value %s", a.Name, i.Value)
					}
				}
			}

			c.optionalArgs = append(c.optionalArgs, i)
		}

		// Remove given args from map last because given is used twice above
		delete(given, a.Name)
	}

	// If any cmd args given on the cmd line weren't used, then they're args
	// that the request uses. This is an error for now because we want to be
	// exact, but in the future we might want an option to ignore these to
	// allow for deprecation, backwards-compatibility, etc.
	if len(given) != 0 {
		bad := make([]string, 0, len(given))
		for k := range given {
			bad = append(bad, k)
		}
		return ErrUnknownArgs{
			Request: c.reqName,
			Args:    bad,
		}
	}

	return nil
}

func (c *Start) Run() error {
	// Prompt user for missing required args and possibly optional args
	p := prompt.NewGuidedPrompt(c.requiredArgs, c.ctx.In, c.ctx.Out)
	p.Prompt()
	p = prompt.NewGuidedPrompt(c.optionalArgs, c.ctx.In, c.ctx.Out)
	p.Prompt()

	if c.debug {
		app.Debug("required args: %#v", c.requiredArgs)
		app.Debug("optional args: %#v", c.optionalArgs)
	}

	// Print full command that user can copy-paste to re-run without prompts.
	// Also build the request args map.
	c.Cmd()
	if c.debug {
		app.Debug("request args: %#v", c.args)
	}
	fmt.Printf("\n# spinc %s\n\n", c.fullCmd)

	// Prompt for 'ok' until user enters it or aborts
	ok := prompt.NewConfirmationPrompt("Enter 'ok' to start, or ctrl-c to abort: ", "ok", c.ctx.In, c.ctx.Out)
	for {
		if err := ok.Prompt(); err == nil {
			break
		}
	}

	// //////////////////////////////////////////////////////////////////////
	// Start request
	// //////////////////////////////////////////////////////////////////////
	reqId, err := c.ctx.RMClient.CreateRequest(c.reqName, c.args)
	if err != nil {
		return err
	}

	fmt.Printf("OK, started %s request %s\n\n"+
		"  spinc status %s%s\n\n", c.reqName, reqId, c.userOptionsString(), reqId)

	return nil
}

func (c *Start) userOptionsString() string {
	var userOptions string

	// Add critical options to the output if set
	if c.ctx.UserOptions.Env != "" {
		userOptions = "--env " + escapeArg(c.ctx.Options.Env) + " "
	}

	if c.ctx.UserOptions.Addr != "" {
		userOptions += "--addr " + escapeArg(c.ctx.Options.Addr) + " "
	}

	if c.ctx.UserOptions.Config != "" {
		userOptions += "--config " + escapeArg(c.ctx.Options.Config) + " "
	}

	return userOptions
}

func (c *Start) Cmd() string {
	if c.fullCmd != "" {
		return c.fullCmd
	}

	fullCmd := c.userOptionsString()

	fullCmd += "start " + c.reqName
	args := map[string]interface{}{}
	for _, i := range c.requiredArgs {
		fullCmd += " " + i.Name + "=" + QuoteArgValue(i.Value)
		if i.Value != "" {
			args[i.Name] = i.Value
		}
	}
	for _, i := range c.optionalArgs {
		if !i.IsDefault {
			fullCmd += " " + i.Name + "=" + QuoteArgValue(i.Value)
			args[i.Name] = i.Value
		}
	}
	c.args = args
	c.fullCmd = fullCmd
	return c.fullCmd
}

func (c *Start) Help() string {
	return "'spinc start <request> [args]' starts a new request.\n" +
		"Request args can be provided, else spinc prompts for them. Run 'spinc help <request>' to list the request args.\n"
}

// Escapes strings with whitespace using double quotes
func escapeArg(str string) string {
	if ok, _ := regexp.Match(`.*\s.*`, []byte(str)); ok {
		return "\"" + str + "\""
	} else {
		return str
	}
}
