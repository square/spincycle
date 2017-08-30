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
	status, err := c.ctx.RMClient.RequestStatus(c.reqId)
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
