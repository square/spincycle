// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/square/spincycle/v2/job"
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// ResolverFactory returns Resolvers tailored to create request graphs
// for a specific input request.
type ResolverFactory interface {
	// Make makes a Resolver. A new resolver should be made for every request.
	Make(proto.Request) Resolver
}

// Implements ResolverFactory interface.
type resolverFactory struct {
	jf        job.Factory
	seqSpecs  map[string]*spec.Sequence
	seqGraphs map[string]*Graph
	idf       id.GeneratorFactory
}

func NewResolverFactory(jf job.Factory, seqSpecs map[string]*spec.Sequence, seqGraphs map[string]*Graph, idf id.GeneratorFactory) ResolverFactory {
	return &resolverFactory{
		jf:        jf,
		seqSpecs:  seqSpecs,
		seqGraphs: seqGraphs,
		idf:       idf,
	}
}

func (f *resolverFactory) Make(req proto.Request) Resolver {
	return &resolver{
		request:    req,
		jobFactory: f.jf,
		seqSpecs:   f.seqSpecs,
		seqGraphs:  f.seqGraphs,
		idGen:      f.idf.Make(),
	}
}

// Resolver builds a request graph for a specific request.
type Resolver interface {
	// Convert job args map to a list of proto.RequestArgs.
	RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error)

	// Build the request graph. Returns an error if any error occurs.
	BuildRequestGraph(jobArgs map[string]interface{}) (*Graph, error)
}

// resolver implements the Resolver interface.
type resolver struct {
	request    proto.Request             // the request spec this resolver can create job chain for
	jobFactory job.Factory               // factory to create nodes' jobs
	seqSpecs   map[string]*spec.Sequence // sequence name --> sequence spec
	seqGraphs  map[string]*Graph         // sequence name --> sequence graph
	idGen      id.Generator              // generates UIDs for jobs
}

// RequestArgs takes user input args and returns them as a job args map, the form
// expected by the Resolver later when building the request graph. Optional and
// static args are filled in with their default values.
func (r *resolver) RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error) {
	reqArgs := []proto.RequestArg{}

	seq, ok := r.seqSpecs[r.request.Type]
	if !ok {
		return nil, fmt.Errorf("cannot find specs for request: %s", r.request.Type)
	}
	if !seq.Request {
		return nil, fmt.Errorf("%s is not a request", r.request.Type)
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
			val = arg.Default
		}
		reqArgs = append(reqArgs, proto.RequestArg{
			Pos:     i,
			Name:    *arg.Name,
			Type:    proto.ARG_TYPE_OPTIONAL,
			Default: arg.Default,
			Value:   val,
			Given:   ok,
		})
	}

	for i, arg := range seq.Args.Static {
		reqArgs = append(reqArgs, proto.RequestArg{
			Pos:   i,
			Name:  *arg.Name,
			Type:  proto.ARG_TYPE_STATIC,
			Value: arg.Default,
		})
	}

	return reqArgs, nil
}

// Parameters to buildSequence helper function.
type buildSequenceConfig struct {
	graphName    string                 // Name of graph and its source/sink nodes
	seqName      string                 // Name of sequence to build
	jobArgs      map[string]interface{} // Set of job args sequence is given
	seqRetry     uint                   // Retry info for sequence
	seqRetryWait string
}

// BuildRequestGraph returns a request graph with the given starting job args.
func (r *resolver) BuildRequestGraph(jobArgs map[string]interface{}) (*Graph, error) {
	cfg := buildSequenceConfig{
		graphName:    "request_" + r.request.Type,
		seqName:      r.request.Type,
		jobArgs:      jobArgs,
		seqRetry:     0,
		seqRetryWait: "0s",
	}
	reqGraph, err := r.buildSequence(cfg)
	if err != nil {
		return nil, err
	}

	return reqGraph, nil
}

// buildSequence recursively builds a sequence. If a sequence graph node represents
// a job, buildSequence creates the corresponding job. If a sequence graph node needs
// to be expanded, i.e. it represents anything but a job, it is recursively expanded
// with another call to buildSequence. buildSequence also chooses the correct path
// for conditional nodes.
func (r *resolver) buildSequence(cfg buildSequenceConfig) (*Graph, error) {
	seqName := cfg.seqName
	jobArgs := cfg.jobArgs

	// Add optional and static sequence arguments to map.
	// If this is the request sequence, then jobArgs was the output of RequestArgs,
	// and these arguments are already present. However, subsequences will need
	// these args.
	seq, ok := r.seqSpecs[seqName]
	if !ok {
		// Static checks should prevent this from happening.
		// If this error is thrown, there's a bug in the code.
		return nil, fmt.Errorf("cannot find specs for sequence: %s", seqName)
	}
	for _, arg := range seq.Args.Optional {
		if _, ok := jobArgs[*arg.Name]; !ok {
			jobArgs[*arg.Name] = arg.Default
		}
	}
	for _, arg := range seq.Args.Static {
		if _, ok := jobArgs[*arg.Name]; !ok {
			jobArgs[*arg.Name] = arg.Default
		}
	}

	// Build request graph based on sequence graph. We use the sequence graph
	// as a template, traversing it in topological order and processing each
	// of its nodes depending on what category it is (job, sequence, conditional).
	// This sequence graph won't be mutated at all.
	seqGraph, ok := r.seqGraphs[seqName]
	if !ok {
		// Graph checks should prevent this from happening.
		// If this error is thrown, there's a bug in the code.
		return nil, fmt.Errorf("cannot find sequence graph for sequence: %s", seqName)
	}

	// This is the graph we'll actually be filling out. It starts off empty,
	// and we'll add nodes as we traverse the sequence graph.
	reqGraph, err := r.newReqGraph(cfg.graphName, jobArgs)
	if err != nil {
		return nil, err
	}

	// Traverse sequence graph in topological order. Each sequence graph node
	// corresponds to either a single job node, or an entire subgraph of nodes.
	// Build the subgraph and insert it into the larger sequence graph.
	idMap := map[string]*Graph{} // sequence graph node id --> corresponding subgraph
	for _, seqGraphNode := range seqGraph.Order {
		nodeSpec := seqGraphNode.Spec

		// 1. Do sequence expansion.
		// Find out how many times this node has to be repeated.
		// If this node has a `each:` field, then lists[i] and
		// elements[i] describe the ith `list:element`. Specifically,
		// lists[i] is the actual list value, not just the name of the
		// job arg.
		elements, lists, err := r.getIterators(nodeSpec, jobArgs)
		if err != nil {
			return nil, fmt.Errorf("in seq %s, node %s: invalid 'each:' %s", seqName, nodeSpec.Name, err)
		}

		// All the graphs that make up this sequence graph node's subgraph,
		// one for each time the node is repeated
		// For example, with 3 sequences like S1->S2->S3, if we're currently
		// in S1 (i.e. reqGraph=S1) and S2 is expanded ("each: foo:bar"),
		// we'll build N-many parallel S2 and save them in seqExpansion.
		// Then, after this `for` loop, we'll wrap all seqExpansion graphs
		// in a new graph so they have a single source and sink node.
		expandedSeqs := []*Graph{}

		// If no repetition is needed, this loop will only execute once
		// with a dummy `each:` entry `[""]:nil`.
		for i, _ := range lists[0] {
			// -----------------------------------------------------
			// Copy and prepare job args for this expansion of the
			// sequence node

			// Copy the required args into a separate args map here
			// and do the necessary remapping
			jobArgsCopy, err := remapNodeArgs(nodeSpec, jobArgs)
			if err != nil {
				return nil, err
			}

			// Given "each: foos:foo", we add jobArgs[foo] = jobArgs[foos][i],
			// unless this is the dummy `each:` that ensures that we
			// run this loop exactly once, in which case elt == nil.
			for j, elt := range elements {
				if elt != nil {
					// This won't panic because we have earlier asserted that
					// len(elements) == len(lists)
					jobArgsCopy[*elt] = lists[j][i]
				}
			}

			// If this is a conditional node, add the "if" job arg.
			// Static checks asserted if != nil for conditional nodes.
			if nodeSpec.IsConditional() {
				if ifArg, ok := jobArgs[*nodeSpec.If]; ok {
					jobArgsCopy[*nodeSpec.If] = ifArg
				}
			}

			// -----------------------------------------------------
			// Resolve sequence node into a request subgraph and add
			// to list of expansions
			var reqSubgraph *Graph
			if nodeSpec.IsConditional() {
				// Node is a conditional: choose which path to
				// take, and recursively build the subgraph
				conditional, err := chooseConditional(nodeSpec, jobArgsCopy)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", seqName, nodeSpec.Name, err)
				}
				cfg := buildSequenceConfig{
					graphName:    "conditional_" + nodeSpec.Name,
					seqName:      conditional,
					jobArgs:      jobArgsCopy,
					seqRetry:     nodeSpec.Retry,
					seqRetryWait: nodeSpec.RetryWait,
				}
				reqSubgraph, err = r.buildSequence(cfg)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", seqName, nodeSpec.Name, err)
				}
			} else if nodeSpec.IsSequence() {
				// Node is a sequence: recursively build the subgraph
				cfg := buildSequenceConfig{
					graphName:    "sequence_" + nodeSpec.Name,
					seqName:      *nodeSpec.NodeType,
					jobArgs:      jobArgsCopy,
					seqRetry:     nodeSpec.Retry,
					seqRetryWait: nodeSpec.RetryWait,
				}
				reqSubgraph, err = r.buildSequence(cfg)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: %s", seqName, nodeSpec.Name, err)
				}
			} else {
				// Node is a job: create the proto.Job and put
				// it in a graph
				reqSubgraph, err = r.buildSingleVertexGraph(nodeSpec, jobArgsCopy)
				if err != nil {
					return nil, fmt.Errorf("in seq %s, node %s: cannot build job: %s", seqName, nodeSpec.Name, err)
				}
			}

			expandedSeqs = append(expandedSeqs, reqSubgraph)

			// If the node or sequence was determined to set any args
			// copy them from jobArgsCopy into the main jobArgs
			if err := setNodeArgs(nodeSpec, jobArgs, jobArgsCopy); err != nil {
				return nil, err
			}
		} // End loop over lists

		// 2. If sequence was expanded, wrap the expansion between a pair
		// of source/sink nodes.
		// This makes the resulting graph easier to reason about.
		// If sequence was not expanded for the node, do nothing.
		var wrappedReqSubgraph *Graph
		if len(expandedSeqs) > 1 {
			// Create the start and end nodes
			wrappedReqSubgraph, err = r.newReqGraph("repeat_"+nodeSpec.Name, jobArgs)
			if err != nil {
				return nil, err
			}

			// Insert all sequences between the start and end vertices.
			// Place at most `parallel` sequences per parallel expansion.
			// Serialize parallel expansions if number of expanded
			// sequences exceeds `parallel`.
			// Each parallel expansion is wrapped between dummy nodes.
			var parallel uint
			if nodeSpec.Parallel == nil {
				parallel = uint(len(expandedSeqs))
			} else {
				parallel = *nodeSpec.Parallel
			}

			currG, err := r.newReqGraph("repeat_"+nodeSpec.Name, jobArgs)
			if err != nil {
				return nil, err
			}

			prev := wrappedReqSubgraph.Source
			var count uint = 0
			for _, c := range expandedSeqs {
				currG.InsertComponentBetween(c, currG.Source, currG.Sink)
				count++
				if count == parallel {
					wrappedReqSubgraph.InsertComponentBetween(currG, prev, wrappedReqSubgraph.Sink)
					prev = currG.Sink
					currG, err = r.newReqGraph("repeat_"+nodeSpec.Name, jobArgs)
					if err != nil {
						return nil, err
					}
					count = 0
				}
			}
			if count != 0 {
				wrappedReqSubgraph.InsertComponentBetween(currG, prev, wrappedReqSubgraph.Sink)
			}
		} else if len(expandedSeqs) == 1 {
			wrappedReqSubgraph = expandedSeqs[0]
		} else if len(expandedSeqs) == 0 {
			// TODO: L doesn't think this case actually ever happens,
			// but is scared of removing it during a big refactor.
			// Even if there are no lists, we still need to add
			// the node to the graph in order to fulfill dependencies
			// for later nodes.
			wrappedReqSubgraph, err = r.newReqGraph("noop_"+nodeSpec.Name, jobArgs)
			if err != nil {
				return nil, err
			}
		}

		// 3. `wrappedReqSubgraph` is the request subgraph corresponding
		// directly to this sequence graph node. Insert it between its
		// dependencies and the last node.
		idMap[seqGraphNode.Id] = wrappedReqSubgraph
		prevs := seqGraph.GetPrev(seqGraphNode)
		if len(prevs) == 0 {
			// Case: we're processing the sequence graph's source node
			reqGraph.InsertComponentBetween(wrappedReqSubgraph, reqGraph.Source, reqGraph.Sink)
		} else {
			// Case: any other node; insert between sink node of previous node's
			// subgraph, and the last node of the sequence graph
			for _, seqPrev := range prevs {
				prev, ok := idMap[seqPrev.Id]
				if !ok {
					return nil, fmt.Errorf("could not find previous request subgraph of %s; sequence nodes resolved out of order", seqGraphNode.Name)
				}
				reqGraph.InsertComponentBetween(wrappedReqSubgraph, prev.Sink, reqGraph.Sink)
			}
		}
		if err := reqGraph.IsValidGraph(); err != nil {
			return nil, fmt.Errorf("request graph for request %s is not a valid directed acyclic graph after processing node %s: %s",
				seqName, seqGraphNode.Name, err)
		}
	}

	// Sequence IDs of all nodes in this sequence are the UID of the source
	// node.
	// If a node's sequence ID has not been set, then it is a part of this
	// sequence, and we set it.
	// If a node's sequence ID has already been set, then it must've been set
	// by a recursive call to buildSequence. That is, that node is part of a
	// subsequence of this sequence. We don't want to overwrite that sequence
	// ID.
	seqId := reqGraph.Source.Id
	for _, node := range reqGraph.Nodes {
		// Don't overwrite subsequence's sequence IDs.
		if node.SequenceId == "" {
			node.SequenceId = seqId
		}
	}

	// Store configured retry from sequence spec on the first node in the
	// sequence.
	reqGraph.Source.SequenceRetry = cfg.seqRetry
	reqGraph.Source.SequenceRetryWait = cfg.seqRetryWait
	return reqGraph, nil
}

// chooseConditional determines which path of a conditional to take
// based on the value of the job args.
// Assumes `n` is a conditional node.
func chooseConditional(n *spec.Node, jobArgs map[string]interface{}) (string, error) {
	// Node is a conditional, check the value of the "if" jobArg
	val, ok := jobArgs[*n.If]
	if !ok {
		return "", fmt.Errorf("'if' not provided for conditional node")
	}
	valstring, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("'if' arg '%s' is not a string", *n.If)
	}
	// Based on value of "if" jobArg, retrieve sequence to execute
	seqName, ok := n.Eq[valstring]
	if !ok {
		// check if default sequence specified
		seqName, ok = n.Eq["default"]
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

// getIterators splits `each: list:element` values into the list (as a slice)
// and the element.
// Returns the element name, and the slice to iterate over when repeating nodes.
// If there is no repetition required, then it will return an empty string, "",
// and the singleton [""], to indicate that only one iteration is needed.
//
// Precondition: the list must already be present in args
func (r *resolver) getIterators(n *spec.Node, args map[string]interface{}) ([]*string, [][]interface{}, error) {
	empty := []*string{nil}
	empties := [][]interface{}{[]interface{}{""}}
	if len(n.Each) == 0 {
		return empty, empties, nil
	}

	elements := []*string{}
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

		elements = append(elements, &element)
		lists = append(lists, list)
	}

	// Validate that the elements all have variable names
	if len(lists) != len(elements) || len(elements) < 1 {
		return nil, nil, fmt.Errorf("args have different len")
	}
	// Check that all lists are the same length
	listLen := len(lists[0])
	for _, list := range lists {
		if len(list) != listLen {
			return nil, nil, fmt.Errorf("each lists have different lengths")
		}
	}

	return elements, lists, nil
}

// remapeNodeArgs copies args into a new map and renames the arguments
// as defined in the "args" clause.
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

// setNodeArgs copies the args defined in the `sets -> arg` clause into the main
// args map under the name defined by the `sets -> as` field.
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

// buildSingleVertexGraph builds a graph containing a single node.
func (r *resolver) buildSingleVertexGraph(nodeDef *spec.Node, jobArgs map[string]interface{}) (*Graph, error) {
	n, err := r.newNode(nodeDef, jobArgs)
	if err != nil {
		return nil, err
	}
	g := &Graph{
		Name:     nodeDef.Name,
		Source:   n,
		Sink:     n,
		Nodes:    map[string]*Node{n.Id: n},
		Edges:    map[string][]string{},
		RevEdges: map[string][]string{},
	}
	return g, nil
}

// newReqGraph creates an "empty" graph. It contains two nodes: the noop source and sink nodes.
func (r *resolver) newReqGraph(name string, jobArgs map[string]interface{}) (*Graph, error) {
	var err error

	jobArgsCopy, err := remapNodeArgs(&spec.NoopNode, jobArgs)
	if err != nil {
		return nil, err
	}

	source, err := r.newNoopNode(name+"_begin", jobArgsCopy)
	if err != nil {
		return nil, err
	}

	sink, err := r.newNoopNode(name+"_end", jobArgsCopy)
	if err != nil {
		return nil, err
	}

	if err := setNodeArgs(&spec.NoopNode, jobArgs, jobArgsCopy); err != nil {
		return nil, err
	}

	return &Graph{
		Name:     name,
		Source:   source,
		Sink:     sink,
		Nodes:    map[string]*Node{source.Id: source, sink.Id: sink},
		Edges:    map[string][]string{source.Id: []string{sink.Id}},
		RevEdges: map[string][]string{sink.Id: []string{source.Id}},
	}, nil
}

// newNoopNode creates a node witha noop job for use as the graph source and sink.
func (r *resolver) newNoopNode(name string, jobArgs map[string]interface{}) (*Node, error) {
	id, err := r.idGen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for no-op job %s: %s", name, err)
	}
	jid := job.NewIdWithRequestId("noop", name, id, r.request.Id)
	rj, err := r.jobFactory.Make(jid)
	if err != nil {
		switch err {
		case job.ErrUnknownJobType:
			// No custom noop job, use built-in default
			rj = &noopJob{
				id: jid,
			}
		default:
			return nil, fmt.Errorf("Error making no-op job %s: %s", name, err)
		}
	}
	if err := rj.Create(jobArgs); err != nil {
		return nil, fmt.Errorf("Error creating no-op job %s: %s", name, err)
	}
	bytes, err := rj.Serialize()
	if err != nil {
		return nil, fmt.Errorf("Error serializing no-op job %s: %s", name, err)
	}

	return &Node{
		Name:     name,
		Id:       id,
		Spec:     &spec.NoopNode, // on the next refactor, we shouldn't need to set this ourselves
		JobBytes: bytes,
	}, nil
}

// newNode creates job described by node specs `j` and puts it in a node.
func (r *resolver) newNode(j *spec.Node, jobArgs map[string]interface{}) (*Node, error) {
	// Make a copy of the jobArgs before this node gets created and potentially
	// adds additional keys to the jobArgs. A shallow copy is sufficient because
	// args values should never change.
	originalArgs := map[string]interface{}{}
	for k, v := range jobArgs {
		originalArgs[k] = v
	}

	// Make the name of this node unique within the request by assigning it an id.
	id, err := r.idGen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	// Create the job
	rj, err := r.jobFactory.Make(job.NewIdWithRequestId(*j.NodeType, j.Name, id, r.request.Id))
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	if err := rj.Create(jobArgs); err != nil {
		return nil, fmt.Errorf("Error creating '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	bytes, err := rj.Serialize()
	if err != nil {
		return nil, fmt.Errorf("Error serializing '%s %s' job: %s", *j.NodeType, j.Name, err)
	}

	return &Node{
		Name:      j.Name,
		Id:        id,
		Spec:      j, // on the next refactor, we shouldn't need to set this ourselves
		JobBytes:  bytes,
		Args:      originalArgs, // Args is the jobArgs map that this node was created with
		Retry:     j.Retry,
		RetryWait: j.RetryWait,
	}, nil
}
