package cmd

import (
	"fmt"

	"github.com/square/spincycle/spinc/app"
)

type Stop struct {
	ctx   app.Context
	reqId string
}

func NewStop(ctx app.Context) *Stop {
	return &Stop{
		ctx: ctx,
	}
}

func (c *Stop) Prepare() error {
	if len(c.ctx.Command.Args) == 0 {
		return fmt.Errorf("Usage: spinc stop <id>\n")
	}
	c.reqId = c.ctx.Command.Args[0]
	return nil
}

func (c *Stop) Run() error {
	if err := c.ctx.RMClient.StopRequest(c.reqId); err != nil {
		return err
	}
	fmt.Fprintf(c.ctx.Out, "OK, stopped %s\n", c.reqId)
	return nil
}
