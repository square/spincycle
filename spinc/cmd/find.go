// Copyright 2020, Square, Inc.

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/alexflint/go-arg"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
)

const (
	// formatting for outputing request info
	findReqColLen   = 20
	findIdColLen    = 20
	findUserColLen  = 9
	findStateColLen = 9
	findJobsColLen  = 15

	findTimeFmt    = "YYYY-MM-DD HH:MM:SS UTC" // expected time input format
	findTimeFmtStr = "2006-01-02 15:04:05 UTC" // expected time input format as the actual format string (input to time.Parse)
)

var (
	findTimeColLen = len(findTimeFmt)
)

type Find struct {
	ctx    app.Context
	filter proto.RequestFilter
}

// Expected arguments to this command
type FindCmd struct {
	Type   string   `help:"type of request to return"`
	States []string `help:"request states to include"`
	User   string   `help:"user who made the request"`

	Since string `help:"return requests created or run after this time (format: %s)"`
	Until string `help:"return requests created or run before this time (format; %s)"`

	Limit  uint `help:"limit response to this many requests"`
	Offset uint `help:"skip the first <offset> requests, ignored if <limit> not set"`
}

func NewFind(ctx app.Context) *Find {
	return &Find{
		ctx: ctx,
	}
}

func (c *Find) Prepare() error {
	/* Parse. */
	cmd := FindCmd{}
	parser, err := arg.NewParser(arg.Config{Program: "spinc find"}, &cmd)
	if err != nil {
		return fmt.Errorf("Error in arg.Parser: %s", err)
	}

	err = parser.Parse(c.ctx.Command.Args)
	if err != nil {
		return fmt.Errorf("Error parsing args: %s", err)
	}

	/* Further process some args. */
	states := make([]byte, 0, len(cmd.States))
	for _, state := range cmd.States {
		val, ok := proto.StateValue[state]
		if !ok {
			expected := make([]string, 0, len(proto.StateValue))
			for k, _ := range proto.StateValue {
				expected = append(expected, k)
			}
			return fmt.Errorf("Invalid state %s, expected one of: %s", state, strings.Join(expected, ", "))
		}
		states = append(states, val)
	}

	var since time.Time
	if cmd.Since != "" {
		since, err = time.Parse(findTimeFmtStr, cmd.Since)
		if err != nil {
			return fmt.Errorf("Invalid time %s, expected form %s", cmd.Since, findTimeFmt)
		}
	}

	var until time.Time
	if cmd.Until != "" {
		until, err = time.Parse(findTimeFmtStr, cmd.Until)
		if err != nil {
			return fmt.Errorf("Invalid time %s, expected form %s", cmd.Since, findTimeFmt)
		}
	}

	/* Build the request filter. */
	c.filter = proto.RequestFilter{
		Type:   cmd.Type,
		States: states,
		User:   cmd.User,

		Since: since,
		Until: until,

		Limit:  cmd.Limit,
		Offset: cmd.Offset,
	}

	return nil
}

func (c *Find) Run() error {
	requests, err := c.ctx.RMClient.FindRequests(c.filter)
	if err != nil {
		return err
	}
	if c.ctx.Options.Debug {
		app.Debug("requests: %#v", requests)
	}

	if c.ctx.Hooks.CommandRunResult != nil {
		c.ctx.Hooks.CommandRunResult(requests, err)
		return nil
	}

	if len(requests) == 0 {
		return nil
	}

	/*
	   REQUEST              ID                    USER      STATE     CREATED STARTED FINISHED JOBS            HOST
	   12345678901234567890 --------------------  123456789 123456789 ------- ------- -------- 123456789012345 *
	*/
	line := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%s\n",
		findReqColLen, findIdColLen, findUserColLen, findStateColLen, findTimeColLen, findTimeColLen, findTimeColLen, findJobsColLen)

	fmt.Fprintf(c.ctx.Out, line, "REQUEST", "ID", "USER", "STATE", "CREATED", "STARTED", "FINISHED", "JOBS", "HOST")

	for _, r := range requests {
		state, ok := proto.StateName[r.State]
		if !ok {
			state = proto.StateName[proto.STATE_UNKNOWN]
		}

		createdAt := r.CreatedAt.Format(findTimeFmtStr)

		startedAt := "N/A"
		if r.StartedAt != nil {
			startedAt = r.StartedAt.Format(findTimeFmtStr)
		}

		finishedAt := "N/A"
		if r.FinishedAt != nil {
			finishedAt = r.FinishedAt.Format(findTimeFmtStr)
		}

		jobs := fmt.Sprintf("%d / %d", r.FinishedJobs, r.TotalJobs)

		fmt.Fprintf(c.ctx.Out, line,
			SqueezeString(r.Type, findReqColLen, ".."),
			SqueezeString(r.Id, findIdColLen, ".."),
			SqueezeString(r.User, findUserColLen, ".."),
			SqueezeString(state, findStateColLen, ".."),
			createdAt, startedAt, finishedAt,
			SqueezeString(jobs, findJobsColLen, ".."),
			r.JobRunnerURL)
	}

	return nil
}

func (c *Find) Cmd() string {
	if len(c.ctx.Command.Args) > 0 {
		return "find " + strings.Join(c.ctx.Command.Args, " ")
	}
	return "find"
}

func (c *Find) Help() string {
	return "'spinc find <args>' retrieves and filters requests.\n" +
		findUsage()
}

func findUsage() string {
	parser, err := arg.NewParser(arg.Config{Program: "spinc find"}, &FindCmd{})
	if err != nil {
		return fmt.Sprintf("Unable to retrieve help message: error in arg.Parser: %s", err)
	}

	help := &bytes.Buffer{}
	parser.WriteHelp(help)
	return fmt.Sprintf(help.String(), findTimeFmt, findTimeFmt)
}
