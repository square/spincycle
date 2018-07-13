package cmd

import (
	"fmt"
	"text/tabwriter"
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

	w := new(tabwriter.Writer)
	w.Init(c.ctx.Out, 0, 8, 0, '\t', 0)
	hdr := fmt.Sprintf("%%-20s\t%%4s\t%%5s\t%%6s\t%%s\t%%s\t%%s\n")
	line := fmt.Sprintf("%%-20s\t%%4d\t%%5d\t%%6s\t%%s\t%%s\t%%s  %%s\n")

	if c.ctx.Options.Verbose {
		fmt.Fprintf(w, hdr, "ID", "N", "NJOBS", "TIME", "OWNER", "JOB", "REQUEST")
	} else {
		fmt.Fprintf(w, hdr, "ID", "N", "NJOBS", "TIME", "OWNER", "JOB", "")
	}

	for _, r := range status.Jobs {
		runtime := fmt.Sprintf("%.1f", now.Sub(time.Unix(0, r.StartedAt)).Seconds())
		nJobs := 0
		requestName := ""
		owner := ""
		args := map[string]interface{}{}
		if status.Requests != nil {
			if r, ok := status.Requests[r.RequestId]; ok {
				nJobs = r.TotalJobs
				owner = r.User
				if c.ctx.Options.Verbose {
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
		fmt.Fprintf(w, line, r.RequestId, r.N, nJobs, runtime, owner, r.Name, requestName, argString)
	}

	fmt.Fprintln(w)
	w.Flush()

	return nil
}
