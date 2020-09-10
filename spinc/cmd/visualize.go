package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"

	"github.com/square/spincycle/v2/print-graph"
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
)

const visualizeUsage = "spinc visualize <request-id> [color=value]"

type Visualize struct {
	ctx   app.Context
	reqId string
	color bool
}

func NewVisualize(ctx app.Context) *Visualize {
	return &Visualize{
		ctx: ctx,
	}
}

func (c *Visualize) Prepare() error {
	args := c.ctx.Command.Args
	n := len(args)
	if n == 0 {
		return fmt.Errorf("Usage: %s", visualizeUsage)
	}
	c.reqId = args[0]
	c.color = true // Turn color on by default
	if n == 2 {
		split := strings.Split(args[1], "=")
		if len(split) != 2 {
			return fmt.Errorf("Invalid command arg %s: expected arg of form color=value (should contain exactly one '=')", args[1])
		}
		color := strings.ToLower(split[1])
		switch color {
		case "off":
			c.color = false
		case "on":
			c.color = true
		default:
			return fmt.Errorf("Invalid value %s in arg color=value: expected 'on' or 'off'", color)
		}
	}
	return nil
}

func (c *Visualize) Run() error {
	jc, err := c.getJobChain()
	if err != nil {
		return err
	}

	// Print request type and values of non-static sequence args
	fmt.Fprintf(jc.output, " REQUEST TYPE: %s\n", jc.req.Type)
	if jc.req.Args != nil {
		for _, arg := range jc.req.Args {
			if arg.Type == "static" {
				continue
			}
			fmt.Fprintf(jc.output, "          ARG: %s=%v\n", arg.Name, arg.Value)
		}
	}

	// Print job chain
	jobs := map[string]bool{}
	for id, _ := range jc.Jobs {
		jobs[id] = true
	}
	g := graph.New(jobs, jc.AdjacencyList)

	// Error messages to be printed after a failed job
	// A single job may correspond to multiple lines of graph output. (See
	// graph package for more details. Specifically, the large block comment
	// describing graph states.) We'll print these messages after the line
	// containing the job info, but before we move onto the next job, and add
	// padding if we need to.
	var errorLinesToPrint []string

	// 0123456789 <graph line>[additional info]
	// E.g.:
	//       FAIL | *-. \     job-name (job-type) [job-id]
	//            | |\ \ \        ERROR: error message
	fmtOutput := "%10s %s%s\n"

	// While there are more graph lines to output...
	for g.NextLine() {
		if g.State == graph.STATE_PADDING {
			// Finish printing all errors before moving on. We only
			// move into this state between jobs (specifically, right
			// after we finish printing all the lines for a job), so
			// this is our last chance.
			// Since PADDING is, well, padding, we can print it as
			// many times as we want without ruining the graph output.
			graphLine, _ := g.GetLine()
			for _, errorLine := range errorLinesToPrint {
				fmt.Fprintf(jc.output, fmtOutput, "", graphLine, errorLine)
			}
			errorLinesToPrint = []string{}
			continue
		}

		// Get the next graph line and print it
		graphLine, id := g.GetLine()

		// Case: graph line contains no job; it's just some filler stuff
		// to connect up the jobs properly.
		if id == nil {
			// We might have an error message to output.
			var errorLine string
			if len(errorLinesToPrint) != 0 {
				errorLine = errorLinesToPrint[0]
				errorLinesToPrint = errorLinesToPrint[1:]
			}
			fmt.Fprintf(jc.output, fmtOutput, "", graphLine, errorLine)
			continue
		}

		// Case: graph line contains a job; format the job info output.

		// This is the minimum we'll print. In some cases, we'll add some
		// more info.
		job, ok := jc.Jobs[*id]
		if !ok {
			return fmt.Errorf("could not find job %s in job chain jobs map", *id)
		}
		jobLine := fmt.Sprintf("%s (%s) [%s]", job.Name, job.Type, job.Id)

		// Determine the job state and color it appropriately
		state := jc.jobLogs[job.Id].State
		stateLine := jc.getStateStr(state)

		// jobsIP contains only running jobs. For running jobs, we also
		// want to indicate length of time elapsed and runtime status.
		if status, ok := jc.jobsIP[job.Id]; ok {
			// We don't retrieve the job logs and the list of running jobs at
			// exactly the same time, so they might be slightly different.
			// If a job shows up in the map of in-progress jobs, we'll just
			// call it running.
			// Update the state formatting string here.
			stateLine = jc.getStateStr(proto.STATE_RUNNING)
			elapsedTime, _ := time.ParseDuration(fmt.Sprintf("%dns", jc.reqTime-status.StartedAt))
			jobLine = fmt.Sprintf("%s (T: %s) STATUS: %s", jobLine, elapsedTime.String(), status.Status)
		}

		// Do the actual printing
		fmt.Fprintf(jc.output, fmtOutput, stateLine, graphLine, jobLine)

		// If any errors occurred, record that we need to print them.
		// errLinesToPrint should be empty, since we print all a job's
		// errors before we move onto the next job.
		if state == proto.STATE_FAIL {
			errorLinesToPrint = jc.errorLines(*id)
		}
	}

	// Print final state of request
	final := jc.getStateStr(jc.req.State)
	fmt.Fprintf(jc.output, "REQUEST STATE: %s\n", final)

	return nil
}

func (c *Visualize) Cmd() string {
	return "visualize " + c.reqId
}

func (c *Visualize) Help() string {
	return fmt.Sprintf(`'%s' prints to terminal a text visualization of <request-id>'s job chain.

Args:
  color        "on" for color, "off" for no color (e.g. for output to a text file)
`, visualizeUsage)
}

/* -------------------------------------------------------------------------- */
// visualizeJC is a job chain plus metainfo that we'll print alongside the job
// chain.
type visualizeJC struct {
	proto.JobChain

	req proto.Request // request represented by this job chain

	jobLogs map[string]proto.JobLog    // job ID -> info about finished job
	jobsIP  map[string]proto.JobStatus // job (in progress) ID -> status
	reqTime int64                      // unix nano, time at which we received all this info

	output io.Writer     // where to print output
	color  aurora.Aurora // optionally colors output
}

func (c *Visualize) getJobChain() (*visualizeJC, error) {
	jc, err := c.ctx.RMClient.GetJobChain(c.reqId)
	if err != nil {
		return nil, err
	}
	req, err := c.ctx.RMClient.GetRequest(c.reqId)
	if err != nil {
		return nil, err
	}

	jobLogs := map[string]proto.JobLog{}
	ls, err := c.ctx.RMClient.GetJL(c.reqId)
	if err != nil {
		return nil, err
	}
	for _, l := range ls {
		jobLogs[l.JobId] = l
	}

	jobsIP := map[string]proto.JobStatus{}
	rs, err := c.ctx.RMClient.Running(proto.StatusFilter{RequestId: c.reqId})
	if err != nil {
		return nil, err
	}
	for _, job := range rs.Jobs {
		jobsIP[job.JobId] = job
	}

	return &visualizeJC{
		JobChain: jc,
		req:      req,

		jobLogs: jobLogs,
		jobsIP:  jobsIP,
		reqTime: time.Now().UnixNano(),

		output: c.ctx.Out,
		color:  aurora.NewAurora(c.color),
	}, nil
}

func (jc *visualizeJC) getStateStr(state byte) aurora.Value {
	stateStr, ok := proto.StateName[state]
	if !ok {
		stateStr = "UNKNOWN"
	}

	var stateLine aurora.Value

	switch state {
	case proto.STATE_COMPLETE:
		stateLine = jc.color.Green(stateStr)
	case proto.STATE_STOPPED:
		stateLine = jc.color.Blue(stateStr)
	case proto.STATE_RUNNING:
		stateLine = jc.color.White(stateStr).BgCyan()
	case proto.STATE_FAIL:
		stateLine = jc.color.White(stateStr).BgRed()
	default:
		stateLine = jc.color.Yellow(stateStr)
	}

	return stateLine
}

func (jc *visualizeJC) errorLines(id string) []string {
	errorLines := []string{}

	// If job isn't in job log, only a blank line gets printed
	// (which is what we want)
	joblog, _ := jc.jobLogs[id]

	fmtStr := "    %s %s"
	// Print err
	for i, line := range strings.Split(joblog.Error, "\n") {
		if i == 0 {
			errorLines = append(errorLines, fmt.Sprintf(fmtStr, "ERROR: ", line))
		} else {
			errorLines = append(errorLines, fmt.Sprintf(fmtStr, "       ", line))
		}
	}
	// Print stdout
	for i, line := range strings.Split(joblog.Stdout, "\n") {
		if i == 0 {
			errorLines = append(errorLines, fmt.Sprintf(fmtStr, "STDOUT:", line))
		} else {
			errorLines = append(errorLines, fmt.Sprintf(fmtStr, "       ", line))
		}
	}
	// Print stderr
	for i, line := range strings.Split(joblog.Stderr, "\n") {
		if i == 0 {
			errorLines = append(errorLines, fmt.Sprintf(fmtStr, "STDERR:", line))
		} else {
			errorLines = append(errorLines, fmt.Sprintf(fmtStr, "       ", line))
		}
	}

	return errorLines
}
