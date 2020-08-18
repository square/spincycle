package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/logrusorgru/aurora"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
)

type Visualize struct {
	ctx   app.Context
	reqId string
	color bool
}

type VisualizeCmd struct {
	RequestId string `arg:"positional" help:"ID of job chain to print"`
	NoColor   bool   `arg:"--no-color" help:"don't color output (e.g. for output to files)"`
}

func visualizeUsage() string {
	parser, err := arg.NewParser(arg.Config{Program: "spinc visualize"}, &VisualizeCmd{})
	if err != nil {
		return fmt.Sprintf("Unable to retrieve help message: error in arg.Parser: %s", err)
	}

	help := &bytes.Buffer{}
	parser.WriteHelp(help)
	return help.String()
}

func NewVisualize(ctx app.Context) *Visualize {
	return &Visualize{
		ctx: ctx,
	}
}

func (c *Visualize) Prepare() error {
	cmd := VisualizeCmd{}
	parser, err := arg.NewParser(arg.Config{Program: "spinc visualize"}, &cmd)
	if err != nil {
		return fmt.Errorf("Error in arg.Parser: %s", err)
	}

	err = parser.Parse(c.ctx.Command.Args)
	if err != nil {
		return fmt.Errorf("Error parsing args: %s\n%s", err, visualizeUsage())
	}

	c.reqId = cmd.RequestId
	c.color = !cmd.NoColor
	return nil
}

func (c *Visualize) Run() error {
	jc, err := c.getJobChain(os.Args[1])
	if err != nil {
		return err
	}

	err = jc.Print()
	if err != nil {
		return err
	}

	return nil
}

func (c *Visualize) Cmd() string {
	return "visualize " + c.reqId
}

func (c *Visualize) Help() string {
	return "'spinc visualize [request ID]' prints to terminal a text visualization of [request ID]'s job chain.\n" +
		visualizeUsage()
}

/* ========================================================================== */
// A printable job chain plus metainfo with accompanying creator functions
type visualizeJC struct {
	proto.JobChain

	req proto.Request // request represented by this job chain

	reverseDependencies map[string][]string // job ID -> list of <id>s | edge (job ID, <id>) \in job chain
	inDegrees           map[string]int      // job ID -> number of in edges
	outDegrees          map[string]int      // job ID -> number of out edges

	jobHitCount map[string]int             // job ID -> number of times we've visited the job (for printing)
	jobLogs     map[string]proto.JobLog    // job ID -> info about finished job
	jobsIP      map[string]proto.JobStatus // job (in progress) ID -> status
	reqTime     int64                      // unix nano, time at which we received all this info

	output io.Writer     // where to print output
	color  aurora.Aurora // optionally colors output
}

func (c *Visualize) getJobChain(id string) (*visualizeJC, error) {

	jc, err := c.ctx.RMClient.GetJobChain(id)
	if err != nil {
		return nil, err
	}
	req, err := c.ctx.RMClient.GetRequest(id)
	if err != nil {
		return nil, err
	}

	revDep := getReverseDependencies(jc.AdjacencyList)
	outDeg := map[string]int{}
	for node, nexts := range jc.AdjacencyList {
		outDeg[node] = len(nexts)
	}
	inDeg := map[string]int{}
	for node, nexts := range revDep {
		inDeg[node] = len(nexts)
	}

	jobLogs, err := c.getJobLog(id)
	if err != nil {
		return nil, err
	}
	jobsIP, err := c.getRunningJobs(id)
	if err != nil {
		return nil, err
	}

	return &visualizeJC{
		JobChain: jc,
		req:      req,

		reverseDependencies: revDep,
		inDegrees:           inDeg,
		outDegrees:          outDeg,

		jobHitCount: map[string]int{},
		jobLogs:     jobLogs,
		jobsIP:      jobsIP,
		reqTime:     time.Now().UnixNano(),

		output: c.ctx.Out,
		color:  aurora.NewAurora(c.color),
	}, nil
}

func getReverseDependencies(forward map[string][]string) map[string][]string {
	// job id -> set of ids of reverse dependencies
	rev := map[string]map[string]bool{}

	// BFS
	// Start with nodes with no in edges
	currLevel := findStarts(forward)
	for len(currLevel) > 0 {
		nextLevel := []string{}

		// For each node at the current level...
		for _, node := range currLevel {
			nexts := forward[node]

			// Add edge (node, next)
			for _, next := range nexts {
				if rev[next] == nil {
					rev[next] = map[string]bool{}
				}
				rev[next][node] = true
			}

			nextLevel = append(nextLevel, nexts...)
		}

		currLevel = nextLevel
	}

	// Convert `rev` to expected return type
	ret := map[string][]string{}
	for key, vals := range rev {
		for val, _ := range vals {
			ret[key] = append(ret[key], val)
		}
	}

	return ret
}

// map job.id -> log
func (c *Visualize) getJobLog(requestID string) (map[string]proto.JobLog, error) {
	logs := map[string]proto.JobLog{}

	ls, err := c.ctx.RMClient.GetJL(requestID)
	if err != nil {
		return nil, err
	}
	for _, l := range ls {
		logs[l.JobId] = l
	}

	return logs, nil
}

// map jobId of in progress jobs -> job status
func (c *Visualize) getRunningJobs(requestID string) (map[string]proto.JobStatus, error) {
	jobs := map[string]proto.JobStatus{}
	rs, err := c.ctx.RMClient.Running(proto.StatusFilter{RequestId: requestID})
	if err != nil {
		return nil, err
	}
	for _, job := range rs.Jobs {
		jobs[job.JobId] = job
	}
	return jobs, nil
}

/* ========================================================================== */
// Printing logic

// Describes a job to be printed
type jobLine struct {
	id      string // ID of job
	level   int    // Indent level
	isFirst bool   // True iff it is the first job in a branch
}

// Print job chain
// Can only be called once per instance of job chain
func (jc *visualizeJC) Print() error {
	fmt.Fprintf(jc.output, " REQUEST TYPE: %s\n", jc.req.Type)
	if jc.req.Args != nil {
		for _, arg := range jc.req.Args {
			if arg.Type == "static" {
				continue
			}
			fmt.Fprintf(jc.output, "          ARG: %s=%v\n", arg.Name, arg.Value)
		}
	}

	fmt.Fprintf(jc.output, "     START |->\n")
	startNodes := findStarts(jc.AdjacencyList)
	for _, node := range startNodes {
		err := jc.printChain(jobLine{node, 0, true})
		if err != nil {
			return err
		}
	}

	final, ok := proto.StateName[jc.req.State]
	if !ok {
		final = "UNKNOWN"
	}
	fmt.Fprintf(jc.output, "     END ->| %s\n", final)

	return nil
}

// Traverse job chain
func (jc *visualizeJC) printChain(job jobLine) error {
	// Don't print until all previous nodes have been printed
	jc.jobHitCount[job.id]++
	if jc.jobHitCount[job.id] < jc.inDegrees[job.id] {
		return nil
	}

	// If there are multiple in edges, we want to step out of the indent
	// Also, don't show such a job as the first one in a fork
	if jc.inDegrees[job.id] > 1 {
		inDegree := jc.inDegrees[job.id]
		job.level = job.level - inDegree + 1
		if job.level < 0 {
			job.level = 0
		}
		job.isFirst = false
	}

	// Do the actual printing
	err := jc.printNode(job)
	if err != nil {
		return err
	}

	// Recursively traverse remainder of chain
	// If job isn't in adjacency list, nexts is the empty list
	nexts, _ := jc.AdjacencyList[job.id]
	outDegree := len(nexts) // == jc.outDegrees[job.id]
	if outDegree == 1 {
		// Case: only one next node
		// Nothing special, just print it
		err = jc.printChain(jobLine{nexts[0], job.level, false})
		if err != nil {
			return err
		}
	} else {
		// Case: fork occurs here
		// Indent by 'outDegree'. Then, if branches merge later at different
		// times, we can unindent them by their in degree, i.e. by the number of
		// merging branches, while ensuring that branches are still indented
		// more than the parent node (this one).
		for _, next := range nexts {
			err = jc.printChain(jobLine{next, job.level + outDegree - 1, true})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Print a single job (node)
func (jc *visualizeJC) printNode(j jobLine) error {
	// Add indent based on j.level
	indent := ""
	for i := 0; i < j.level; i++ {
		indent = indent + "|  "
		if i > 12 {
			break
		}
	}

	// Add prefix based on j.isFirst
	// Entire prefix is indent + prefix
	var prefix string
	if j.isFirst && jc.inDegrees[j.id] < 2 {
		prefix = indent + "|->"
	} else {
		prefix = indent + "|  "
	}

	// This is the minimum we'll print
	// In some cases, we'll add some more info
	job, ok := jc.Jobs[j.id]
	if !ok {
		return fmt.Errorf("could not find job %s in job chain jobs map", j.id)
	}
	line := fmt.Sprintf("%s (%s) [%s]", job.Name, job.Type, job.Id)

	// Determine the job state and color it appropriately
	// 'state' = UNKNOWN if job not in job log
	var stateFmt aurora.Value
	state := jc.jobLogs[job.Id].State
	stateStr, ok := proto.StateName[state]
	if !ok {
		stateStr = "UNKNOWN"
	}

	switch state {
	case proto.STATE_COMPLETE:
		stateFmt = jc.color.Green(stateStr)
	case proto.STATE_STOPPED:
		stateFmt = jc.color.Blue(stateStr)
	case proto.STATE_RUNNING:
		stateFmt = jc.color.White(stateStr).BgCyan()
	case proto.STATE_FAIL:
		stateFmt = jc.color.White(stateStr).BgRed()
	default:
		stateFmt = jc.color.Yellow(stateStr)
	}

	// jobsIP contains only running jobs
	// Running jobs also indicate length of time elapsed and runtime status
	if status, ok := jc.jobsIP[job.Id]; ok {
		stateFmt = jc.color.White(stateStr).BgCyan() // set here in case it wasn't set before
		elapsedTime, _ := time.ParseDuration(fmt.Sprintf("%dns", jc.reqTime-status.StartedAt))
		line = fmt.Sprintf("%s (T: %s) STATUS: %s", line, elapsedTime.String(), status.Status)
	}

	// Do the actual printing
	fmt.Fprintf(jc.output, "%10s %s%s\n", stateFmt, prefix, line)
	// and also print any errors
	if state == proto.STATE_FAIL {
		jc.printErr(job.Id, indent)
	}

	return nil
}

func (jc *visualizeJC) printErr(id, indent string) {
	// If job isn't in job log, only a blank line gets printed
	// (which is what we want)
	job, _ := jc.jobLogs[id]
	// Give the error messages ++indent and an arrow
	indent = "           " + indent + "|  |->"
	// Print err
	for i, line := range strings.Split(job.Error, "\n") {
		if i == 0 {
			fmt.Fprintf(jc.output, "%s %s: %s\n", indent, "ERROR", line)
		} else {
			fmt.Fprintf(jc.output, "%s %s: %s\n", indent, "      ", line)
		}
	}
	// Print stdout
	for i, line := range strings.Split(job.Stdout, "\n") {
		if i == 0 {
			fmt.Fprintf(jc.output, "%s %s: %s\n", indent, "STDOUT", line)
		} else {
			fmt.Fprintf(jc.output, "%s %s: %s\n", indent, "      ", line)
		}
	}
	// Print stderr
	for i, line := range strings.Split(job.Stderr, "\n") {
		if i == 0 {
			fmt.Fprintf(jc.output, "%s %s: %s\n", indent, "STDERR", line)
		} else {
			fmt.Fprintf(jc.output, "%s %s: %s\n", indent, "      ", line)
		}
	}
}

/* ========================================================================== */

// Given adjacency list, finds starting node(s) of a graph, i.e. node(s) with no in edges
func findStarts(al map[string][]string) []string {
	starts := []string{}

	foundWithEdgesGoingIn := map[string]bool{}
	allNodes := map[string]bool{}

	for out, ins := range al {
		allNodes[out] = true
		for _, node := range ins {
			foundWithEdgesGoingIn[node] = true
			allNodes[node] = true
		}
	}

	for node, _ := range allNodes {
		if _, ok := foundWithEdgesGoingIn[node]; !ok {
			starts = append(starts, node)
		}
	}

	return starts
}
