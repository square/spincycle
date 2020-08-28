// Copyright 2020, Square, Inc.

package template

import (
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Node in template graph. Implments graph.Node.
type Node struct {
	Spec   *spec.Node // sequence node that this graph node represents
	NodeId string     // unique ID of node within template
}

func (g *Node) Id() string {
	return g.NodeId
}

func (g *Node) Name() string {
	return g.Spec.Name
}

// graph prety printer function already prints relevant info; there's nothing to add here
func (g *Node) String() string {
	return ""
}

// Template graph (containing template nodes). Includes graph itself, some other metainfo,
// and useful functions.
// Built by template.Grapher while performing graph checks. Serves as template for creator.Chain
// when building job chains.
type Graph struct {
	// Graph of Nodes. Write operations must be performed using methods below. Can be read directly.
	graph graph.Graph
	// Set of arguments set by sequence described by graph. Should be read/written directly.
	sets map[string]bool

	// List of nodes in graph in topological order. All operations, read and write, must be performed using methods below.
	iterator []*Node
	// Make sure we don't duplicate nodes.
	iteratorSet map[*Node]bool
	// Generates UIDs for this graph. Should not be accessed externally.
	idgen id.Generator
}

// Iterate through graph in topological order. Order is not deterministic, but order for
// a single instance of a template graph will be the same eveyr time.
func (t *Graph) Iterator() []*Node {
	return append(t.iterator, t.graph.Last.(*Node))
}

// Get previous nodes (in edges) as node id --> node
func (t *Graph) GetPrev(n graph.Node) map[string]graph.Node {
	return t.graph.GetPrev(n)
}

// Get new (empty) graph with single source and single sink node.
func newGraph(name string, idgen id.Generator) (*Graph, error) {
	t := &Graph{
		sets:  map[string]bool{},
		idgen: idgen,
	}
	err := t.initGraph(name)
	if err != nil {
		return nil, err
	}
	t.iterator = []*Node{t.graph.First.(*Node)}
	t.iteratorSet = map[*Node]bool{t.graph.First.(*Node): true}
	return t, nil
}

func (t *Graph) initGraph(name string) error {
	first, err := t.newNoopNode(name + "_start")
	if err != nil {
		return err
	}

	last, err := t.newNoopNode(name + "_end")
	if err != nil {
		return err
	}

	t.graph = graph.Graph{
		Name:     name,
		First:    first,
		Last:     last,
		Vertices: map[string]graph.Node{first.Id(): first, last.Id(): last},
		Edges:    map[string][]string{first.Id(): []string{last.Id()}},
		RevEdges: map[string][]string{last.Id(): []string{first.Id()}},
	}

	return nil
}

// Insert node `node` between `prev` and sink node.
func (t *Graph) addNodeAfter(node *Node, prev graph.Node) error {
	subgraph := &graph.Graph{
		Name:     node.Name(),
		First:    node,
		Last:     node,
		Vertices: map[string]graph.Node{node.Id(): node},
		Edges:    map[string][]string{},
		RevEdges: map[string][]string{},
	}
	err := t.graph.InsertComponentBetween(subgraph, prev, t.graph.Last)
	if err != nil {
		return err
	}
	if !t.iteratorSet[node] {
		t.iterator = append(t.iterator, node)
		t.iteratorSet[node] = true
	}
	return nil
}

// Generate a new node for this template graph with payload `node`.
func (t *Graph) newNode(node *spec.Node) (*Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	return &Node{
		Spec:   node,
		NodeId: id,
	}, nil
}

// Generate a new noop node for this template graph with name `name`.
func (t *Graph) newNoopNode(name string) (*Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	noopSpec := spec.NoopNode // copy
	noopSpec.Name = name
	return &Node{
		Spec:   &noopSpec,
		NodeId: id,
	}, nil
}
