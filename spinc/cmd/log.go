// Copyright 2017-2019, Square, Inc.

package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/spinc/app"
)

const (
	RECORD_SEPARATOR = "--\n"
)

type jobLog []proto.JobLog

func (l jobLog) Len() int           { return len(l) }
func (l jobLog) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l jobLog) Less(i, j int) bool { return l[i].FinishedAt < l[j].FinishedAt }

type Log struct {
	ctx   app.Context
	reqId string
}

func NewLog(ctx app.Context) *Log {
	return &Log{
		ctx: ctx,
	}
}

func (c *Log) Prepare() error {
	if len(c.ctx.Command.Args) == 0 {
		return fmt.Errorf("Usage: spinc log <id>\n")
	}
	c.reqId = c.ctx.Command.Args[0]
	return nil
}

func (c *Log) Run() error {
	jl, err := c.ctx.RMClient.GetJL(c.reqId)
	if err != nil {
		return err
	}

	sort.Sort(jobLog(jl))

	if c.ctx.Hooks.CommandRunResult != nil {
		c.ctx.Hooks.CommandRunResult(jl, err)
		return nil
	}

	n := len(jl)
	for i, l := range jl {
		var (
			started  time.Time
			finished time.Time
		)
		if l.StartedAt != 0 {
			started = time.Unix(0, l.StartedAt)
		}
		if l.FinishedAt != 0 {
			finished = time.Unix(0, l.FinishedAt)
		}
		d := finished.Sub(started)

		fmt.Printf("job id:       %s\n", l.JobId)
		fmt.Printf("job name:     %s\n", l.Name)
		fmt.Printf("job type:     %s\n", l.Type)
		fmt.Printf("state:        %s\n", proto.StateName[l.State])
		fmt.Printf("exit:         %d\n", l.Exit)
		fmt.Printf("error:        %s\n", l.Error)
		fmt.Printf("try:          %d\n", l.Try)
		fmt.Printf("sequence id:  %s\n", l.SequenceId)
		fmt.Printf("sequence try: %d\n", l.SequenceTry)
		fmt.Printf("runtime:      %fs\n", d.Seconds())
		fmt.Printf("started:      %s\n", started)
		fmt.Printf("finished:     %s\n", finished)
		fmt.Printf("stdout:       %s\n", l.Stdout)
		fmt.Printf("stderr:       %s\n", l.Stderr)

		if i < n-1 {
			fmt.Print(RECORD_SEPARATOR)
		}
	}

	return nil
}

func (c *Log) Cmd() string {
	return "log " + c.reqId
}

func (c *Log) Help() string {
	return "'spin log <request ID>' prints the entire job log of the request.\n" +
		"The job log can be long, so pipe the output to less: 'spinc log <request ID> | less'.\n"
}
