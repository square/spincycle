// Copyright 2017-2018, Square, Inc.

package grapher

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/id"
)

const DEFAULT = "default"

// The Grapher struct contains the sequence specs required to construct graphs.
// The user must handle the creation of the Sequence Specs.
//
// CreateGraph will create a graph. The user must provide a Sequence Type, to indicate
// what graph will be created.
type Grapher struct {
	AllSequences map[string]*SequenceSpec // All sequences that were read in from the Config
	JobFactory   job.Factory              // factory to create nodes' jobs.

	idgen id.Generator // Generates UIDs for the nodes created by the Grapher.
	req   proto.Request
}

// NewGrapher returns a new Grapher struct. The caller of NewGrapher must provide
// a Job Factory for Grapher to create the jobs that will be stored at each node.
// An id generator must also be provided (used for generating ids for nodes).
//
// A new Grapher should be made for every request.
func NewGrapher(req proto.Request, nf job.Factory, cfg Config, idgen id.Generator) *Grapher {
	o := &Grapher{
		JobFactory:   nf,
		AllSequences: cfg.Sequences,
		idgen:        idgen,
		req:          req,
	}
	return o
}

// A GrapherFactory makes Graphers.
type GrapherFactory interface {
	// Make makes a Grapher. A new grapher should be made for every request.
	Make(proto.Request) *Grapher
}

// grapherFactory implements the GrapherFactory interface.
type grapherFactory struct {
	jf     job.Factory
	config Config
	idf    id.GeneratorFactory
}

// NewGrapherFactory creates a GrapherFactory.
func NewGrapherFactory(jf job.Factory, cfg Config, idf id.GeneratorFactory) GrapherFactory {
	return &grapherFactory{
		jf:     jf,
		config: cfg,
		idf:    idf,
	}
}

func (gf *grapherFactory) Make(req proto.Request) *Grapher {
	return NewGrapher(req, gf.jf, gf.config, gf.idf.Make()) // create a Grapher with a new id Generator
}

// CreateGraph will create a graph. The user must provide a Sequence Name, to indicate
// what graph will be created. The caller must also provide the first set of args.
func (o *Grapher) CreateGraph(sequenceName string, args map[string]interface{}) (*Graph, error) {
	request, ok := o.AllSequences[sequenceName]
	if !ok {
		return nil, fmt.Errorf("cannot find definition for request: %s", sequenceName)
	}
	g, err := o.buildSequence(request.Name, request, args)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *Grapher) Sequences() map[string]*SequenceSpec {
	return g.AllSequences
}

// buildSequence will take in a sequence spec and return a Graph that represents the sequence
func (o *Grapher) buildSequence(name string, seq *SequenceSpec, args map[string]interface{}) (*Graph, error) {

	// Verify all required arguments are present
	for _, arg := range seq.Args.Required {
		if _, ok := args[arg.Name]; !ok {
			return nil, fmt.Errorf("sequence %s missing arg %s", name, arg.Name)
		}
	}

	// Verify all optional arguments with defaults provided are included
	for _, arg := range seq.Args.Optional {
		if _, ok := args[arg.Name]; !ok {
			args[arg.Name] = arg.Default
		}
	}

	// Verify all static arguments with defaults provided are included
	for _, arg := range seq.Args.Static {
		if _, ok := args[arg.Name]; !ok {
			args[arg.Name] = arg.Default
		}
	}
	return o.buildComponent("sequence_"+name, seq.Nodes, args, seq.Retry)
}

func (o *Grapher) buildConditional(name string, n *NodeSpec, nodeArgs map[string]interface{}) (*Graph, error) {
	// Node is a conditional, check the value of the "if" jobArg
	if n.If == "" {
		return nil, fmt.Errorf("No 'if' jobArg specified for conditional")
	}
	val, ok := nodeArgs[n.If]
	if !ok {
		return nil, fmt.Errorf("could not find the conditional jobArg %s", n.If)
	}
	valstring, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("did not provide string type sequence name for conditional")
	}
	// Based on value of "if" jobArg, get which sequence to execute
	seqName, ok := n.Eq[valstring]
	if !ok {
		// check if default sequence specified
		seqName, ok = n.Eq[DEFAULT]
		if !ok {
			return nil, fmt.Errorf("the value of the conditional jobArg %s did not match any of the options", n.If)
		}
	}
	// Build the correct sequence
	sequence, ok := o.AllSequences[seqName]
	if !ok {
		return nil, fmt.Errorf("could not find sequence %s", name)
	}
	sequence.Retry = n.Retry
	s, err := o.buildSequence(seqName, sequence, nodeArgs)
	if err != nil {
		return nil, err
	}

	// Create the start and end nodes
	g, err := o.newEmptyGraph("conditional_"+n.Name, nodeArgs)
	if err != nil {
		return nil, err
	}

	// Insert all components between the start and end vertices.
	g.insertComponentBetween(s, g.First, g.Last)

	return g, nil
}

// buildComponent, given a set of node specs, and node args, create a graph that represents the list of node specs,
// nodeArgs represents the arguments that will be passed into the nodes on creation
func (o *Grapher) buildComponent(name string, nodeDefs map[string]*NodeSpec, nodeArgs map[string]interface{}, sequenceRetry uint) (*Graph, error) {

	// Start with an empty graph. Enforce a
	// single source and a single sink node here.
	g, err := o.newEmptyGraph(name, nodeArgs)
	if err != nil {
		return nil, err
	}

	// Store configured retry from sequence spec on the first node in the
	// sequence
	g.First.SequenceRetry = sequenceRetry

	//////////////////////////////////////////////////////
	// First, construct all subcomponents of this graph

	// components is a map of all sub-components of this graph
	components := map[*NodeSpec]*Graph{}

	// nodesToBeDone is a map of all jobs that have not yet been built
	nodesToBeDone := map[*NodeSpec]bool{}
	for _, node := range nodeDefs {
		nodesToBeDone[node] = true
	}
	nodesCount := len(nodesToBeDone)

	for nodesCount > 0 {

		for n, _ := range nodesToBeDone {

			// First check that all arguments required for the node are present.
			// If not all arguments are present, then find another node to construct
			if !o.allArgsPresent(n, nodeArgs) {
				continue
			}

			// Find out how many times this node has to be repeated
			iterators, iterateOvers, err := o.getIterators(n, nodeArgs)
			if err != nil {
				return nil, err
			}

			// All the graphs that make up this component
			componentsForThisNode := []*Graph{}

			// Validate that the iterators all have variable names
			if len(iterateOvers) != len(iterators) || len(iterators) < 1 {
				return nil, fmt.Errorf("Error making node %s: malformed arguments given.", n.Name)
			}

			// If no repetition is needed, this loop will only execute once
			for i, _ := range iterateOvers[0] {

				// Copy the required args into a separate args map here.
				// Do the necessary remapping here.
				nodeArgsCopy, err := o.remapNodeArgs(n, nodeArgs)
				if err != nil {
					return nil, err
				}

				// Add the iterator to the node args unless there is no iterator for this node
				for j, iterator := range iterators {
					if iterator != "" {
						// This won't panic because we have earlier asserted that
						// len(iterators) == len(iterateOvers)
						nodeArgsCopy[iterator] = iterateOvers[j][i]
					}
				}

				// Add the "if" to the node args if it's present
				if ifArg, ok := nodeArgs[n.If]; ok {
					nodeArgsCopy[n.If] = ifArg
				}

				// Build next graph component and assert that it's valid
				var g *Graph
				if n.isConditional() {
					// Node is a conditional
					g, err = o.buildConditional(name, n, nodeArgsCopy)
					if err != nil {
						return nil, err
					}
				} else if n.isSequence() {
					// Node is a sequence, recursively construct its components
					sequence, ok := o.AllSequences[n.NodeType]
					if !ok {
						return nil, fmt.Errorf("could not find sequence %s", name)
					}
					sequence.Retry = n.Retry
					g, err = o.buildSequence(n.Name, sequence, nodeArgsCopy)
					if err != nil {
						return nil, err
					}
				} else {
					// Node is a job, create a graph that contains only the node
					g, err = o.buildSingleVertexGraph(n, nodeArgsCopy)
					if err != nil {
						return nil, err
					}
				}
				if !g.IsValidGraph() {
					return nil, fmt.Errorf("malformed graph created")
				}

				// Add the new node to the map of completed components
				componentsForThisNode = append(componentsForThisNode, g)

				// If the node (or sequence) was determined to set any args
				// copy them from nodeArgsCopy into the main nodeArgs
				err = o.setNodeArgs(n, nodeArgs, nodeArgsCopy)
				if err != nil {
					return nil, err
				}
			}

			// If this component was repeated multiple times,
			// wrap it between a single dummy start and end vertices.
			// This makes the resulting graph easier to reason about.
			// If there are no components for the node, do nothing.
			if len(componentsForThisNode) > 1 {

				// Create the start and end nodes
				g, err := o.newEmptyGraph("repeat_"+n.Name, nodeArgs)
				if err != nil {
					return nil, err
				}

				// Insert all components between the start and end vertices.
				for _, c := range componentsForThisNode {
					g.insertComponentBetween(c, g.First, g.Last)
				}

				// Assert g is a well formed graph
				if !g.IsValidGraph() {
					return nil, fmt.Errorf("malformed graph created")
				}

				components[n] = g
			} else if len(componentsForThisNode) == 1 {
				components[n] = componentsForThisNode[0]
			} else if len(componentsForThisNode) == 0 {
				// Even if there are no iterateOvers, we still need to add
				// the node to the graph in order to fulfill dependencies
				// for later nodes.
				g, err := o.newEmptyGraph("noop_"+n.Name, nodeArgs)
				if err != nil {
					return nil, err
				}
				// Assert g is a well formed graph
				if !g.IsValidGraph() {
					return nil, fmt.Errorf("malformed graph created")
				}
				components[n] = g
			}

			// After all subcomponents are built, remove the node from the nodesToBeDone array
			delete(nodesToBeDone, n)
		}

		// If the number of nodes remaining has not changed, then we are unable to create
		// any new nodes because of missing arguments
		if nodesCount == len(nodesToBeDone) {
			missingArgsNodes := []string{}
			for n, _ := range nodesToBeDone {
				missingArgsNodes = append(missingArgsNodes, n.Name)
			}
			return nil, fmt.Errorf("Job Args are missing for %v", missingArgsNodes)
		}

		nodesCount = len(nodesToBeDone)
	}

	///////////////////////////////////////////////////////////
	// Second, put all subcomponents together to create graph

	// Create list of all components to add to graph
	componentsToAdd := map[*NodeSpec]*Graph{}
	for k, v := range components {
		componentsToAdd[k] = v
	}

	// Repeat until all components have been added to graph
	componentsAdded := map[*NodeSpec]bool{}
	componentsRemaining := len(componentsToAdd)
	for componentsRemaining > 0 {

		// Build graph by adding components, starting from the source node, and then
		// adding all adjacent nodes to the source node, and so on.
		// We cannot add components in any order because we do not know the reverse dependencies
		for node, component := range componentsToAdd {

			// If there are no dependencies, then this job will come "first". Insert it
			// directly after the Start node.
			if len(node.Dependencies) == 0 {
				err := g.insertComponentBetween(component, g.First, g.Last)
				if err != nil {
					return nil, err
				}
				delete(componentsToAdd, node)
				componentsAdded[node] = true

			} else if containsAll(componentsAdded, node.Dependencies, nodeDefs) {
				// If all the dependencies for this job have been added to the graph,
				// then add it. If not all the dependecies have been added, skip it for now.

				// Insert the component between all its dependencies and the end node.
				for _, dependencyName := range node.Dependencies {
					dependency := nodeDefs[dependencyName]
					prevComponent := components[dependency]
					err := g.insertComponentBetween(component, prevComponent.Last, g.Last)
					if err != nil {
						return nil, err
					}
				}

				// remove this node from the components to add list
				delete(componentsToAdd, node)
				componentsAdded[node] = true
			}

		}

		// If the number of components remaining has not changed,
		// then there exists a cycle in the graph.
		if componentsRemaining == len(componentsToAdd) {
			cs := []string{}
			for c, _ := range componentsToAdd {
				cs = append(cs, c.Name)
			}
			return nil, fmt.Errorf("Impossible dependencies found amongst: %v", cs)
		}
		componentsRemaining = len(componentsToAdd)
	}

	///////////////////////////////////////////////////////////
	// Third, mark all vertices in sequence except start vertex
	// start sequence id

	// A graph is built by constructing its inner most components, which are
	// sequences, and building its way out. When constructing the inner most
	// sequence, we want to set SequenceId for all but the first vertex in the
	// sequence. The SequenceId for the first vertex in the sequence will be set
	// on a subsequent pass. Lastly, the first vertex in the completed graph will
	// have no SequenceId set, as that vertex is part of a larger sequence.
	sequenceId := g.First.Datum.Id().Id
	for _, vertex := range g.Vertices {
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

	// Assert g is a well formed graph
	if !g.IsValidGraph() {
		return nil, fmt.Errorf("malformed graph created")
	}

	return g, nil
}

// Gets the iterables and iterators ( the "each" clause in the yaml )
// returns the iterator name, and the slice to iterate over when repeating nodes.
// If there is no repetition required, then it will return an empty string, "",
// and the singleton [""], to indicate that only one iteration is needed.
//
// Precondition: the iteratable must already be present in args
func (o *Grapher) getIterators(n *NodeSpec, args map[string]interface{}) ([]string, [][]interface{}, error) {
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

	return iterators, iterateOvers, nil
}

// Assert that all arguments required (as defined in the "args" clause in the yaml)
// are present. Returns true if all required arguments are present, and false otherwise.
func (o *Grapher) allArgsPresent(n *NodeSpec, args map[string]interface{}) bool {
	iterateSet := ""
	iterator := ""

	// Assert that the iterable variable is present.
	for _, each := range n.Each {
		if each == "" {
			continue
		}

		// This is malformed input.
		if len(strings.Split(each, ":")) != 2 {
			return false
		}

		iterator = strings.Split(each, ":")[1]
		iterateSet = strings.Split(each, ":")[0]
		if _, ok := args[iterateSet]; !ok {
			return false
		}
	}

	// Assert all other defined args are present
	for _, arg := range n.Args {
		if arg.Expected == iterator {
			continue // this one we can expect to not have
		}
		if _, ok := args[arg.Given]; !ok {
			return false
		}
	}
	return true
}

// Given a node definition and an args, copy args into a new map,
// but also rename the arguments as defined in the "args" clause.
// A shallow copy is sufficient because args values should never
// change.
func (o *Grapher) remapNodeArgs(n *NodeSpec, args map[string]interface{}) (map[string]interface{}, error) {
	nodeArgs2 := map[string]interface{}{}
	for _, arg := range n.Args {
		var ok bool
		nodeArgs2[arg.Expected], ok = args[arg.Given]
		if !ok {
			return nil, fmt.Errorf("cannot create job %s: missing %s from job args", n.NodeType, arg.Given)
		}
	}
	return nodeArgs2, nil
}

// Given a node definition and two args sets. Copy the arguments that
// are defined in the "sets" clause into the main args map.
func (o *Grapher) setNodeArgs(n *NodeSpec, argsTo, argsFrom map[string]interface{}) error {
	if len(n.Sets) == 0 {
		return nil
	}
	for _, key := range n.Sets {
		var ok bool
		var val interface{}
		val, ok = argsFrom[key]
		if !ok {
			return fmt.Errorf("expected %s to set %s in jobargs", n.NodeType, key)
		}
		argsTo[key] = val
	}

	return nil
}

// Builds a graph containing a single node
func (o *Grapher) buildSingleVertexGraph(nodeDef *NodeSpec, nodeArgs map[string]interface{}) (*Graph, error) {
	n, err := o.newNode(nodeDef, nodeArgs)
	if err != nil {
		return nil, err
	}
	g := &Graph{
		Name:     nodeDef.Name,
		First:    n,
		Last:     n,
		Vertices: map[string]*Node{n.Datum.Id().Id: n},
		Edges:    map[string][]string{},
	}
	return g, nil
}

// NewEmptyGraph creates an "empty" graph. It contains two nodes: the "start" and "end" nodes. Both of these nodes
// are no-op jobs
func (o *Grapher) newEmptyGraph(name string, nodeArgs map[string]interface{}) (*Graph, error) {
	var err error
	g := &Graph{
		Name: name,
	}

	nodeArgsCopy, err := o.remapNodeArgs(noopSpec, nodeArgs)
	if err != nil {
		return nil, err
	}

	g.First, err = o.newNoopNode(name+"_start", nodeArgsCopy)
	if err != nil {
		return nil, err
	}

	g.Last, err = o.newNoopNode(name+"_end", nodeArgsCopy)
	if err != nil {
		return nil, err
	}

	err = o.setNodeArgs(noopSpec, nodeArgs, nodeArgsCopy)
	if err != nil {
		return nil, err
	}

	g.First.Next[g.Last.Datum.Id().Id] = g.Last
	g.Last.Prev[g.First.Datum.Id().Id] = g.First
	g.Vertices = map[string]*Node{
		g.First.Datum.Id().Id: g.First,
		g.Last.Datum.Id().Id:  g.Last,
	}
	g.Edges = map[string][]string{g.First.Datum.Id().Id: []string{g.Last.Datum.Id().Id}}
	return g, nil
}

// NewStartNode creates an empty "start" node. There is no job defined for this node, but it can serve
// as a marker for a sequence/request.
func (o *Grapher) newNoopNode(name string, nodeArgs map[string]interface{}) (*Node, error) {
	id, err := o.idgen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for no-op node %s: %s", name, err)
	}
	jid := job.NewIdWithRequestId("noop", "noop", id, o.req.Id)
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
	err = rj.Create(nodeArgs)
	if err != nil {
		return nil, fmt.Errorf("Error creating no-op node %s: %s", name, err)
	}

	return &Node{
		Datum: rj,
		Next:  map[string]*Node{},
		Prev:  map[string]*Node{},
		Name:  name,
	}, nil
}

// newNode creates a node for the given job j
func (o *Grapher) newNode(j *NodeSpec, nodeArgs map[string]interface{}) (*Node, error) {
	// Make a copy of the nodeArgs before this node gets created and potentially
	// adds additional keys to the nodeArgs. A shallow copy is sufficient because
	// args values should never change.
	originalArgs := map[string]interface{}{}
	for k, v := range nodeArgs {
		originalArgs[k] = v
	}

	// Make the name of this node unique within the request by assigning it an id.
	id, err := o.idgen.UID()
	if err != nil {
		return nil, fmt.Errorf("Error making id for '%s %s' job: %s", j.NodeType, j.Name, err)
	}

	// Create the job
	rj, err := o.JobFactory.Make(job.NewIdWithRequestId(j.NodeType, j.Name, id, o.req.Id))
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", j.NodeType, j.Name, err)
	}

	err = rj.Create(nodeArgs)
	if err != nil {
		return nil, fmt.Errorf("Error creating '%s %s' job: %s", j.NodeType, j.Name, err)
	}

	return &Node{
		Datum:     rj,
		Next:      map[string]*Node{},
		Prev:      map[string]*Node{},
		Name:      j.Name,
		Args:      originalArgs, // Args is the nodeArgs map that this node was created with
		Retry:     j.Retry,
		RetryWait: j.RetryWait,
	}, nil
}

// containsAll is a convenience function for checking membership in a map.
// Returns true if m contains every elements in ss
func containsAll(m map[*NodeSpec]bool, ss []string, nodes map[string]*NodeSpec) bool {
	for _, s := range ss {
		name := nodes[s]
		if _, ok := m[name]; !ok {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------------- //

// Mock grapher factory for testing.
type MockGrapherFactory struct {
	MakeFunc func(proto.Request) *Grapher
}

func (gf *MockGrapherFactory) Make(req proto.Request) *Grapher {
	if gf.MakeFunc != nil {
		return gf.MakeFunc(req)
	}
	return nil
}
