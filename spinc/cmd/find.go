// Copyright 2020, Square, Inc.

package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
)

const (
	findLimitDefault = 10 // limit to 10 requests by default

	// formatting for outputing request info
	findReqColLen   = 40
	findIdColLen    = 20
	findUserColLen  = 9
	findStateColLen = 9

	findTimeFmt    = "YYYY-MM-DD HH:MM:SS UTC" // expected time input format
	findTimeFmtStr = "2006-01-02 15:04:05 MST" // expected time input format as the actual format string (input to time.Parse)
)

var (
	findTimeColLen = len(findTimeFmt)
	findUtcIndex   = strings.Index(findTimeFmt, "UTC")
)

type Find struct {
	ctx app.Context

	local  bool
	filter proto.RequestFilter
}

func NewFind(ctx app.Context) *Find {
	return &Find{
		ctx: ctx,
	}
}

func (c *Find) Prepare() error {
	/* Parse. */
	// See command usage for details about each filter
	validArgs := map[string]bool{
		"timezone": true,

		"type":   true,
		"states": true,
		"user":   true,
		"since":  true,
		"until":  true,
		"limit":  true,
		"offset": true,
	}
	args := map[string]string{}
	for _, arg := range c.ctx.Command.Args {
		split := strings.SplitN(arg, "=", 2)
		if len(split) != 2 {
			return fmt.Errorf("Invalid command arg: %s: split on = produced %d values, expected 2 (filter=value)", arg, len(split))
		}
		arg := split[0]
		value := split[1]

		if !validArgs[arg] {
			return fmt.Errorf("Invalid arg '%s'", arg)
		}
		if _, ok := args[arg]; ok {
			return fmt.Errorf("Filter '%s' specified multiple times", arg)
		}
		args[arg] = value

		if c.ctx.Options.Debug {
			app.Debug("arg '%s'='%s'", arg, value)
		}
	}

	/* Process some args. */
	var err error

	local := false
	switch args["timezone"] {
	case "":
	case "utc":
	case "local":
		local = true
	default:
		return fmt.Errorf("Invalid timezone '%s': expected 'utc' or 'local'", args["timezone"])
	}

	states := []byte{}
	if len(args["states"]) > 0 {
		for _, state := range strings.Split(args["states"], ",") {
			val, ok := proto.StateValue[state]
			if !ok {
				expected := make([]string, 0, len(proto.StateValue))
				for k, _ := range proto.StateValue {
					expected = append(expected, k)
				}
				return fmt.Errorf("Invalid state '%s', expected one of: %s", state, strings.Join(expected, ", "))
			}
			states = append(states, val)
		}
	}

	var since time.Time
	if args["since"] != "" {
		if strings.Index(args["since"], "UTC") != findUtcIndex {
			return fmt.Errorf("Invalid time %s, expected string 'UTC' at index %d", args["since"], findUtcIndex)
		}
		since, err = time.Parse(findTimeFmtStr, args["since"])
		if err != nil {
			return fmt.Errorf("Invalid time %s, expected form '%s'", args["since"], findTimeFmt)
		}
	}

	var until time.Time
	if args["until"] != "" {
		if strings.Index(args["until"], "UTC") != findUtcIndex {
			return fmt.Errorf("Invalid time %s, expected string 'UTC' at index %d", args["until"], findUtcIndex)
		}
		until, err = time.Parse(findTimeFmtStr, args["until"])
		if err != nil {
			return fmt.Errorf("Invalid time %s, expected form '%s'", args["until"], findTimeFmt)
		}
	}

	var limit uint
	if args["limit"] == "" {
		limit = findLimitDefault
	} else {
		l, err := strconv.ParseUint(args["limit"], 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("Invalid limit '%s', expected value >= 0", args["limit"])
		}
		limit = uint(l)
	}

	var offset uint
	if args["offset"] != "" {
		o, err := strconv.ParseUint(args["offset"], 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("Invalid offset '%s', expected value >= 0", args["offset"])
		}
		offset = uint(o)
	}

	/* Save args. */
	c.local = local
	c.filter = proto.RequestFilter{
		Type:   args["type"],
		States: states,
		User:   args["user"],

		Since: since,
		Until: until,

		Limit:  limit,
		Offset: offset,
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
	   ID                   REQUEST                                  USER      STATE     CREATED STARTED FINISHED JOBS
	   -------------------- 1234567890123456789012345678901234567890 123456789 123456789 ------- ------- -------- *
	*/
	line := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%s\n",
		findIdColLen, findReqColLen, findUserColLen, findStateColLen, findTimeColLen, findTimeColLen, findTimeColLen)

	fmt.Fprintf(c.ctx.Out, line, "ID", "REQUEST", "USER", "STATE", "CREATED", "STARTED", "FINISHED", "JOBS")

	timeConv := (time.Time).UTC
	if c.local {
		timeConv = (time.Time).Local
	}

	for _, r := range requests {
		state, ok := proto.StateName[r.State]
		if !ok {
			state = proto.StateName[proto.STATE_UNKNOWN]
		}

		createdAt := timeConv(r.CreatedAt).Format(findTimeFmtStr)

		startedAt := "N/A"
		if r.StartedAt != nil {
			startedAt = timeConv(*r.StartedAt).Format(findTimeFmtStr)
		}

		finishedAt := "N/A"
		if r.FinishedAt != nil {
			finishedAt = timeConv(*r.FinishedAt).Format(findTimeFmtStr)
		}

		jobs := fmt.Sprintf("%d / %d", r.FinishedJobs, r.TotalJobs)

		fmt.Fprintf(c.ctx.Out, line,
			SqueezeString(r.Id, findIdColLen, ".."),
			SqueezeString(r.Type, findReqColLen, ".."),
			SqueezeString(r.User, findUserColLen, ".."),
			SqueezeString(state, findStateColLen, ".."),
			createdAt, startedAt, finishedAt,
			jobs)
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
	return fmt.Sprintf(`'spinc find [arg=value] [filter=value]' retrieves and filters requests.

Args:
  timezone    timezone to use in output ('utc' | 'local')

Filters:
  type        type of request to return
  states      comma-separated list of request states to include
  user        return only requests made by this user
  since       return requests created or run after this time
  until       return requests created or run before this time
  limit       limit response to this many requests (default: %d)
  offset      skip the first <offset> requests

Times should be formated as '%s'. Time should be specified in UTC.
`, findLimitDefault, findTimeFmt)
}
