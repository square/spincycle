package cmd

import (
	"fmt"

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

	hdr := fmt.Sprintf("%%-32s  %%4s  %%5s  %%6s  %%s\n")
	line := fmt.Sprintf("%%-32s  %%4d  %%5d  %%6s  %%s\n")
	fmt.Fprintf(c.ctx.Out, hdr, "ID", "N", "NJOBS", "TIME", "JOB")
	for _, r := range status.Jobs {
		runtime := fmt.Sprintf("%.1f", r.Runtime)
		job := r.JobId
		m := 0
		if status.Requests != nil {
			if r, ok := status.Requests[r.RequestId]; ok {
				m = r.TotalJobs
			}
		}
		if len(job) > JOB_COL_LEN {
			// "long_job_id@5" -> "..._job_id@5"
			job = "..." + job[len(job)-(JOB_COL_LEN-3):len(job)] // +3 for "..."
		}
		fmt.Fprintf(c.ctx.Out, line, r.RequestId, r.N, m, runtime, r.JobId)
	}

	return nil
}
