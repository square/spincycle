// Copyright 2019, Square, Inc.

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
)

var (
	tsFormat = "2006-01-02 15:04:05 MST"
)

type Info struct {
	ctx   app.Context
	reqId string
}

func NewInfo(ctx app.Context) *Info {
	return &Info{
		ctx: ctx,
	}
}

func (c *Info) Prepare() error {
	if len(c.ctx.Command.Args) == 0 {
		return fmt.Errorf("Usage: spinc info <request ID>\n")
	}
	c.reqId = c.ctx.Command.Args[0]
	return nil
}

func (c *Info) Run() error {
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

	now := time.Now()

	var started string
	if r.StartedAt != nil && !r.StartedAt.IsZero() {
		started = fmt.Sprintf("%s (%s ago)", r.StartedAt.Format(tsFormat), now.Sub(*r.StartedAt).Round(time.Second))
	}

	var finished string
	if r.FinishedAt != nil && !r.FinishedAt.IsZero() {
		finished = fmt.Sprintf("%s (%s ago)", r.FinishedAt.Format(tsFormat), now.Sub(*r.FinishedAt).Round(time.Second))
	}

	args := []string{}
	for _, arg := range r.Args {
		if arg.Type == "static" {
			continue
		}
		val := fmt.Sprintf("%s", arg.Value)
		args = append(args, fmt.Sprintf("%s=%s", arg.Name, QuoteArgValue(val)))
	}

	fmt.Fprintf(c.ctx.Out, "      id: %s\n", r.Id)
	fmt.Fprintf(c.ctx.Out, " request: %s\n", r.Type)
	fmt.Fprintf(c.ctx.Out, "  caller: %s\n", r.User)
	fmt.Fprintf(c.ctx.Out, " created: %s (%s ago)\n", r.CreatedAt.Format(tsFormat), now.Sub(r.CreatedAt).Round(time.Second))
	fmt.Fprintf(c.ctx.Out, " started: %s\n", started)
	fmt.Fprintf(c.ctx.Out, "finished: %s\n", finished)
	fmt.Fprintf(c.ctx.Out, "   state: %s\n", proto.StateName[r.State])
	fmt.Fprintf(c.ctx.Out, "    host: %s\n", r.JobRunnerURL)
	fmt.Fprintf(c.ctx.Out, "    jobs: %d (%d complete)\n", r.TotalJobs, r.FinishedJobs)
	fmt.Fprintf(c.ctx.Out, "    args: %s\n", strings.Join(args, " "))

	return nil
}

func (c *Info) Cmd() string {
	return "info " + c.reqId
}

func (c *Info) Help() string {
	return "'spinc info <request ID>' prints complete request information.\n" +
		"For basic request status and information, use 'spinc status <request ID>'.\n"
}
