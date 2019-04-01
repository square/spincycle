// Copyright 2017-2019, Square, Inc.

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
)

type Status struct {
	ctx   app.Context
	reqId string
}

func NewStatus(ctx app.Context) *Status {
	return &Status{
		ctx: ctx,
	}
}

func (c *Status) Prepare() error {
	if len(c.ctx.Command.Args) == 0 {
		return fmt.Errorf("Usage: spinc status <request ID>\n")
	}
	c.reqId = c.ctx.Command.Args[0]
	return nil
}

func (c *Status) Run() error {
	r, err := c.ctx.RMClient.GetRequest(c.reqId)
	if err != nil {
		return err
	}
	if c.ctx.Options.Debug {
		app.Debug("request: %#v", r)
	}
	if c.ctx.Hooks.CommandRunResult != nil {
		c.ctx.Hooks.CommandRunResult(r, err)
		return nil
	}

	runtime := "not started"
	if r.StartedAt != nil && !r.StartedAt.IsZero() {
		runtime = time.Now().Sub(*r.StartedAt).Round(time.Second).String()
	}

	args := []string{}
	for _, arg := range r.Args {
		if arg.Type != "required" {
			continue
		}
		args = append(args, fmt.Sprintf("%s=%s", arg.Name, arg.Value))
	}

	fmt.Fprintf(c.ctx.Out, "   state: %s\n", proto.StateName[r.State])
	fmt.Fprintf(c.ctx.Out, "progress: %s\n", fmt.Sprintf("%.0f%%", float64(r.FinishedJobs)/float64(r.TotalJobs)*100))
	fmt.Fprintf(c.ctx.Out, " runtime: %s\n", runtime)
	fmt.Fprintf(c.ctx.Out, " request: %s\n", r.Type)
	fmt.Fprintf(c.ctx.Out, "  caller: %s\n", r.User)
	fmt.Fprintf(c.ctx.Out, "    args: %s\n", strings.Join(args, " "))

	return nil
}

func (c *Status) Cmd() string {
	return "status " + c.reqId
}

func (c *Status) Help() string {
	return "'spinc status <request ID>' prints request status and basic information.\n" +
		"For complete request information, use 'spinc info <request ID>'.\n"
}
