// Copyright 2017-2019, Square, Inc.

package cmd

import (
	"fmt"
	"time"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
)

const (
	reqColLen  = 20
	userColLen = 9
	jobColLen  = 22
)

type Ps struct {
	ctx   app.Context
	reqId string
}

func NewPs(ctx app.Context) *Ps {
	return &Ps{
		ctx: ctx,
	}
}

func (c *Ps) Prepare() error {
	n := len(c.ctx.Command.Args)
	if n == 0 {
		return nil
	}
	if n == 1 {
		c.reqId = c.ctx.Command.Args[0]
		return nil
	}
	return fmt.Errorf("Usage: spinc ps [id]\n")
}

func (c *Ps) Run() error {
	f := proto.StatusFilter{
		RequestId: c.reqId,
	}
	status, err := c.ctx.RMClient.Running(f)
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

	if len(status.Jobs) == 0 {
		return nil
	}

	now := time.Now()

	/*
	   REQUEST              ID                    PRG  USER      RUNTIME  TRY JOB                  STATUS
	   12345678901234567890 --------------------  100% 123456789 12345678   3 12345678901234567890 *
	*/
	hdr := "%-" + fmt.Sprintf("%d", reqColLen) + "s %-20s %4s  %-" + fmt.Sprintf("%d", userColLen) + "s %-8s %3s %-" + fmt.Sprintf("%d", jobColLen) + "s %s\n"
	fmt.Fprintf(c.ctx.Out, hdr, "REQUEST", "ID", "PRG", "USER", "RUNTIME", "TRY", "JOB", "STATUS")

	line := "%-" + fmt.Sprintf("%d", reqColLen) + "s %-20s %4s  %-" + fmt.Sprintf("%d", userColLen) + "s %-8s %3d %-" + fmt.Sprintf("%d", jobColLen) + "s %s\n"
	for _, j := range status.Jobs {
		reqName := "unknown"
		reqId := ""
		reqPrg := "0"
		reqUser := ""
		if r, ok := status.Requests[j.RequestId]; ok {
			reqName = r.Type
			reqId = r.Id
			reqPrg = fmt.Sprintf("%.0f%%", float64(r.FinishedJobs)/float64(r.TotalJobs)*100)
			reqUser = r.User
		}
		runtime := now.Sub(time.Unix(0, j.StartedAt)).Round(time.Second)
		fmt.Fprintf(c.ctx.Out, line,
			SqueezeString(reqName, reqColLen, ".."), reqId, reqPrg, SqueezeString(reqUser, userColLen, ".."),
			runtime, j.Try, SqueezeString(j.Name, jobColLen, ".."),
			j.Status,
		)
	}

	return nil
}

func (c *Ps) Cmd() string {
	if c.reqId != "" {
		return "ps " + c.reqId
	}
	return "ps"
}

func (c *Ps) Help() string {
	return "'spinc ps [request ID]' prints running requests and jobs.\n" +
		"Request ID is optional. If given, only its running jobs are printed; else, all requests' running jobs are printed.\n" +
		"Columns:\n" +
		"  REQUEST: Request name\n" +
		"  ID:      Request ID\n" +
		"  PRG:     Request progress\n" +
		"  USER:    User/owner who started the request\n" +
		"  RUNTIME: Job runtime (1s resolution)\n" +
		"  TRY:     Job try count\n" +
		"  JOB:     Job name from request spec\n" +
		"  STATUS:  Real-time job status\n" +
		"Long column values are truncated in the middle with '..'.\n"
}
