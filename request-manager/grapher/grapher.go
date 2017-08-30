package grapher

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"

	"github.com/square/spincycle/job"

	"gopkg.in/yaml.v2"
)

// The Grapher struct contains the sequence specs required to construct graphs.
// The user must handle the creation of the Sequence Specs.
// The user will also provide a "no-op" job that Grapher can use to create no-op
// nodes in the graph.
//
// CreateGraph will create a graph. The user must provide a Sequence Type, to indicate
// what graph will be created.
type Grapher struct {
	AllSequences map[string]*SequenceSpec // All sequences that were read in from the Config
	JobFactory   job.Factory              // factory to create nodes' jobs.
	NoopNode     *NodeSpec                // static no-op job that Grapher can use to signify start/end of sequences.

	nextNodeId uint64     // next unique job id
	m          sync.Mutex // mutex to lock nextNodeId
}

////////////////////////////////////////////////////////////////////////////////////////
// Graph struct and friends

// Graph represents a graph. It represents a graph via
// Vertices, a map of vertex name -> Node, and Edges, an
// adjacency list. Also contained in Graph are the First and
// Last Nodes in the graph.
type Graph struct {
	Name     string              // Name of the Graph
	First    *Node               // The source node of the graph
	Last     *Node               // The sink node of the graph
	Vertices map[string]*Node    // All vertices in the graph (node name -> node)
	Edges    map[string][]string // All edges (source node name -> sink node name)
}

// Node represents a single vertex within a Graph.
// Each node consists of a Payload (i.e. the data that the
// user cares about), a list of next and prev Nodes, and other
// information about the node such as the number of times it
// should be retried on error. Next defines all the out edges
// from Node, and Prev defines all the in edges to Node.
type Node struct {
	Datum     Payload                // Data stored at this Node
	Next      map[string]*Node       // out edges ( node name -> Node )
	Prev      map[string]*Node       // in edges ( node name -> Node )
	Args      map[string]interface{} // the args the node was created with
	Retry     uint                   // retry N times if first run fails
	RetryWait uint                   // wait time (milliseconds) between retries
}

// Payload defines the interface of structs that can be
// stored within Graphs. The Create() function will
// be used by Grapher to actually create the Payload.
type Payload interface {
	Create(map[string]interface{}) error
	Serialize() ([]byte, error)
	Type() string
	Name() string
}

////////////////////////////////////////////////////////////////////////////////////////
// Specs for reading in from config file.

// NodeSpec defines the structure expected from the yaml file to define each nodes.
type NodeSpec struct {
	Name         string     `yaml:"name"`     // unique name assigned to this node
	Category     string     `yaml:"category"` // "job" or "sequence"
	NodeType     string     `yaml:"type"`     // the type of job or sequence to create
	Each         string     `yaml:"each"`     // arguments to repeat over
	Args         []*NodeArg `yaml:"args"`     // expected arguments
	Sets         []string   `yaml:"sets"`     // expected job args to be set
	Dependencies []string   `yaml:"deps"`     // nodes with out-edges leading to this node

	// Retry only apply to nodes with category="job". Fields are ignored for "sequence"s
	Retry     uint `yaml:"retry"`     // retry N times if first run fails
	RetryWait uint `yaml:"retryWait"` // wait time (milliseconds) between retries
}

// NodeArg defines the structure expected from the yaml file to define a job's args.
type NodeArg struct {
	Expected string `yaml:"expected"` // the name of the argument that this job expects
	Given    string `yaml:"given"`    // the name of the argument that will be given to this job
}

// SequenceSpec defines the structure expected from the config yaml file to
// define each sequence
type SequenceSpec struct {
	Name  string               `yaml:"name"`  // name of the sequence
	Args  SequenceArgs         `yaml:"args"`  // arguments to the sequence
	Nodes map[string]*NodeSpec `yaml:"nodes"` // list of nodes that are a part of the sequence
}

// SequenceArgs defines the structure expected from the config file to define
// a sequence's arguments. A sequence can have required arguemnts; an arguments
// on this list that are missing will result in an error from Grapher.
// A sequence can also have optional arguemnts; arguments on this list that are
// missing will not result in an error. Additionally optional arguments can
// have default values that will be used if not explicitly given.
type SequenceArgs struct {
	Required []*ArgSpec `yaml:"required"`
	Optional []*ArgSpec `yaml:"optional"`
}

// ArgSpec defines the structure expected from the config to define sequence args.
type ArgSpec struct {
	Name    string `yaml:"name"`
	Default string `yaml:"default"`
}

// All Sequences in the yaml. Also contains the user defined no-op job.
type Config struct {
	Sequences map[string]*SequenceSpec `yaml:"sequences"`
	NoopNode  *NodeSpec                `yaml:"noop-node"`
}

/////////////////////////////////////////////////////////////////////////////////
// unexported structs and vars

var (
	errArgsNotPresent = fmt.Errorf("arguments are missing from nodeArgs")
)

/////////////////////////////////////////////////////////////////////////////////
// Create Graphs

// NewGrapher returns a new Grapher interface. The caller of NewGrapher mut provide
// a Job Factory for Grapher to create the jobs that will be stored at each node.
// The caller must also specify a no-op job and args for Grapher to create no-op nodes.
func NewGrapher(nf job.Factory, cfg *Config) *Grapher {
	o := &Grapher{
		nextNodeId:   1,
		JobFactory:   nf,
		AllSequences: cfg.Sequences,
		NoopNode:     cfg.NoopNode,
	}

	return o
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

//////////////////////////////////////////////////////////////////////////////////
// Helper functions for validating Graphs

// Finds a node (identified by name) in g. Assumes that g contains no cycles
func (g *Graph) FindNode(name string) *Node {
	return findNodeAfterDFS(name, g.First)
}

// returns true iff the graph has at least one cycle in it
func (g *Graph) HasCycles() bool {
	seen := map[string]*Node{g.First.Datum.Name(): g.First}
	return hasCyclesDFS(seen, g.First)
}

// returns true iff every node is reachable from the start node, and every path
// terminates at the end node
func (g *Graph) IsConnected() bool {
	// Check forwards connectivity and backwards connectivity
	return g.connectedToLastNodeDFS(g.First) && g.connectedToFirstNodeDFS(g.Last)
}

// Checks that the adjacency list (given by g.Vertices and g.Edges) matches
// the linked list structure provided through node.Next and node.Prev.
func (g *Graph) AdjacencyListMatchesLL() bool {
	// Create the expected adjacency lists from the linked list structure
	edges, vertices := g.createAdjacencyList()

	// check that all the vertex lists match
	for vertexName, node := range vertices {
		if n, ok := g.Vertices[vertexName]; !ok || n != node {
			return false
		}
	}
	for vertexName, node := range g.Vertices {
		if n, ok := vertices[vertexName]; !ok || n != node {
			return false
		}
	}

	// Check that the edges all match as well
	for source, sinks := range edges {
		if e := g.Edges[source]; !slicesMatch(e, sinks) {
			return false
		}
	}
	for source, sinks := range g.Edges {
		if e := edges[source]; !slicesMatch(e, sinks) {
			return false
		}
	}
	return true
}

// Asserts that g is a valid graph (according to Grapher's use case).
// Ensures that g is acyclic, is connected (not fully connected),
// and the adjacency list matches its linked list.
func (g *Graph) IsValidGraph() bool {
	return !g.HasCycles() && g.IsConnected() && g.AdjacencyListMatchesLL()
}

// Returns true iff every node in nodes is in g
func (g *Graph) ContainsNodes(nodes []string) bool {
	for _, node := range nodes {
		if g.FindNode(node) == nil {
			return false
		}
	}
	return true
}

// Prints out g in DOT graph format.
// To use the output, please refer to the graphviz specs found at: http://www.graphviz.org/
func (g *Graph) PrintDot() {
	fmt.Printf("digraph {\n")
	fmt.Printf("\trankdir=UD;\n")
	fmt.Printf("\tlabelloc=\"t\";\n")
	fmt.Printf("\tlabel=\"%s\"\n", g.Name)
	fmt.Printf("\tfontsize=22\n")
	for vertexName, _ := range g.Vertices {
		fmt.Printf("\tnode [style=filled,color=\"%s\",shape=box]\n", "#86cedf")
		fmt.Printf("\t\"%s\" [label=\"%s\"]\n", vertexName, vertexName)
	}
	for out, ins := range g.Edges {
		for _, in := range ins {
			fmt.Printf("\t\"%s\" -> \"%s\";\n", out, in)
		}
	}
	fmt.Println("}")
}

// ReadConfig will read from configFile and return a Config that the user
// can then use for NewGrapher(). configFile is expected to be in the yaml
// format specified.
func ReadConfig(configFile string) (*Config, error) {
	sequenceData, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = yaml.Unmarshal(sequenceData, cfg)
	if err != nil {
		return nil, err
	}

	for sequenceName, sequence := range cfg.Sequences {
		sequence.Name = sequenceName
		for nodeName, node := range sequence.Nodes {
			node.Name = nodeName
		}
	}

	if cfg.NoopNode == nil {
		return nil, fmt.Errorf("Noop node spec is missing")
	}

	cfg.NoopNode.Name = "noop-job"

	return cfg, nil
}

//////////////////////////////////////////////////////////////////////////////
// unexported helper functions

// returns a unique id to be used for tagging nodes in graph creation
func (o *Grapher) getNewNodeId() uint64 {
	o.m.Lock()
	defer o.m.Unlock()
	myId := o.nextNodeId
	o.nextNodeId += 1
	return myId
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
		if _, ok := args[arg.Name]; !ok && arg.Default != "" {
			args[arg.Name] = arg.Default
		}
	}

	return o.buildComponent("sequence_"+name, seq.Nodes, args)
}

// buildComponent, given a set of node specs, and node args, create a graph that represents the list of node specs,
// nodeArgs represents the arguments that will be passed into the nodes on creation
func (o *Grapher) buildComponent(name string, nodeDefs map[string]*NodeSpec, nodeArgs map[string]interface{}) (*Graph, error) {

	// Start with an empty graph. Enforce a
	// single source and a single sink node here.
	g, err := o.newEmptyGraph(name, nodeArgs)
	if err != nil {
		return nil, err
	}

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

			// First check that all arguments required for the job are present.
			// If not all arguments are present, then find another node to construct
			if !o.allArgsPresent(n, nodeArgs) {
				continue
			}

			// Find out how many times this node has to be repeated
			iterator, iterateOver, err := o.getIterators(n, nodeArgs)
			if err != nil {
				return nil, err
			}

			// All the graphs that make up this component
			componentsForThisNode := []*Graph{}

			// If no repetition is needed, this loop will only execute once
			for _, i := range iterateOver {

				// Copy the required args into a separate args map here.
				// Do the necessary remapping here.
				nodeArgsCopy, err := o.remapNodeArgs(n, nodeArgs)
				if err != nil {
					return nil, err
				}

				// Add the iterator to the node args unless there is no iterator for this node
				if iterator != "" {
					nodeArgsCopy[iterator] = i
				}

				var g *Graph

				if !n.isSequence() {

					// If this node is just a job, then create a graph that contains
					// only that node
					g, err = o.buildSingleVertexGraph(n, nodeArgsCopy)
					if err != nil {
						return nil, err
					}

					// Assert g is a well formed graph
					if !g.IsValidGraph() {
						return nil, fmt.Errorf("malformed graph created")
					}

				} else if n.isSequence() {

					// If this node is a sequence, then recursively construct its components
					sequence, ok := o.AllSequences[n.NodeType]
					if !ok {
						return nil, fmt.Errorf("could not find sequence %s", name)
					}
					g, err = o.buildSequence(n.Name, sequence, nodeArgsCopy)
					if err != nil {
						return nil, err
					}

					// Assert g is a well formed graph
					if !g.IsValidGraph() {
						return nil, fmt.Errorf("malformed graph created")
					}
				}

				// Add the new job to the map of completed components
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
			} else {
				components[n] = componentsForThisNode[0]
			}

			// After all subcomponents are built, remove the job from the jobsToBeDone array
			delete(nodesToBeDone, n)
		}

		// If the number of jobs remaining has not changed, then we are unable to create
		// any new jobs because of missing arguments
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
func (o *Grapher) getIterators(n *NodeSpec, args map[string]interface{}) (string, []interface{}, error) {
	if n.Each == "" {
		return "", []interface{}{""}, nil
	}

	iterateSet := strings.Split(n.Each, ":")[0]
	iterator := strings.Split(n.Each, ":")[1]
	iterateOver := []interface{}{}

	// Grab the iterable set out of args
	iterables, ok := args[iterateSet]
	if !ok {
		return "", []interface{}{""}, fmt.Errorf("could not find required arg: %s", iterateSet)
	}

	// Assert that this is a slice
	if reflect.TypeOf(iterables).Kind() == reflect.Slice {

		a := reflect.ValueOf(iterables)
		for i := 0; i < a.Len(); i++ {
			iterateOver = append(iterateOver, a.Index(i).Interface())
		}

	} else {
		return "", []interface{}{""}, fmt.Errorf("%s is not of kind Slice", iterateSet)
	}

	return iterator, iterateOver, nil
}

// Assert that all arguments required (as defined in the "args" clause in the yaml)
// are present. Returns true if all required arguments are present, and false otherwise.
func (o *Grapher) allArgsPresent(n *NodeSpec, args map[string]interface{}) bool {
	iterateSet := ""
	iterator := ""

	// Assert that the iterable variable is present.
	if n.Each != "" {
		iterator = strings.Split(n.Each, ":")[1]
		iterateSet = strings.Split(n.Each, ":")[0]
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
		Vertices: map[string]*Node{n.Datum.Name(): n},
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

	nodeArgsCopy, err := o.remapNodeArgs(o.NoopNode, nodeArgs)
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

	err = o.setNodeArgs(o.NoopNode, nodeArgs, nodeArgsCopy)
	if err != nil {
		return nil, err
	}

	g.First.Next[g.Last.Datum.Name()] = g.Last
	g.Last.Prev[g.First.Datum.Name()] = g.First
	g.Vertices = map[string]*Node{
		g.First.Datum.Name(): g.First,
		g.Last.Datum.Name():  g.Last,
	}
	g.Edges = map[string][]string{g.First.Datum.Name(): []string{g.Last.Datum.Name()}}
	return g, nil
}

// NewStartNode creates an empty "start" node. There is no job defined for this node, but it can serve
// as a marker for a sequence/request.
func (o *Grapher) newNoopNode(name string, nodeArgs map[string]interface{}) (*Node, error) {
	name = fmt.Sprintf("%s@%d", name, o.getNewNodeId())
	rj, err := o.JobFactory.Make(o.NoopNode.NodeType, name)
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", o.NoopNode.Name, name, err)
	}
	err = rj.Create(nodeArgs)
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", o.NoopNode.Name, name, err)
	}

	return &Node{
		Datum: rj,
		Next:  map[string]*Node{},
		Prev:  map[string]*Node{},
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
	name := fmt.Sprintf("%s@%d", j.Name, o.getNewNodeId())

	// Create the job
	rj, err := o.JobFactory.Make(j.NodeType, name)
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", j.NodeType, name, err)
	}

	err = rj.Create(nodeArgs)
	if err != nil {
		return nil, fmt.Errorf("Error making '%s %s' job: %s", j.NodeType, name, err)
	}

	return &Node{
		Datum:     rj,
		Next:      map[string]*Node{},
		Prev:      map[string]*Node{},
		Args:      originalArgs, // Args is the nodeArgs map that this node was created with
		Retry:     j.Retry,
		RetryWait: j.RetryWait,
	}, nil
}

// isSequence will return true if j is a Sequence, and false otherwise.
func (j *NodeSpec) isSequence() bool {
	return j.Category == "sequence"
}

// Returns true if the last node in g is reachable from n
func (g *Graph) connectedToLastNodeDFS(n *Node) bool {
	if n == nil {
		return false
	}
	if g.Last == n {
		return true
	}
	if g.Last != n && (n.Next == nil || len(n.Next) == 0) {
		return false
	}
	for _, next := range n.Next {

		// Every node after n must also be connected to the last node
		connected := g.connectedToLastNodeDFS(next)
		if !connected {
			return false
		}
	}
	return true
}

// Returns true if n is reachable from the first node in g
func (g *Graph) connectedToFirstNodeDFS(n *Node) bool {
	if n == nil {
		return false
	}
	if g.First == n {
		return true
	}
	if g.First != n && (n.Prev == nil || len(n.Prev) == 0) {
		return false
	}
	for _, prev := range n.Prev {

		// Every node before n must also be connected to the first node
		connected := g.connectedToFirstNodeDFS(prev)
		if !connected {
			return false
		}
	}
	return true
}

// InsertComponentBetween will take a Graph as input, and insert it between the given prev and next nodes.
// Preconditions:
//      component and  g are connected and acyclic
//      prev and next both are present in g
//      next "comes after" prev in the graph, when traversing from the source node
func (g *Graph) insertComponentBetween(component *Graph, prev *Node, next *Node) error {
	// Cannot check for the adjacency list match here because of the way we insert components.
	if g.HasCycles() || component.HasCycles() ||
		!g.IsConnected() || !component.IsConnected() {
		return fmt.Errorf("Graph not valid!")
	}

	// have component point to prev and next nodes
	component.First.Prev[prev.Datum.Name()] = prev
	component.Last.Next[next.Datum.Name()] = next

	// have prev and next nodes point to component
	prev.Next[component.First.Datum.Name()] = component.First
	next.Prev[component.Last.Datum.Name()] = component.Last

	// Remove edges between prev and next if it exists
	delete(prev.Next, next.Datum.Name())
	delete(next.Prev, prev.Datum.Name())

	// update vertices list

	// Add in the new vertices
	for k, v := range component.Vertices {
		g.Vertices[k] = v
	}

	// update adjacency list
	for k, v := range component.Edges {
		g.Edges[k] = v
	}

	// for the edges of the previous node, add the start node of this component
	g.Edges[prev.Datum.Name()] = append(g.Edges[prev.Datum.Name()], component.First.Datum.Name())

	// for the edges of the last node of this component, add the next node
	if find(g.Edges[component.Last.Datum.Name()], next.Datum.Name()) < 0 {
		g.Edges[component.Last.Datum.Name()] = append(g.Edges[component.Last.Datum.Name()], next.Datum.Name())
	}

	// Remove all occurences next from the adjacency list of prev
	i := find(g.Edges[prev.Datum.Name()], next.Datum.Name())
	for i >= 0 {
		g.Edges[prev.Datum.Name()][i] = g.Edges[prev.Datum.Name()][len(g.Edges[prev.Datum.Name()])-1]
		g.Edges[prev.Datum.Name()] = g.Edges[prev.Datum.Name()][:len(g.Edges[prev.Datum.Name()])-1]
		i = find(g.Edges[prev.Datum.Name()], next.Datum.Name())
	}

	// verify resulting graph is ok
	if g.HasCycles() || !g.IsConnected() {
		return fmt.Errorf("graph not valid after insert")
	}
	return nil
}

// returns the index of s in ss, returns -1 if s is not found in ss
func find(ss []string, s string) int {
	for i, j := range ss {
		if j == s {
			return i
		}
	}
	return -1
}

// Returns a list of edges and vertices based on the linked list structure
// of the graph. Useful for asserting that the structures match and for error checking.
func (g *Graph) createAdjacencyList() (map[string][]string, map[string]*Node) {
	edges := map[string][]string{}
	vertices := []string{}
	s := edges

	n := g.First

	// Classic BFS
	seen := map[string]*Node{}
	frontier := map[string]*Node{}

	for name, node := range n.Next {
		frontier[name] = node
	}
	seen[n.Datum.Name()] = n

	// Search while the frontier set is non-empty
	for len(frontier) > 0 {

		// For every node in the frontier
		for name, next := range frontier {

			// Look at the edges connecting that node to a node in the seen set.
			for n, _ := range next.Prev {

				// If this edge has not been seen yet, add it to the edge list
				if _, ok := s[n]; !ok {
					s[n] = []string{}
				}
				s[n] = append(s[n], name)
			}

			// add each "next" node to the frontier set
			for k, v := range next.Next {
				if _, ok := seen[k]; !ok {
					frontier[k] = v
				}
			}

			// delete node from the frontier set
			delete(frontier, name)
			seen[name] = next
		}
	}

	// Build vertex list from seen list
	for name, _ := range seen {
		vertices = append(vertices, name)
	}
	return edges, seen
}

// find node (identified by name) that appears after n using DFS.
// Precondition: graph has no cycles.
func findNodeAfterDFS(name string, n *Node) *Node {
	if n == nil {
		return nil
	}
	if n.Datum.Name() == name {
		return n
	}
	if n.Next == nil || len(n.Next) == 0 {
		return nil
	}
	for _, next := range n.Next {
		found := findNodeAfterDFS(name, next)
		if found != nil {
			return found
		}
	}
	return nil
}

// Determines if a graph has cycles, using dfs
// precondition: start node is already in seen list
func hasCyclesDFS(seen map[string]*Node, start *Node) bool {
	for _, next := range start.Next {

		// If the next node has already  been seen, return true
		if _, ok := seen[next.Datum.Name()]; ok {
			return true
		}

		// Add next node to seen list
		seen[next.Datum.Name()] = next

		// Continue searching after next node
		if hasCyclesDFS(seen, next) {
			return true
		}

		// Remove next node from seen list
		delete(seen, next.Datum.Name())
	}
	return false
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

// Returns true if a matches b, regardless of ordering
func slicesMatch(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i, _ := range a {
		ok := false
		for j, _ := range b {
			if a[i] == b[j] {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}

	return true
}
