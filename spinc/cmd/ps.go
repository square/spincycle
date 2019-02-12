// Copyright 2017-2019, Square, Inc.

package cmd

import (
	"fmt"
	"sort"
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
	var line string
	if c.ctx.Options.Verbose {
		fmt.Fprintf(w, hdr, "ID", "N", "NJOBS", "TIME", "OWNER", "JOB", "REQUEST")
		line = fmt.Sprintf("%%-20s\t%%4d\t%%5d\t%%6s\t%%s\t%%s\t%%s  %%s\n")
	} else {
		fmt.Fprintf(w, hdr, "ID", "N", "NJOBS", "TIME", "OWNER", "JOB", "")
		line = fmt.Sprintf("%%-20s\t%%4d\t%%5d\t%%6s\t%%s\t%%s\n")
	}

	for _, r := range status.Jobs {
		runtime := fmt.Sprintf("%.1f", now.Sub(time.Unix(0, r.StartedAt)).Seconds())
		nJobs := 0
		requestName := ""
		owner := ""
		args := map[string]interface{}{}
		var argNames []string
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

					argNames = make([]string, len(request.Args))
					i := 0
					for k, v := range request.Args {
						args[k] = v
						argNames[i] = k
						i++
					}
				}
			}
		}
		jobNameLen := len(r.Name)
		if jobNameLen > JOB_COL_LEN {
			// "very_long_job_name" -> "very_long_job_..."
			r.Name = r.Name[jobNameLen-(JOB_COL_LEN-3):jobNameLen] + "..." // -3 for "..."
		}

		if c.ctx.Options.Verbose {
			sort.Strings(argNames)
			argString := ""
			for _, k := range argNames {
				v := args[k]
				val, ok := v.(string)
				if !ok {
					val = ""
				}
				argString = argString + k + "=" + val + " "
			}
			fmt.Fprintf(w, line, r.RequestId, r.N, nJobs, runtime, owner, r.Name, requestName, argString)
		} else {
			fmt.Fprintf(w, line, r.RequestId, r.N, nJobs, runtime, owner, r.Name)
		}
	}

	w.Flush()

	return nil
}
