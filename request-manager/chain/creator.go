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

// Returns Creator structs, tailored to create job chains for the input request.
type CreatorFactory interface {
	// Make makes a Creator. A new creator should be made for every request.
	Make(proto.Request) Creator
}

// Implements CreatorFactory interface.
type creatorFactory struct {
	jf        job.Factory
	seqs      map[string]*spec.Sequence
	templates map[string]*template.Graph
	idf       id.GeneratorFactory
}

func NewCreatorFactory(jf job.Factory, specs map[string]*spec.Sequence, templates map[string]*template.Graph, idf id.GeneratorFactory) CreatorFactory {
	return &creatorFactory{
		jf:        jf,
		seqs:      specs,
		templates: templates,
		idf:       idf,
	}
}

func (f *creatorFactory) Make(req proto.Request) Creator {
	return &creator{
		Request:           req,
		JobFactory:        f.jf,
		AllSequences:      f.seqs,
		SequenceTemplates: f.templates,
		IdGen:             f.idf.Make(),
	}
}

// Creates a job chain for a single request.
type Creator interface {
	// Convert job args map to a list of proto.RequestArgs.
	RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error)

	// Build the actual job chain. Returns an error if any error occurs.
	BuildJobChain(jobArgs map[string]interface{}) (*proto.JobChain, error)
}

// Implements Creator interface.
type creator struct {
	Request           proto.Request              // the request spec this creator can create job chain for
	JobFactory        job.Factory                // factory to create nodes' jobs
	AllSequences      map[string]*spec.Sequence  // sequence name --> sequence spec
	SequenceTemplates map[string]*template.Graph // sequence name --> template graph
	IdGen             id.Generator               // generates UIDs for jobs
}

func (o *creator) RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error) {
	reqArgs := []proto.RequestArg{}

	seq, ok := o.AllSequences[o.Request.Type]
	if !ok {
		return nil, fmt.Errorf("cannot find definition for request: %s", o.Request.Type)
	}
	if !seq.Request {
		return nil, fmt.Errorf("%s is not a request", o.Request.Type)
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

// Parameters to buildSequence helper function.
type buildSequenceConfig struct {
	wrapperName       string                 // Name of graph and its source/sink nodes
	sequenceName      string                 // Name of sequence to build
	jobArgs           map[string]interface{} // Set of job args sequence is given
	sequenceRetry     uint                   // Retry info for sequence
	sequenceRetryWait string
}

func (o *creator) BuildJobChain(jobArgs map[string]interface{}) (*proto.JobChain, error) {
	/* Build graph of actual jobs based on template. */
	cfg := buildSequenceConfig{
		wrapperName:       "request_" + o.Request.Type,
		sequenceName:      o.Request.Type,
		jobArgs:           jobArgs,
		sequenceRetry:     0,
		sequenceRetryWait: "0s",
	}
	chainGraph, err := o.buildSequence(cfg)
	if err != nil {
		return nil, err
	}

	/* Turn it into a job chain. */
	jc := &proto.JobChain{
		AdjacencyList: chainGraph.Graph.Edges,
		RequestId:     o.Request.Id,
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

// (Recursively) build a sequence.
func (o *creator) buildSequence(cfg buildSequenceConfig) (*Graph, error) {
	sequenceName := cfg.sequenceName
	jobArgs := cfg.jobArgs

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

	g, err := o.newEmptyGraph(cfg.wrapperName, jobArgs)
	if err != nil {
		return nil, err
	}

	// Traverse template graph in topological order. Each template node corresponds to either a single
	// job node, or an entire subgraph of nodes. Build the subgraph and insert it into the larger sequence
	// graph.
	idMap := map[string]*graph.Graph{} // template node id --> corresponding subgraph
	for _, templateNode := range templateGraph.Iterator() {
		n := templateNode.Node

		// Find out how many times this node has to be repeated
		elements, lists, err := o.getIterators(n, jobArgs)
		if err != nil {
			return nil, fmt.Errorf("in seq %s, node %s: invalid 'each:' %s", sequenceName, n.Name, err)
		}

		// All the graphs that make up this template node's subgraph
		components := []*graph.Graph{}

		// If no repetition is needed, this loop will only execute once
		for i, _ := range lists[0] {
			// Copy the required args into a separate args map here.
			// Do the necessary remapping here.
			jobArgsCopy, err := remapNodeArgs(n, jobArgs)
			if err != nil {
				return nil, err
			}

			// Add the element to the node args unless there is none for this node
			for j, elt := range elements {
				if elt != "" {
					// This won't panic because we have earlier asserted that
					// len(elements) == len(lists)
					jobArgsCopy[elt] = lists[j][i]
				}
			}

			// Add the "if" to the node args if it's present
			if n.If != nil {
				if ifArg, ok := jobArgs[*n.If]; ok {
					jobArgsCopy[*n.If] = ifArg
				}
			}

			// Build graph component and assert that it's valid
			var subgraph *graph.Graph
			if n.IsConditional() {
				// Node is a conditional
				conditional, err := chooseConditional(n, jobArgsCopy)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", sequenceName, n.Name, err)
				}
				cfg := buildSequenceConfig{
					wrapperName:       "conditional_" + n.Name,
					sequenceName:      conditional,
					jobArgs:           jobArgsCopy,
					sequenceRetry:     n.Retry,
					sequenceRetryWait: n.RetryWait,
				}
				chainSubgraph, err := o.buildSequence(cfg)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", sequenceName, n.Name, err)
				}
				subgraph = &chainSubgraph.Graph
			} else if n.IsSequence() {
				// Node is a sequence, recursively construct its components
				cfg := buildSequenceConfig{
					wrapperName:       "sequence_" + n.Name,
					sequenceName:      *n.NodeType,
					jobArgs:           jobArgsCopy,
					sequenceRetry:     n.Retry,
					sequenceRetryWait: n.RetryWait,
				}
				chainSubgraph, err := o.buildSequence(cfg)
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
				return nil, fmt.Errorf("malformed subgraph created")
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
			// Even if there are no lists, we still need to add
			// the node to the graph in order to fulfill dependencies
			// for later nodes.
			wrappedSubgraph, err = o.newEmptyGraph("noop_"+n.Name, jobArgs)
			if err != nil {
				return nil, err
			}
		}
		if !wrappedSubgraph.IsValidGraph() {
			return nil, fmt.Errorf("malformed wrappedSubgraph created")
		}

		// `wrappedSubgraph` is the graph of jobs corresponding directly to this
		// template node. Insert it between its dependencies and the last node.
		idMap[templateNode.Id()] = wrappedSubgraph
		prevs := templateGraph.GetPrev(templateNode)
		if len(prevs) == 0 {
			// case: template graph's source node
			g.InsertComponentBetween(wrappedSubgraph, g.First, g.Last)
		} else {
			// case: every other node; insert between sink node of previous node's
			// subgraph, and the last node of the sequence graph
			for id, _ := range prevs {
				prev, ok := idMap[id]
				if !ok {
					return nil, fmt.Errorf("could not find previous component of %s (components added out of order)", templateNode.Name())
				}
				g.InsertComponentBetween(wrappedSubgraph, prev.Last, g.Last)
			}
		}
		if !g.IsValidGraph() {
			return nil, fmt.Errorf("malformed g created")
		}
	}

	// Mark all vertices in sequence except start vertex start sequence id
	// A graph is built by constructing its inner most components, which are
	// sequences, and building its way out. When constructing the inner most
	// sequence, we want to set SequenceId for all but the first vertex in the
	// sequence. The SequenceId for the first vertex in the sequence will be set
	// on a subsequent pass. Lastly, the first vertex in the completed graph will
	// have no SequenceId set, as that vertex is part of a larger sequence.
	chainGraph := &Graph{*g}
	sequenceId := g.First.Id()
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
	chainGraph.setSequenceRetryInfo(cfg.sequenceRetry, cfg.sequenceRetryWait)
	return chainGraph, nil
}

// Based on values of job args, determine which path of a conditional to take.
// Assumes `n` is a conditional node.
func chooseConditional(n *spec.Node, jobArgs map[string]interface{}) (string, error) {
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

// Split `each: list:element` values into the list (as a slice) and the element.
// Returns the element name, and the slice to iterate over when repeating nodes.
// If there is no repetition required, then it will return an empty string, "",
// and the singleton [""], to indicate that only one iteration is needed.
//
// Precondition: the list must already be present in args
func (o *creator) getIterators(n *spec.Node, args map[string]interface{}) ([]string, [][]interface{}, error) {
	empty := []string{""}
	empties := [][]interface{}{[]interface{}{""}}
	if len(n.Each) == 0 {
		return empty, empties, nil
	}

	elements := []string{}
	lists := [][]interface{}{}

	for _, each := range n.Each {
		split := strings.Split(each, ":")
		if len(split) != 2 {
			err := fmt.Errorf("invalid each value: %s: split on ':' yielded %d values, expected 2", n.Each, len(split))
			return empty, empties, err
		}
		listName := split[0]
		element := split[1]
		list := []interface{}{}

		// Grab the iterable set out of args
		iterables, ok := args[listName]
		if !ok {
			return empty, empties, fmt.Errorf("each:%s: arg %s not set", n.Each, listName)
		}

		// Assert that this is a slice
		if reflect.TypeOf(iterables).Kind() != reflect.Slice {
			return empty, empties, fmt.Errorf("each:%s: arg %s is a %s, expected a slice", n.Each, listName, reflect.TypeOf(iterables).Kind())
		}

		a := reflect.ValueOf(iterables)
		for i := 0; i < a.Len(); i++ {
			list = append(list, a.Index(i).Interface())
		}

		elements = append(elements, element)
		lists = append(lists, list)
	}

	// Validate that the elements all have variable names
	// @todo: I presume this means each: x\n y  len(x)==len(y) so we have an
	//        "even" number of iterable values?
	// @todo: fix, it doesn't catch uneven list len
	if len(lists) != len(elements) || len(elements) < 1 {
		return nil, nil, fmt.Errorf("args have different len")
	}

	return elements, lists, nil
}

// Given a node definition and an args, copy args into a new map,
// but also rename the arguments as defined in the "args" clause.
// A shallow copy is sufficient because args values should never
// change.
func remapNodeArgs(n *spec.Node, args map[string]interface{}) (map[string]interface{}, error) {
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
func setNodeArgs(n *spec.Node, argsTo, argsFrom map[string]interface{}) error {
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
func (o *creator) buildSingleVertexGraph(nodeDef *spec.Node, jobArgs map[string]interface{}) (*graph.Graph, error) {
	n, err := o.newNode(nodeDef, jobArgs)
	if err != nil {
		return nil, err
	}
	g := &graph.Graph{
		Name:     nodeDef.Name,
		First:    n,
		Last:     n,
		Vertices: map[string]graph.Node{n.Id(): n},
		Edges:    map[string][]string{},
		RevEdges: map[string][]string{},
	}
	return g, nil
}

// NewEmptyGraph creates an "empty" graph. It contains two nodes: the "start" and "end" nodes. Both of these nodes
// are no-op jobs
func (o *creator) newEmptyGraph(name string, jobArgs map[string]interface{}) (*graph.Graph, error) {
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

	return &graph.Graph{
		Name:     name,
		First:    first,
		Last:     last,
		Vertices: map[string]graph.Node{first.Id(): first, last.Id(): last},
		Edges:    map[string][]string{first.Id(): []string{last.Id()}},
		RevEdges: map[string][]string{last.Id(): []string{first.Id()}},
	}, nil
}

// NewStartNode creates an empty "start" node. There is no job defined for this node, but it can serve
// as a marker for a sequence/request.
func (o *creator) newNoopNode(name string, jobArgs map[string]interface{}) (*Node, error) {
	id, err := o.IdGen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for no-op node %s: %s", name, err)
	}
	jid := job.NewIdWithRequestId("noop", name, id, o.Request.Id)
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
		Job:      rj,
		NodeName: name,
	}, nil
}

// newNode creates a node for the given job j
func (o *creator) newNode(j *spec.Node, jobArgs map[string]interface{}) (*Node, error) {
	// Make a copy of the jobArgs before this node gets created and potentially
	// adds additional keys to the jobArgs. A shallow copy is sufficient because
	// args values should never change.
	originalArgs := map[string]interface{}{}
	for k, v := range jobArgs {
		originalArgs[k] = v
	}

	// Make the name of this node unique within the request by assigning it an id.
	id, err := o.IdGen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	// Create the job
	rj, err := o.JobFactory.Make(job.NewIdWithRequestId(*j.NodeType, j.Name, id, o.Request.Id))
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	err = rj.Create(jobArgs)
	if err != nil {
		return nil, fmt.Errorf("Error creating '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	return &Node{
		Job:       rj,
		NodeName:  j.Name,
		Args:      originalArgs, // Args is the jobArgs map that this node was created with
		Retry:     j.Retry,
		RetryWait: j.RetryWait,
	}, nil
}

/* ========================================================================== */
// Mocks for testing
type MockCreatorFactory struct {
	MakeFunc func(proto.Request) Creator
}

func (f *MockCreatorFactory) Make(req proto.Request) Creator {
	if f.MakeFunc != nil {
		return f.MakeFunc(req)
	}
	return nil
}

type MockCreator struct {
	RequestArgsFunc   func(jobArgs map[string]interface{}) ([]proto.RequestArg, error)
	BuildJobChainFunc func(jobArgs map[string]interface{}) (*proto.JobChain, error)
}

func (o *MockCreator) RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error) {
	if o.RequestArgsFunc != nil {
		return o.RequestArgsFunc(jobArgs)
	}
	return []proto.RequestArg{}, nil
}
func (o *MockCreator) BuildJobChain(jobArgs map[string]interface{}) (*proto.JobChain, error) {
	if o.BuildJobChainFunc != nil {
		return o.BuildJobChainFunc(jobArgs)
	}
	return nil, nil
}
