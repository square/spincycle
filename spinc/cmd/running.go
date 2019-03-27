// Copyright 2017-2019, Square, Inc.

package cmd

import (
	"fmt"
	"os"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
)

type Running struct {
	ctx   app.Context
	reqId string
}

func NewRunning(ctx app.Context) *Running {
	return &Running{
		ctx: ctx,
	}
}

func (c *Running) Prepare() error {
	if len(c.ctx.Command.Args) == 0 {
		return fmt.Errorf("Usage: spinc running <id>\n")
	}
	c.reqId = c.ctx.Command.Args[0]
	return nil
}

func (c *Running) Run() error {
	status, err := c.ctx.RMClient.GetRequest(c.reqId)
	if err != nil {
		return err
	}
	if c.ctx.Options.Debug {
		app.Debug("status: %#v", status)
	}

	if c.ctx.Hooks.CommandRunResult != nil {
		c.ctx.Hooks.CommandRunResult(status, err)
		return nil
	}

	// Request is running if in these two states:
	if status.State == proto.STATE_PENDING || status.State == proto.STATE_RUNNING {
		os.Exit(0)
	}

	os.Exit(1)
	return nil // not reached
}

func (c *Running) Cmd() string {
	return "running " + c.reqId
}

func (c *Running) Help() string {
	return "'spinc running <request ID>' exits 0 if the request is pending or running, else exits 1.\n" +
		"This can be used in Bash scripts like: 'while spinc running <request ID>; do sleep 2; done'.\n"
}
