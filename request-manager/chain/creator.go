// Copyright 2017-2020, Square, Inc.

package chain

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/square/spincycle/v2/job"
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
	"github.com/square/spincycle/v2/request-manager/template"
)

const DEFAULT = "default"

type Creator struct {
	JobFactory        job.Factory                   // factory to create nodes' jobs.
	AllSequences      map[string]*spec.SequenceSpec // sequence name --> sequence spec
	SequenceTemplates map[string]*template.Graph    // sequence name --> template graph

	idgen id.Generator // generates UIDs for the nodes created by the Creator
	req   proto.Request
}

func NewCreator(req proto.Request, nf job.Factory, specs map[string]*spec.SequenceSpec, templates map[string]*template.Graph, idgen id.Generator) *Creator {
	o := &Creator{
		JobFactory:        nf,
		AllSequences:      specs,
		SequenceTemplates: templates,
		idgen:             idgen,
		req:               req,
	}
	return o
}

type CreatorFactory interface {
	// Make makes a Creator. A new creator should be made for every request.
	Make(proto.Request) *Creator
	// Retrieve all sequence (specs) the factory knows about.
	Sequences() map[string]*spec.SequenceSpec
}

type creatorFactory struct {
	jf        job.Factory
	seqs      map[string]*spec.SequenceSpec
	templates map[string]*template.Graph
	idf       id.GeneratorFactory
}

func NewCreatorFactory(jf job.Factory, specs map[string]*spec.SequenceSpec, templates map[string]*template.Graph, idf id.GeneratorFactory) CreatorFactory {
	return &creatorFactory{
		jf:        jf,
		seqs:      specs,
		templates: templates,
		idf:       idf,
	}
}

func (f *creatorFactory) Make(req proto.Request) *Creator {
	return NewCreator(req, f.jf, f.seqs, f.templates, f.idf.Make()) // create a Creator with a new id Generator
}

func (f *creatorFactory) Sequences() map[string]*spec.SequenceSpec {
	return f.seqs
}

func (o *Creator) RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error) {
	reqArgs := []proto.RequestArg{}

	seq, ok := o.AllSequences[o.req.Type]
	if !ok {
		return nil, fmt.Errorf("cannot find definition for request: %s", o.req.Type)
	}
	if !seq.Request {
		return nil, fmt.Errorf("%s is not a request", o.req.Type)
	}

	for i, arg := range seq.Args.Required {
		val, ok := jobArgs[*arg.Name]
		if !ok {
			return nil, fmt.Errorf("required arg '%s' not set", *arg.Name)
		}
		reqArgs = append(reqArgs, proto.RequestArg{
			Pos:   i,
			Name:  *arg.Name,
			Type:  proto.ARG_TYPE_REQUIRED,
			Value: val,
			Given: true,
		})
	}

	for i, arg := range seq.Args.Optional {
		val, ok := jobArgs[*arg.Name]
		if !ok {
			val = *arg.Default
		}
		reqArgs = append(reqArgs, proto.RequestArg{
			Pos:     i,
			Name:    *arg.Name,
			Type:    proto.ARG_TYPE_OPTIONAL,
			Default: *arg.Default,
			Value:   val,
			Given:   ok,
		})
	}

	for i, arg := range seq.Args.Static {
		reqArgs = append(reqArgs, proto.RequestArg{
			Pos:   i,
			Name:  *arg.Name,
			Type:  proto.ARG_TYPE_STATIC,
			Value: *arg.Default,
		})
	}

	return reqArgs, nil
}

func (o *Creator) BuildJobChain(jobArgs map[string]interface{}) (*proto.JobChain, error) {
	/* Build graph of actual jobs based on template. */
	chainGraph, err := o.buildSequence("request_"+o.req.Type, o.req.Type, jobArgs, 0, "0s")
	if err != nil {
		return nil, err
	}

	/* Turn it into a job chain. */
	jc := &proto.JobChain{
		AdjacencyList: chainGraph.Graph.Edges,
		RequestId:     o.req.Id,
		State:         proto.STATE_PENDING,
		Jobs:          map[string]proto.Job{},
	}
	for jobId, node := range chainGraph.getVertices() {
		bytes, err := node.Job.Serialize()
		if err != nil {
			return nil, err
		}
		job := proto.Job{
			Type:              node.Job.Id().Type,
			Id:                node.Job.Id().Id,
			Name:              node.Job.Id().Name,
			Bytes:             bytes,
			Args:              node.Args,
			Retry:             node.Retry,
			RetryWait:         node.RetryWait,
			SequenceId:        node.SequenceId,
			SequenceRetry:     node.SequenceRetry,
			SequenceRetryWait: node.SequenceRetryWait,
			State:             proto.STATE_PENDING,
		}
		jc.Jobs[jobId] = job
	}

	return jc, nil
}

func (o *Creator) buildSequence(wrapperName, sequenceName string, jobArgs map[string]interface{}, sequenceRetry uint, sequenceRetryWait string) (*Graph, error) {
	/* Add optional and static sequence arguments to map. */
	seq, ok := o.AllSequences[sequenceName]
	if !ok {
		return nil, fmt.Errorf("cannot find definition for sequence: %s", sequenceName)
	}
	for _, arg := range seq.Args.Optional {
		if _, ok := jobArgs[*arg.Name]; !ok {
			jobArgs[*arg.Name] = *arg.Default
		}
	}
	for _, arg := range seq.Args.Static {
		if _, ok := jobArgs[*arg.Name]; !ok {
			jobArgs[*arg.Name] = *arg.Default
		}
	}

	/* Build graph based on template. */
	templateGraph, ok := o.SequenceTemplates[sequenceName]
	if !ok {
		return nil, fmt.Errorf("cannot find template for sequence: %s", sequenceName)
	}
	idMap := map[string]*graph.Graph{} // template node id --> actual subgraph
	g, err := o.newEmptyGraph(wrapperName, jobArgs)
	if err != nil {
		return nil, err
	}

	for _, templateNode := range templateGraph.Iterator() {
		n := templateNode.NodeSpec

		// Find out how many times this node has to be repeated
		iterators, iterateOvers, err := o.getIterators(n, jobArgs)
		if err != nil {
			return nil, fmt.Errorf("in seq %s, node %s: invalid 'each:' %s", sequenceName, n.Name, err)
		}

		// All the graphs that make up this component
		components := []*graph.Graph{}

		// If no repetition is needed, this loop will only execute once
		for i, _ := range iterateOvers[0] {
			// Copy the required args into a separate args map here.
			// Do the necessary remapping here.
			jobArgsCopy, err := remapNodeArgs(n, jobArgs)
			if err != nil {
				return nil, err
			}

			// Add the iterator to the node args unless there is no iterator for this node
			for j, iterator := range iterators {
				if iterator != "" {
					// This won't panic because we have earlier asserted that
					// len(iterators) == len(iterateOvers)
					jobArgsCopy[iterator] = iterateOvers[j][i]
				}
			}

			// Add the "if" to the node args if it's present
			if n.If != nil {
				if ifArg, ok := jobArgs[*n.If]; ok {
					jobArgsCopy[*n.If] = ifArg
				}
			}

			// Build next graph component and assert that it's valid
			var subgraph *graph.Graph
			if n.IsConditional() {
				// Node is a conditional
				conditional, err := chooseConditional(n, jobArgsCopy)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", sequenceName, n.Name, err)
				}
				chainSubgraph, err := o.buildSequence("conditional_"+n.Name, conditional, jobArgsCopy, n.Retry, n.RetryWait)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", sequenceName, n.Name, err)
				}
				subgraph = &chainSubgraph.Graph
			} else if n.IsSequence() {
				// Node is a sequence, recursively construct its components
				chainSubgraph, err := o.buildSequence("sequence_"+n.Name, *n.NodeType, jobArgsCopy, n.Retry, n.RetryWait)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", sequenceName, n.Name, err)
				}
				subgraph = &chainSubgraph.Graph
			} else {
				// Node is a job, create a graph that contains only the node
				subgraph, err = o.buildSingleVertexGraph(n, jobArgsCopy)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: cannot build job: %s", sequenceName, n.Name, err)
				}
			}
			if !subgraph.IsValidGraph() {
				return nil, fmt.Errorf("malformed graph created")
			}

			// Add the new node to the map of completed components
			components = append(components, subgraph)

			// If the node (or sequence) was determined to set any args
			// copy them from jobArgsCopy into the main jobArgs
			err = setNodeArgs(n, jobArgs, jobArgsCopy)
			if err != nil {
				return nil, err
			}
		}

		// If this component was repeated multiple times,
		// wrap it between a single dummy start and end vertices.
		// This makes the resulting graph easier to reason about.
		// If there are no components for the node, do nothing.
		var wrappedSubgraph *graph.Graph
		if len(components) > 1 {
			// Create the start and end nodes
			wrappedSubgraph, err = o.newEmptyGraph("repeat_"+n.Name, jobArgs)
			if err != nil {
				return nil, err
			}

			// Insert all components between the start and end vertices.
			// Place at most `parallel` components per parallel supercomponent.
			// Serialize parallel supercomponents if number of components
			// exceeds `parallel`.
			// Each parallel supercomponent is wrapped between dummy nodes.
			var parallel uint
			if n.Parallel == nil {
				parallel = uint(len(components))
			} else {
				parallel = *n.Parallel
			}

			currG, err := o.newEmptyGraph("repeat_"+n.Name, jobArgs)
			if err != nil {
				return nil, err
			}

			prev := wrappedSubgraph.First
			var count uint = 0
			for _, c := range components {
				currG.InsertComponentBetween(c, currG.First, currG.Last)
				count++
				if count == parallel {
					wrappedSubgraph.InsertComponentBetween(currG, prev, wrappedSubgraph.Last)
					prev = currG.Last
					currG, err = o.newEmptyGraph("repeat_"+n.Name, jobArgs)
					if err != nil {
						return nil, err
					}
					count = 0
				}
			}
			if count != 0 {
				wrappedSubgraph.InsertComponentBetween(currG, prev, wrappedSubgraph.Last)
			}
		} else if len(components) == 1 {
			wrappedSubgraph = components[0]
		} else if len(components) == 0 {
			// Even if there are no iterateOvers, we still need to add
			// the node to the graph in order to fulfill dependencies
			// for later nodes.
			wrappedSubgraph, err = o.newEmptyGraph("noop_"+n.Name, jobArgs)
			if err != nil {
				return nil, err
			}
		}
		if !wrappedSubgraph.IsValidGraph() {
			return nil, fmt.Errorf("malformed graph created")
		}

		idMap[templateNode.GetId()] = wrappedSubgraph
		if len(templateNode.Prev) == 0 {
			g.InsertComponentBetween(wrappedSubgraph, g.First, g.Last)
		} else {
			for id, _ := range templateNode.Prev {
				prev, ok := idMap[id]
				if !ok {
					return nil, fmt.Errorf("could not find previous component of %s (components added out of order)", templateNode.GetName())
				}
				g.InsertComponentBetween(wrappedSubgraph, prev.Last, g.Last)
			}
		}

		if !g.IsValidGraph() {
			return nil, fmt.Errorf("malformed graph created")
		}
	}

	// Assert g is a well formed graph
	if !g.IsValidGraph() {
		return nil, fmt.Errorf("malformed graph created")
	}

	// Mark all vertices in sequence except start vertex start sequence id
	// A graph is built by constructing its inner most components, which are
	// sequences, and building its way out. When constructing the inner most
	// sequence, we want to set SequenceId for all but the first vertex in the
	// sequence. The SequenceId for the first vertex in the sequence will be set
	// on a subsequent pass. Lastly, the first vertex in the completed graph will
	// have no SequenceId set, as that vertex is part of a larger sequence.
	chainGraph := &Graph{*g}
	sequenceId := g.First.GetId()
	for _, vertex := range chainGraph.getVertices() {
		// TODO(alyssa): Add `ParentSequenceId` to start vertex of each sequence.
		// It's important to do this check before setting `SequenceId`
		// if vertex.Id == vertex.SequenceId && vertex.ParentSequenceId == "" {
		//   vertex.ParentSequenceId = sequenceId
		// }

		// Set SequenceId if it has not been set yet. This check also ensures that
		// it is not overwritten on subsequent visits to this vertex.
		if vertex.SequenceId == "" {
			vertex.SequenceId = sequenceId
		}
	}
	// store configured retry from sequence spec on the first node in the sequence
	chainGraph.setSequenceRetryInfo(sequenceRetry, sequenceRetryWait)

	return chainGraph, nil
}

func chooseConditional(n *spec.NodeSpec, jobArgs map[string]interface{}) (string, error) {
	// Node is a conditional, check the value of the "if" jobArg
	val, ok := jobArgs[*n.If]
	valstring, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("'if' arg '%s' is not a string", *n.If)
	}
	// Based on value of "if" jobArg, get which sequence to execute
	seqName, ok := n.Eq[valstring]
	if !ok {
		// check if default sequence specified
		seqName, ok = n.Eq[DEFAULT]
		if !ok {
			values := make([]string, 0, len(n.Eq))
			for k := range n.Eq {
				values = append(values, k)
			}
			sort.Strings(values)
			return "", fmt.Errorf("'if: %s' value '%s' does not match any options: %s", *n.If, valstring, strings.Join(values, ", "))
		}
	}

	return seqName, nil
}

// Gets the iterables and iterators ( the "each" clause in the yaml )
// returns the iterator name, and the slice to iterate over when repeating nodes.
// If there is no repetition required, then it will return an empty string, "",
// and the singleton [""], to indicate that only one iteration is needed.
//
// Precondition: the iteratable must already be present in args
func (o *Creator) getIterators(n *spec.NodeSpec, args map[string]interface{}) ([]string, [][]interface{}, error) {
	empty := []string{""}
	empties := [][]interface{}{[]interface{}{""}}
	if len(n.Each) == 0 {
		return empty, empties, nil
	}

	iterators := []string{}
	iterateOvers := [][]interface{}{}

	for _, each := range n.Each {
		p := strings.Split(each, ":")
		if len(p) != 2 {
			err := fmt.Errorf("invalid each value: %s: split on ':' yielded %d values, expected 2", n.Each, len(p))
			return empty, empties, err
		}
		iterateSet := p[0]
		iterator := p[1]
		iterateOver := []interface{}{}

		// Grab the iterable set out of args
		iterables, ok := args[iterateSet]
		if !ok {
			return empty, empties, fmt.Errorf("each:%s: arg %s not set", n.Each, iterateSet)
		}

		// Assert that this is a slice
		if reflect.TypeOf(iterables).Kind() != reflect.Slice {
			return empty, empties, fmt.Errorf("each:%s: arg %s is a %s, expected a slice", n.Each, iterateSet, reflect.TypeOf(iterables).Kind())
		}

		a := reflect.ValueOf(iterables)
		for i := 0; i < a.Len(); i++ {
			iterateOver = append(iterateOver, a.Index(i).Interface())
		}

		iterators = append(iterators, iterator)
		iterateOvers = append(iterateOvers, iterateOver)
	}

	// Validate that the iterators all have variable names
	// @todo: I presume this means each: x\n y  len(x)==len(y) so we have an
	//        "even" number of iterable values?
	// @todo: fix, it doesn't catch uneven list len
	if len(iterateOvers) != len(iterators) || len(iterators) < 1 {
		return nil, nil, fmt.Errorf("args have different len")
	}

	return iterators, iterateOvers, nil
}

// Given a node definition and an args, copy args into a new map,
// but also rename the arguments as defined in the "args" clause.
// A shallow copy is sufficient because args values should never
// change.
func remapNodeArgs(n *spec.NodeSpec, args map[string]interface{}) (map[string]interface{}, error) {
	jobArgs2 := map[string]interface{}{}
	for _, arg := range n.Args {
		var ok bool
		jobArgs2[*arg.Expected], ok = args[*arg.Given]
		if !ok {
			return nil, fmt.Errorf("cannot create job %s: missing %s from job args", *n.NodeType, *arg.Given)
		}
	}
	return jobArgs2, nil
}

// Given a node definition and two args sets. Copy the arguments that
// are defined in the "sets/arg" clause into the main args map under
// the name defined by the "sets/as" field.
func setNodeArgs(n *spec.NodeSpec, argsTo, argsFrom map[string]interface{}) error {
	if len(n.Sets) == 0 {
		return nil
	}
	for _, key := range n.Sets {
		var ok bool
		var val interface{}
		val, ok = argsFrom[*key.Arg]
		if !ok {
			return fmt.Errorf("expected %s to set %s in jobargs", *n.NodeType, *key.Arg)
		}
		argsTo[*key.As] = val
	}

	return nil
}

// Builds a graph containing a single node
func (o *Creator) buildSingleVertexGraph(nodeDef *spec.NodeSpec, jobArgs map[string]interface{}) (*graph.Graph, error) {
	n, err := o.newNode(nodeDef, jobArgs)
	if err != nil {
		return nil, err
	}
	g := &graph.Graph{
		Name:     nodeDef.Name,
		First:    n,
		Last:     n,
		Vertices: map[string]graph.Node{n.GetId(): n},
		Edges:    map[string][]string{},
	}
	return g, nil
}

// NewEmptyGraph creates an "empty" graph. It contains two nodes: the "start" and "end" nodes. Both of these nodes
// are no-op jobs
func (o *Creator) newEmptyGraph(name string, jobArgs map[string]interface{}) (*graph.Graph, error) {
	var err error

	jobArgsCopy, err := remapNodeArgs(noopSpec, jobArgs)
	if err != nil {
		return nil, err
	}

	first, err := o.newNoopNode(name+"_start", jobArgsCopy)
	if err != nil {
		return nil, err
	}

	last, err := o.newNoopNode(name+"_end", jobArgsCopy)
	if err != nil {
		return nil, err
	}

	err = setNodeArgs(noopSpec, jobArgs, jobArgsCopy)
	if err != nil {
		return nil, err
	}

	first.Next[last.GetId()] = last
	last.Prev[first.GetId()] = first

	return &graph.Graph{
		Name:     name,
		First:    first,
		Last:     last,
		Vertices: map[string]graph.Node{first.GetId(): first, last.GetId(): last},
		Edges:    map[string][]string{first.GetId(): []string{last.GetId()}},
	}, nil
}

// NewStartNode creates an empty "start" node. There is no job defined for this node, but it can serve
// as a marker for a sequence/request.
func (o *Creator) newNoopNode(name string, jobArgs map[string]interface{}) (*Node, error) {
	id, err := o.idgen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for no-op node %s: %s", name, err)
	}
	jid := job.NewIdWithRequestId("noop", name, id, o.req.Id)
	rj, err := o.JobFactory.Make(jid)
	if err != nil {
		switch err {
		case job.ErrUnknownJobType:
			// No custom noop job, use built-in default
			rj = &noopJob{
				id: jid,
			}
		default:
			return nil, fmt.Errorf("Error making no-op node %s: %s", name, err)
		}
	}
	err = rj.Create(jobArgs)
	if err != nil {
		return nil, fmt.Errorf("Error creating no-op node %s: %s", name, err)
	}

	return &Node{
		Job:  rj,
		Next: map[string]graph.Node{},
		Prev: map[string]graph.Node{},
		Name: name,
	}, nil
}

// newNode creates a node for the given job j
func (o *Creator) newNode(j *spec.NodeSpec, jobArgs map[string]interface{}) (*Node, error) {
	// Make a copy of the jobArgs before this node gets created and potentially
	// adds additional keys to the jobArgs. A shallow copy is sufficient because
	// args values should never change.
	originalArgs := map[string]interface{}{}
	for k, v := range jobArgs {
		originalArgs[k] = v
	}

	// Make the name of this node unique within the request by assigning it an id.
	id, err := o.idgen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	// Create the job
	rj, err := o.JobFactory.Make(job.NewIdWithRequestId(*j.NodeType, j.Name, id, o.req.Id))
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	err = rj.Create(jobArgs)
	if err != nil {
		return nil, fmt.Errorf("Error creating '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	return &Node{
		Job:       rj,
		Next:      map[string]graph.Node{},
		Prev:      map[string]graph.Node{},
		Name:      j.Name,
		Args:      originalArgs, // Args is the jobArgs map that this node was created with
		Retry:     j.Retry,
		RetryWait: j.RetryWait,
	}, nil
}

// containsAll is a convenience function for checking membership in a map.
// Returns true if m contains every elements in ss
func containsAll(m map[*spec.NodeSpec]bool, ss []string, nodes map[string]*spec.NodeSpec) bool {
	for _, s := range ss {
		name := nodes[s]
		if _, ok := m[name]; !ok {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------------- //

// Mock creator factory for testing.
type MockCreatorFactory struct {
	MakeFunc func(proto.Request) *Creator
}

func (cf *MockCreatorFactory) Make(req proto.Request) *Creator {
	if cf.MakeFunc != nil {
		return cf.MakeFunc(req)
	}
	return nil
}

func (cf *MockCreatorFactory) Sequences() map[string]*spec.SequenceSpec {
	return map[string]*spec.SequenceSpec{}
}
