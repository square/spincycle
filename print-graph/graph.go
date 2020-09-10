package graph

import (
	"fmt"
)

// graph is a state machine that allows us to print a job chain.
// This code and select comments were translated from the code git uses to pretty
// print git commit history (github.com/git/git:graph.c).
//
// Public member functions:
// - NextLine(): Moves onto the next line in the graph. Returns true if there
//  exists such a line; false if we're done.
// - GetLine(): Get the line we're currently on. Call NextLine() before calling
//   this function.
//
// Example usage:
// 	g := New(vertices, edges)
// 	for g.GetNext() {
// 		line, id := g.GetLine()
// 	 	idstr := ""
// 	 	if id != nil {
// 			idstr = *id
// 	     }
// 	     fmt.Printf("%s %s\n", line, id)
// 	}
type graph struct {
	// The graph we're printing. These are set when we create the graph and
	// never modified.
	Vertices map[string]bool
	Edges    map[string][]string
	RevEdges map[string][]string

	// ---------------------------------------------------------------------
	// State information. These are updated with calls to NextLine(), but not
	// necessarily update with every call to NextLine().
	CurrId        *string // Job we're currently printing
	Width         int     // Width of graph line
	ExpansionRows int     // How far to expand in PRE_JOB state
	State         byte    // Current state
	PrevState     byte    // Previous state
	IdIndex       int     // Index into Cols of current job
	PrevIdIndex   int     // Value of IdIndex for last job

	// Which layout variant to use to display merge commits. If the
	// commit's first parent is known to be in a column to the left of the
	// merge, then this value is 0 and we use the layout on the left.
	// Otherwise, the value is 1 and the layout on the right is used. This
	// field tells us how many columns the first parent occupies.
	//
	// 		0)			1)
	//
	// 		| | | *-.		| | *---.
	// 		| |_|/|\ \		| | |\ \ \
	// 		|/| | | | |		| | | | | *
	MergeLayout int

	// The number of columns added to the graph by the current commit. For
	// 2-way and octopus merges, this is usually one less than the
	// number of parents:
	//
	// 		| | |			| |    \
	//		| * |			| *---. \
	//		| |\ \			| |\ \ \ \
	//		| | | |         	| | | | | |
	//
	//		num_parents: 2		num_parents: 4
	//		edges_added: 1		edges_added: 3
	//
	// For left-skewed merges, the first parent fuses with its neighbor and
	// so one less column is added:
	//
	//		| | |			| |  \
	//		| * |			| *-. \
	//		|/| |			|/|\ \ \
	//		| | |			| | | | |
	//
	//		num_parents: 2		num_parents: 4
	//		edges_added: 0		edges_added: 2
	//
	// This number determines how edges to the right of the merge are
	// displayed in commit and post-merge lines; if no columns have been
	// added then a vertical line should be used where a right-tracking
	// line would otherwise be used.
	//
	//		| * \			| * |
	//		| |\ \			|/| |
	//		| | * \			| * |
	EdgesAdded     int
	PrevEdgesAdded int // Value of EdgesAdded for previous job

	// We need to know where all the jobs are in order to know what kind of
	// lines to draw (/, |, or \).
	//   | | * job 1
	//   | * | job 2
	//   * | | job 3
	// This can be encoded as [job 3, job 2, job 1].
	// Cols records the last output job of each column (including the current
	// job).
	Cols []string
	// NewCols records the next job to output in each column. The size of this
	// slice may be greater or less than the size of Cols, dependingo on the
	// current job.
	NewCols []string
	// Mapping tracks the current state of each character in the output line
	// during state STATE_COLLAPSING. Each entry is -1 if this character is
	// empty, or a non-negative integer if the character contains a branch
	// line. The value of the integer indicates the target position for this
	// branch line. (I.e., this array maps the current column positions to their
	// desired positions.) Entires indicate the target COLUMN index, not the
	// target MAPPING index.
	//
	// Empty entries (-1) correspond to a space. Non-empty entries contain
	// some character, depending on where its target position is relative to
	// its current position.
	//
	// Since each column occupies two characters in the output (/, |, \, *
	// and a space), len(Mapping) = 2 * len(Cols), and the target mapping index
	// is twice the target column index.
	//
	// Mapping is updated for every job, but it is primarily used during in
	// the STATE_COLLAPSE state.
	Mapping    []int
	OldMapping []int

	// TODO
	HorizontalEdge       int
	HorizontalEdgeTarget int

	// ---------------------------------------------------------------------
	// Keep track of which jobs to output next using BFS.
	ToVisit []string        // Queue of jobs to process
	Seen    map[string]bool // Set of processed jobs to avoid duplication
}

// Allowed states for graph. Each state STATE_X has a corresponding function
// g.getXLine(), which outputs the graph line for that state.
//
// A single job might produce multiple graph lines, both before and after the
// job line itself. The following example graph has seven jobs. Each line is
// labelled with the graph state and current job ID that output the line.
// Padding was skipped except in one instance as an example.
//
// JOB       0  *
// BRANCH    0  |\
// JOB       1  | *
// PRE_JOB   2  | |
// PRE_JOB   2  |  \
// JOB       2  *-. \
// BRANCH    2  |\ \ \
// JOB       3  | | | *
// PADDING   3  | | | |
// JOB       4  | | * |
// COLLAPSE  4  | | |/
// JOB       5  | * |
// COLLAPSE  5  | |/
// JOB       6  * |
// COLLAPSE  6  |/
// JOB       7  *
const (
	// No changes occur in this state. The graph line can be printed as many times
	// as desired and the resulting graph will still be correct.
	STATE_PADDING byte = 0

	// Lines to print before printing the job line. This happens when the job
	// splits, but the job isn't in the rightmost column.
	STATE_PRE_JOB byte = 1

	// Line containing the job (marked with a *). There is exactly one STATE_JOB
	// state per job.
	STATE_JOB byte = 2

	// Job branches into multiple paths.
	STATE_BRANCH byte = 3

	// Job joins multiple paths.
	STATE_COLLAPSE byte = 4
)

var StateName = map[byte]string{
	STATE_PADDING:  "PADDING",
	STATE_PRE_JOB:  "PRE_JOB",
	STATE_JOB:      "JOB",
	STATE_BRANCH:   "BRANCH",
	STATE_COLLAPSE: "COLLAPSE",
}

// findID returns the index of `id` in `arr`, or -1 if not found.
func findId(arr []string, id string) int {
	for i, elt := range arr {
		if elt == id {
			return i
		}
	}
	return -1
}

// revEdges returns the reverse adjacency list.
func revEdges(edges map[string][]string) map[string][]string {
	revs := map[string][]string{}
	for vertex, nexts := range edges {
		for _, next := range nexts {
			revs[next] = append(revs[next], vertex)
		}
	}
	return revs
}

// getFirsts returns vertices with no in nodes, i.e. the source(s).
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

// New returns a graph given the set of vertices and the directed
// edges. NextLine() should then be called at least once before GetLine().
func New(vertices map[string]bool, edges map[string][]string) *graph {
	return &graph{
		Vertices: vertices,
		Edges:    edges,
		RevEdges: revEdges(edges),

		State:     STATE_PADDING,
		PrevState: STATE_PADDING,

		ToVisit: getFirsts(vertices, edges),
		Seen:    map[string]bool{},
	}
}

func (g *graph) updateState(state byte) {
	g.PrevState = g.State
	g.State = state
}

// nexts returns next nodes of node `g.CurrId`.
func (g *graph) nexts() []string {
	if g.CurrId == nil {
		return []string{}
	}
	return g.Edges[*g.CurrId]
}

// numDashedParents returns the number of dashes we need to draw given the
// number of children the current job has and the merge layout we're using.
// See comment for `graph.MergeLayout` for examples.
func (g *graph) numDashedParents() int {
	return len(g.nexts()) + g.MergeLayout - 3
}

// numExpansionRows returns how many pre-job rows we need.
func (g *graph) numExpansionRows() int {
	return g.numDashedParents() * 2
}

// needsPreJobLine determines whether we need pre-job lines at all. It's called
// when we first move onto a new job, and subsequently whenever we're in STATE_PRE_JOB.
func (g *graph) needsPreJobLine() bool {
	return len(g.nexts()) >= 3 && g.IdIndex < len(g.Cols)-1 && g.ExpansionRows < g.numExpansionRows()
}

// mappingCorrect determines whether all columns have arrived at their target. Used
// to determine whether we need to move to/remain in STATE_COLLAPSE.
func (g *graph) mappingCorrect() bool {
	for i, target := range g.Mapping {
		// Mapping[i] = -1 if it's empty (i.e. is a space).
		// Since entries indicate target column index, which is half the
		// target mapping index, an entry i is correct iff Mapping[i] = i/2.
		if target >= 0 && target != i/2 {
			return false
		}
	}
	return true
}

// insertIntoNewCols is a helper function to updateCols that inserts a specific
// job into NewCols.
//
// This function does double duty.
// If parentIdx == -1, it appends the new job if it's not already been added to
// NewCols (i.e. the job has multiple parents), or sets Mappping appropriately
// if it has.
// If parentIdx != -1, then we're inserting the nexts of the new job. For the first
// next, we determine the MergeLayout. Subsequence nexts behave like the case of
// parentIdx == -1.
//
// Note that in updateCols, we iterate in order through Cols and call this function
// for each element. Thus, in the simple case where the current job has exactly
// one next, this function just copies Cols into NewCols, except the current job
// is replaced with its next.
func (g *graph) insertIntoNewCols(id string, parentIdx int) {
	// Find the commit, which may already be in NewCols e.g. if it has another
	// parent that has already been processed. In that case, that parent added id
	// to NewCols. Then, it was copied over into NewCols again by this function,
	// etc. until now, and we're trying to add it again because it's a child of
	// the current job.
	i := findId(g.NewCols, id)
	numNexts := len(g.nexts())
	// If commit isn't already in NewCols, append it.
	// This is the usual case.
	if i < 0 {
		i = len(g.NewCols)
		g.NewCols = append(g.NewCols, id)
	}

	// Index into g.Mapping, which we'll set depending on where we insert this job.
	var mappingIdx int
	if numNexts > 1 && parentIdx != -1 && g.MergeLayout == -1 {
		// Case: id is the leftmost next of current job. We decide here which layout
		// to use depending on whether id is to the left or right of the current job.
		//
		// This code from git does some magic to avoid branching, which I'm keeping
		// because it's pretty neat, but easy to write out as if/elses to see what's
		// actually happening.
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
	} else if g.EdgesAdded > 0 && i == g.Mapping[g.Width-2] {
		// If some columns have been added by a merge, but this commit
		// was found in the last existing column, then adjust the
		// numbers so that the two edges immediately join, i.e.:
		//
		//     * |           * |
		//     |\ \    =>    |\|
		//     | |/          | *
		//     | *
		mappingIdx = g.Width - 2
		g.EdgesAdded = -1
	} else {
		mappingIdx = g.Width
		g.Width += 2
	}

	g.Mapping[mappingIdx] = i
}

// updateCols resets and updates state variables in preparation for a new job.
func (g *graph) updateCols() {
	g.Cols = g.NewCols
	g.Width = 0
	g.PrevEdgesAdded = g.EdgesAdded
	g.EdgesAdded = 0

	// We'll have at most the current number of cols, plus the number of new cols
	maxNewCols := len(g.Cols) + len(g.nexts())
	g.NewCols = make([]string, 0, maxNewCols)

	// Clear out mapping and ensure sufficient capacity (we'll trim later)
	g.OldMapping = g.Mapping
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

// nextJob updates CurrId with the next job to process and updates+resets
// metainfo. nextJob also contains the BFS logic that determines the order
// in which jobs are processed.
// This function should be called after creating a graph and before any calls
// to get graph lines.
func (g *graph) nextJob() bool {
	// Get next job to print
	g.CurrId = nil
	for len(g.ToVisit) != 0 {
		// Pop from queue
		id := g.ToVisit[0]
		g.ToVisit = g.ToVisit[1:]

		// It's possible we've already seen it if the job has multiple parents
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
	nexts := g.nexts()
	for i, _ := range nexts {
		g.ToVisit = append(g.ToVisit, nexts[len(nexts)-i-1])
	}
	g.Seen[*g.CurrId] = true

	g.PrevIdIndex = g.IdIndex
	g.ExpansionRows = 0

	g.updateCols()

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
		} else if seen && g.ExpansionRows == 0 {
			// This is the first line of the pre-job output. If the previous job
			// branched into multiple nexts and ended in the STATE_BRANCH state,
			// all branch lines after PrevIdIndex were printed as "\" on the previous
			// line.  Continue to print them as "\" on this line.  Otherwise, print
			// the branch lines as "|".
			if g.PrevState == STATE_BRANCH && g.PrevIdIndex < i {
				line += "\\"
			} else {
				line += "|"
			}
		} else if seen && g.ExpansionRows > 0 {
			line += "\\"
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
	// Iterate up to and including len(g.Cols), since the current job may not be
	// in any of the existing columns, i.e. when the current job doesn't have any
	// parents that have already been processed.
	// As of 2020.09.18, job chains have exactly one source and exactly one sink.
	// However, this may change in the future, e.g. in an effort to eliminate the
	// need for noops in the RM code.
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
		} else if seen && g.EdgesAdded > 1 {
			line += "\\"
		} else if seen && g.EdgesAdded == 1 {
			// This is either a right-skewed 2-way branching job, or a left-skewed
			// 3-way branching job. There is no STATE_PRE_JOB stage for such jobs,
			// so this is the first line of output for this job.  Check to see what
			// the previous line of output was.
			//
			// If it was STATE_BRANCH, the branch line coming into this job may
			// have been '\', and not '|' or '/'.  If so, output the branch line
			// as '\' on this line, instead of '|'.  This makes the output look
			// nicer.
			if g.PrevState == STATE_BRANCH && g.PrevEdgesAdded > 0 && g.PrevIdIndex < i {
				line += "\\"
			} else {
				line += "|"
			}
		} else if g.PrevState == STATE_COLLAPSE && g.OldMapping[2*i+1] == i && g.Mapping[2*i] < i {
			line += "/"
		} else {
			line += "|"
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
			// Find the columns for the children in NewCols and use those to format
			// the edges.
			seen = true
			idx := g.MergeLayout

			for j, next := range g.nexts() {
				nextIndex := findId(g.NewCols, next)
				if nextIndex == -1 {
					// If this happens, it's a bug in graph code
					return "ERORR: next node not found in cols; fix by complaining to the person who wrote this"
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

// GetLine returns the line of the graph corresponding to its current state and
// the id of the job currently being processed if the line is a job line, i.e.
// should be printed with information about the job. Else, if not a job line,
// id is nil.
// GetLine does not change the state of the graph object; it only retrieves a string
// based on the current state.
func (g *graph) GetLine() (line string, id *string) {
	fmtStr := fmt.Sprintf("%%-%ds", g.Width)
	switch g.State {
	case STATE_PADDING:
		return fmt.Sprintf(fmtStr, g.getPaddingLine()), nil
	case STATE_PRE_JOB:
		return fmt.Sprintf(fmtStr, g.getPreJobLine()), nil
	case STATE_JOB:
		return fmt.Sprintf(fmtStr, g.getJobLine()), g.CurrId
	case STATE_BRANCH:
		return fmt.Sprintf(fmtStr, g.getBranchLine()), nil
	case STATE_COLLAPSE:
		return fmt.Sprintf(fmtStr, g.getCollapseLine()), nil
	}
	// This never happens; nextState always moves us into a valid graph state.
	return "bug in code", nil
}

// nextState updates the state of the graph depending on various metainfo.
func (g *graph) nextState() bool {
	switch g.State {
	case STATE_PADDING:
		if !g.nextJob() {
			return false
		}
		if g.needsPreJobLine() {
			g.updateState(STATE_PRE_JOB)
		} else {
			g.updateState(STATE_JOB)
		}
	case STATE_PRE_JOB:
		g.ExpansionRows++
		if !g.needsPreJobLine() {
			g.updateState(STATE_JOB)
		}
	case STATE_JOB:
		if len(g.nexts()) > 1 {
			g.updateState(STATE_BRANCH)
		} else if g.mappingCorrect() {
			g.updateState(STATE_PADDING)
		} else {
			g.updateState(STATE_COLLAPSE)
		}
	case STATE_BRANCH:
		if g.mappingCorrect() {
			g.updateState(STATE_PADDING)
		} else {
			g.updateState(STATE_COLLAPSE)
		}
	case STATE_COLLAPSE:
		if g.mappingCorrect() {
			g.updateState(STATE_PADDING)
		}
	default:
		return false
	}

	// If we're in state STATE_COLLAPSE, we need to also update `g.Mapping`
	// in preparation
	if g.State == STATE_COLLAPSE {
		g.updateCollapse()
	}

	return true
}

// NextLine moves the graph into the next state and returns true if there was a
// state to move to, and false otherwise, i.e. when the graph is complete.
func (g *graph) NextLine() bool {
	if !g.nextState() {
		return false
	}
	return true
}

// ToDot prints out g in DOT graph format.
// Copy and paste output into http://www.webgraphviz.com/
// It's not used anywhere in the code, but it is a very useful debugging tool.
func (g *graph) ToDot() string {
	str := "digraph {\n"
	for u, vs := range g.Edges {
		for _, v := range vs {
			str += fmt.Sprintf(`	"%s" -> "%s";
`, u, v)
		}
	}
	str += "}"
	return str
}

// String prints all state info for the graph for debugging purposes.
func (g *graph) String() string {
	id := ""
	if g.CurrId != nil {
		id = *g.CurrId
	}
	prevState := StateName[g.PrevState]
	state := StateName[g.State]
	return fmt.Sprintf(`CurrId = %s        Width = %d        ExpansionRows = %d MergeLayout = %d
PrevState = %s        State = %s
PrevIdIndex = %d        IdIndex = %d
PrevEdgesAdded = %d        EdgesAdded = %d
Cols = %v
NewCols = %v
OldMapping = %v
Mapping = %v
HorizontalEdge = %d        HorizontalEdgeTarget = %d`,
		id, g.Width, g.ExpansionRows, g.MergeLayout,
		prevState, state, g.PrevIdIndex, g.IdIndex, g.PrevEdgesAdded, g.EdgesAdded,
		g.Cols, g.NewCols, g.OldMapping, g.Mapping,
		g.HorizontalEdge, g.HorizontalEdgeTarget)
}
