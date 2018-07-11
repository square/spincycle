package cmd

import (
	"fmt"
	"time"

	"github.com/square/spincycle/spinc/app"
)

const (
	JOB_COL_LEN = 100
)

type Ps struct {
	ctx app.Context
}

func NewPs(ctx app.Context) *Ps {
	return &Ps{
		ctx: ctx,
	}
}

func (c *Ps) Prepare() error {
	return nil
}

func (c *Ps) Run() error {
	status, err := c.ctx.RMClient.SysStatRunning()
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

	hdr := fmt.Sprintf("%%-20s  %%4s  %%5s  %%6s  %%s  \t%%s\n")
	line := fmt.Sprintf("%%-20s  %%4d  %%5d  %%6s  %%s  \t%%s  %%s\n")
	statusLine := fmt.Sprintf("\tJOB STATUS: %%s\n")
	fmt.Fprintf(c.ctx.Out, hdr, "ID", "N", "NJOBS", "TIME", "JOB", "REQUEST")
	for _, r := range status.Jobs {
		runtime := fmt.Sprintf("%.1f", now.Sub(time.Unix(0, r.StartedAt)).Seconds())
		nJobs := 0
		requestName := ""
		args := map[string]interface{}{}
		if status.Requests != nil {
			if r, ok := status.Requests[r.RequestId]; ok {
				nJobs = r.TotalJobs
				requestName = r.Type
				request, err := c.ctx.RMClient.GetRequest(r.Id)
				if err != nil {
					return err
				}
				for k, v := range request.Params {
					args[k] = v
				}
			}
		}
		jobNameLen := len(r.Name)
		if jobNameLen > JOB_COL_LEN {
			// "very_long_job_name" -> "very_long_job_..."
			r.Name = r.Name[jobNameLen-(JOB_COL_LEN-3):jobNameLen] + "..." // -3 for "..."
		}

		argString := ""
		for k, v := range args {
			val, ok := v.(string)
			if !ok {
				val = ""
			}
			argString = argString + k + "=" + val + " "
		}
		fmt.Fprintf(c.ctx.Out, line, r.RequestId, r.N, nJobs, runtime, r.Name, requestName, argString)
		fmt.Fprintf(c.ctx.Out, statusLine, r.Status)
	}

	return nil
}
