package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"

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
	jc, err := c.getJobChain(c.reqId)
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
	return fmt.Sprintf(`'%s' prints to terminal a text visualization of <request-id>'s job chain.

Args:
  color        "on" for color, "off" for no color (e.g. for output to a text file)
`, visualizeUsage)
}

/* ========================================================================== */
// A printable job chain plus metainfo with accompanying creator functions
type visualizeJC struct {
	proto.JobChain

	req proto.Request // request represented by this job chain

	jobLogs map[string]proto.JobLog    // job ID -> info about finished job
	jobsIP  map[string]proto.JobStatus // job (in progress) ID -> status
	reqTime int64                      // unix nano, time at which we received all this info

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

		jobLogs: jobLogs,
		jobsIP:  jobsIP,
		reqTime: time.Now().UnixNano(),

		output: c.ctx.Out,
		color:  aurora.NewAurora(c.color),
	}, nil
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

	jobs := map[string]bool{}
	for id, _ := range jc.Jobs {
		jobs[id] = true
	}
	g := NewGraph(jobs, jc.AdjacencyList)
	for g.NextLine() {
		graphLine, id := g.GetLine()
		if id == nil {
			fmt.Fprintf(jc.output, "%10s %s\n", "", graphLine)
		} else {
			jc.printJobLine(graphLine, *id)
		}
	}

	final, ok := proto.StateName[jc.req.State]
	if !ok {
		final = "UNKNOWN"
	}
	fmt.Fprintf(jc.output, "     END ->| %s\n", final)

	return nil
}

func (jc *visualizeJC) printJobLine(graph, id string) error {
	// This is the minimum we'll print
	// In some cases, we'll add some more info
	job, ok := jc.Jobs[id]
	if !ok {
		return fmt.Errorf("could not find job %s in job chain jobs map", id)
	}
	jobLine := fmt.Sprintf("%s (%s) [%s]", job.Name, job.Type, job.Id)

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
		jobLine = fmt.Sprintf("%s (T: %s) STATUS: %s", jobLine, elapsedTime.String(), status.Status)
	}

	// Do the actual printing
	fmt.Fprintf(jc.output, "%10s %s%s\n", stateFmt, graph, jobLine)
	// and also print any errors
	// TODO: git graph needs to be able to output padding
	//	if state == proto.STATE_FAIL {
	//		jc.printErr(job.Id, indent)
	//	}

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

const (
	GRAPH_PADDING  byte = 0
	GRAPH_PRE_JOB  byte = 1
	GRAPH_JOB      byte = 2
	GRAPH_BRANCH   byte = 3
	GRAPH_COLLAPSE byte = 4
)

type graph struct {
	Vertices map[string]bool
	Edges    map[string][]string
	RevEdges map[string][]string

	CurrId               *string
	Width                int
	ExpansionRows        int
	State                byte
	PrevState            byte
	IdIndex              int
	PrevIdIndex          int
	MergeLayout          int
	EdgesAdded           int
	PrevEdgesAdded       int
	Cols                 []string
	NewCols              []string
	Mapping              []int
	HorizontalEdge       int
	HorizontalEdgeTarget int

	ToVisit []string
	Seen    map[string]bool
}

func findId(arr []string, id string) int {
	for i, elt := range arr {
		if elt == id {
			return i
		}
	}
	return -1
}

func getRevEdges(edges map[string][]string) map[string][]string {
	revs := map[string][]string{}
	for vertex, nexts := range edges {
		for _, next := range nexts {
			revs[next] = append(revs[next], vertex)
		}
	}
	return revs
}

func getFirsts(vertices map[string]bool, edges map[string][]string) []string {
	ins := map[string]bool{}
	for _, nexts := range edges {
		for _, next := range nexts {
			ins[next] = true
		}
	}

	firsts := []string{}
	for vertex, _ := range vertices {
		if !ins[vertex] {
			firsts = append(firsts, vertex)
		}
	}

	return firsts
}

func NewGraph(vertices map[string]bool, edges map[string][]string) *graph {
	return &graph{
		Vertices: vertices,
		Edges:    edges,
		RevEdges: getRevEdges(edges),

		State:     GRAPH_PADDING,
		PrevState: GRAPH_PADDING,

		ToVisit: getFirsts(vertices, edges),
		Seen:    map[string]bool{},
	}
}

func (g *graph) updateState(state byte) {
	g.PrevState = g.State
	g.State = state
}

// Get next nodes of node `g.CurrId`
func (g *graph) nexts() []string {
	if g.CurrId == nil {
		return []string{}
	}
	return g.Edges[*g.CurrId]
}

func (g *graph) numDashedParents() int {
	return len(g.nexts()) + g.MergeLayout - 3
}

func (g *graph) numExpansionRows() int {
	return g.numDashedParents() * 2
}

func (g *graph) needsPreJobLine() bool {
	return len(g.nexts()) >= 3 && g.IdIndex < len(g.Cols)-1 && g.ExpansionRows < g.numExpansionRows()
}

func (g *graph) mappingCorrect() bool {
	for i, target := range g.Mapping {
		if target >= 0 && target != i/2 {
			return false
		}
	}
	return true
}

func (g *graph) insertIntoNewCols(id string, parentIdx int) {
	var mappingIdx int
	i := findId(g.NewCols, id)
	numNexts := len(g.nexts())

	// If commit isn't already in NewCols, append it
	if i < 0 {
		i = len(g.NewCols)
		g.NewCols = append(g.NewCols, id)
	}

	if numNexts > 1 && parentIdx != -1 && g.MergeLayout == -1 {
		// Case: first next of current node
		// Choose layout based on whether it's to the left or right of current job
		dist := parentIdx - i

		g.MergeLayout = 1
		if dist > 0 {
			g.MergeLayout = 0
		}
		g.EdgesAdded = numNexts + g.MergeLayout - 2

		shift := 1
		if dist > 1 {
			shift = 2*dist - 3
		}
		mappingIdx = g.Width + (g.MergeLayout-1)*shift
		g.Width += 2 * g.MergeLayout
	} else { // TODO: you skipped a case here for simplicity's sake
		mappingIdx = g.Width
		g.Width += 2
	}

	g.Mapping[mappingIdx] = i
}

func (g *graph) updateCols() {
	g.Cols = g.NewCols
	g.Width = 0
	g.PrevEdgesAdded = g.EdgesAdded
	g.EdgesAdded = 0

	// We'll have at most the current number of cols, plus the number of new cols
	maxNewCols := len(g.Cols) + len(g.nexts())
	g.NewCols = make([]string, 0, maxNewCols)

	// Clear out mapping and ensure sufficient capacity (we'll trim later)
	g.Mapping = make([]int, 2*maxNewCols)
	for i, _ := range g.Mapping {
		g.Mapping[i] = -1
	}

	seen := false
	for i := 0; i <= len(g.Cols); i++ {
		var id string
		if i == len(g.Cols) {
			if seen {
				break
			}
			id = *g.CurrId
		} else {
			id = g.Cols[i]
		}

		if id == *g.CurrId {
			seen = true
			g.IdIndex = i
			g.MergeLayout = -1
			for _, next := range g.nexts() {
				g.insertIntoNewCols(next, i)
			}
			if len(g.nexts()) == 0 {
				g.Width += 2
			}
		} else {
			g.insertIntoNewCols(id, -1)
		}
	}

	// Trim `g.Mapping`
	size := len(g.Mapping)
	for size > 1 && g.Mapping[size-1] < 0 {
		size -= 1
	}
	g.Mapping = g.Mapping[:size]
}

func (g *graph) nextJob() bool {

	// Get next job to print
	g.CurrId = nil
	for len(g.ToVisit) != 0 {
		// Pop from queue
		id := g.ToVisit[0]
		g.ToVisit = g.ToVisit[1:]

		// It's possible we've already seen it
		if g.Seen[id] {
			continue
		}

		// Make sure we've actually seen all of its dependencies
		missingDep := false
		for _, prev := range g.RevEdges[id] {
			if !g.Seen[prev] {
				missingDep = true
				break
			}
		}

		// If all dependencies are satisfied, we're done
		if !missingDep {
			g.CurrId = &id
			break
		}
		// Else, we keep going
	}

	// There's no more jobs to print
	if g.CurrId == nil {
		return false
	}

	// Update/reset metainfo
	//	g.ToVisit = append(g.ToVisit, g.nexts()...)
	nexts := g.nexts()
	for i, _ := range nexts {
		g.ToVisit = append(g.ToVisit, nexts[len(nexts)-i-1])
	}
	g.Seen[*g.CurrId] = true

	g.PrevIdIndex = g.IdIndex
	g.ExpansionRows = 0

	g.updateCols()

	if g.needsPreJobLine() {
		g.updateState(GRAPH_PRE_JOB)
	} else {
		g.updateState(GRAPH_JOB)
	}

	return true
}

func (g *graph) getPaddingLine() string {
	line := ""
	for i := 0; i < len(g.NewCols); i++ {
		line += "| "
	}
	return line
}

func (g *graph) getPreJobLine() string {
	line := ""

	seen := false
	for i, col := range g.Cols {
		if col == *g.CurrId {
			seen = true
			line += "|"
			for i := 0; i < g.ExpansionRows; i++ {
				line += " "
			}
		} else if seen {
			if g.ExpansionRows == 0 {
				if g.PrevState == GRAPH_BRANCH && g.PrevIdIndex < i {
					line += "\\"
				} else {
					line += "|"
				}
			} else {
				line += "\\"
			}
		} else {
			line += "|"
		}

		line += " "
	}

	return line
}

func (g *graph) getJobLine() string {
	line := ""

	seen := false
	for i := 0; i <= len(g.Cols); i++ {
		var col string
		if i == len(g.Cols) {
			if seen {
				break
			}
			col = *g.CurrId
		} else {
			col = g.Cols[i]
		}

		if col == *g.CurrId {
			seen = true
			line += "*"

			numNexts := len(g.nexts())
			if numNexts > 2 {
				for i := 0; i < g.numDashedParents()-1; i++ {
					line += "--"
				}
				line += "-."
			}
		} else if seen { // TODO: you skipped a case for convenience's sake
			line += "|"
		} else {
			if g.PrevState == GRAPH_COLLAPSE && g.PrevEdgesAdded > 0 && g.PrevIdIndex < i {
				line += "/"
			} else {
				line += "|"
			}
		}

		line += " "
	}

	return line
}

func (g *graph) getBranchLine() string {
	line := ""

	seen := false
	// len(g.nexts()) > 0, or we wouldn't be in this state
	firstNextIndex := findId(g.Cols, g.nexts()[0])
	if firstNextIndex == -1 {
		// If it's not in cols, set this so that !exists(i) | i > firstNextIndex
		firstNextIndex = len(g.Cols) + 1
	}

	for i := 0; i <= len(g.Cols); i++ {
		var col string
		if i == len(g.Cols) {
			if seen {
				break
			}
			col = *g.CurrId
		} else {
			col = g.Cols[i]
		}

		if col == *g.CurrId {
			seen = true
			idx := g.MergeLayout

			for j, next := range g.nexts() {
				nextIndex := findId(g.NewCols, next) // nextIndex != -1
				if nextIndex == -1 {                 // TODO
					return "ERORR: next node not found in cols"
				}

				switch idx {
				case 0:
					line += "/"
				case 1:
					line += "|"
				case 2:
					line += "\\"
				}

				if idx == 2 {
					if g.EdgesAdded > 0 || j < len(g.nexts())-1 {
						line += " "
					}
				} else {
					idx++
				}
			}
		} else if seen {
			if g.EdgesAdded > 0 {
				line += "\\"
			} else {
				line += "|"
			}
			line += " "
		} else {
			line += "|"
			if g.MergeLayout != 0 || i != g.IdIndex-1 {
				if i > firstNextIndex {
					line += "_"
				} else {
					line += " "
				}
			}
		}
	}

	return line
}

func (g *graph) updateCollapse() {
	// Update/clear out mappings
	oldMapping := g.Mapping
	g.Mapping = make([]int, len(oldMapping))
	for i, _ := range g.Mapping {
		g.Mapping[i] = -1
	}

	g.HorizontalEdge = -1
	g.HorizontalEdgeTarget = -1

	for i, target := range oldMapping {
		if target < 0 {
			continue
		}

		if 2*target == i {
			// Case: this col is already in the correct place
			g.Mapping[i] = target
		} else if g.Mapping[i-1] < 0 {
			// Case: col isn't in the right place yet
			// Also, there's nothing to the left, so we
			// can shift it over by one
			g.Mapping[i-1] = target

			// TODO: I have no idea what the fuck this does
			if g.HorizontalEdge == -1 {
				g.HorizontalEdge = i
				g.HorizontalEdgeTarget = target
				// Evidently, (target*2)+3 is the (screen)
				// column of the first horizontal line
				for j := (target * 2) + 3; j < i-2; j += 2 {
					g.Mapping[j] = target
				}
			}
		} else if g.Mapping[i-1] == target {
			// Case: col isn't in the right place yet
			// There's already a branch line to the
			// left, and it's the target
			// Combine with it since we share the same parent commit
			// No need to actually do anything
		} else {
			// Case: there's already a branch line to the left,
			// but it's not the target, so cross over it
			// The space just to the left of this branch should
			// always be empty
			g.Mapping[i-2] = target

			// TODO: I have no idea what the fuck is happening
			if g.HorizontalEdge == -1 {
				g.HorizontalEdgeTarget = target
				g.HorizontalEdge = i - 1

				for j := (target * 2) + 3; j < i-2; j += 2 {
					g.Mapping[j] = target
				}
			}
		}
	}

	// `g.Mapping` might actually be one smaller than `oldMapping`
	if g.Mapping[len(g.Mapping)-1] == -1 {
		g.Mapping = g.Mapping[:len(g.Mapping)-1]
	}
}

func (g *graph) getCollapseLine() string {
	line := ""

	usedHorizontal := false

	for i, target := range g.Mapping {
		if target == -1 {
			// Case: mapping blank
			line += " "
		} else if target*2 == i {
			// Case: col is already where it needs to be
			line += "|"
		} else if target == g.HorizontalEdgeTarget && i != g.HorizontalEdge-1 {
			// TODO: the fuck?
			if i != (target*2)+3 {
				g.Mapping[i] = -1
			}
			usedHorizontal = true

			line += "_"
		} else {
			if usedHorizontal && i < g.HorizontalEdge {
				g.Mapping[i] = -1
			}
			line += "/"
		}
	}

	return line
}

func (g *graph) GetLine() (line string, id *string) {
	switch g.State {
	case GRAPH_PADDING:
		return g.getPaddingLine(), nil
	case GRAPH_PRE_JOB:
		return g.getPreJobLine(), nil
	case GRAPH_JOB:
		return g.getJobLine(), g.CurrId
	case GRAPH_BRANCH:
		return g.getBranchLine(), nil
	case GRAPH_COLLAPSE:
		return g.getCollapseLine(), nil
	}
	// This never happens; nextState always moves us into a valid graph state.
	return "", nil
}

func (g *graph) nextState() bool {
	switch g.State {
	case GRAPH_PADDING:
		return g.nextJob()
	case GRAPH_PRE_JOB:
		g.ExpansionRows++
		if !g.needsPreJobLine() {
			g.updateState(GRAPH_JOB)
		}
	case GRAPH_JOB:
		if len(g.nexts()) > 1 {
			g.updateState(GRAPH_BRANCH)
		} else if g.mappingCorrect() {
			g.updateState(GRAPH_PADDING)
		} else {
			g.updateState(GRAPH_COLLAPSE)
		}
	case GRAPH_BRANCH:
		if g.mappingCorrect() {
			g.updateState(GRAPH_PADDING)
		} else {
			g.updateState(GRAPH_COLLAPSE)
		}
	case GRAPH_COLLAPSE:
		if g.mappingCorrect() {
			g.updateState(GRAPH_PADDING)
		}
	default:
		return false
	}

	// If we're in state GRAPH_COLLAPSE, we need to also update `g.Mapping` in preparation
	if g.State == GRAPH_COLLAPSE {
		g.updateCollapse()
	}

	return true
}

func (g *graph) NextLine() bool {
	if !g.nextState() {
		return false
	}
	if g.State == GRAPH_PADDING {
		return g.NextLine()
	}
	return true
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
