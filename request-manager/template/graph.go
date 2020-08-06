// Copyright 2020, Square, Inc.

package template

import (
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Node in template graph. Implments graph.Node.
type Node struct {
	// payload
	Node *spec.Node // sequence node that this graph node represents

	// node metainfo fields
	NodeId string                // unique ID of node within template
	Next   map[string]graph.Node // out edges (node id --> *Node)
	Prev   map[string]graph.Node // in edges (node id --> *Node)
}

func (g *Node) Id() string {
	return g.NodeId
}

func (g *Node) GetNext() *map[string]graph.Node {
	return &g.Next
}

func (g *Node) GetPrev() *map[string]graph.Node {
	return &g.Prev
}

func (g *Node) Name() string {
	return g.Node.Name
}

// graph prety printer function already prints relevant info; there's nothing to add here
func (g *Node) String() string {
	return ""
}

// Template graph (containing template nodes). Includes graph itself, some other metainfo,
// and useful functions.
// Graph will have a single source and a single sink node. Graph modification functions enforce this.
type Graph struct {
	// Graph of Nodes. Write operations must be performed using methods below. Can be read directly.
	Graph graph.Graph
	// Set of arguments set by sequence described by graph. Can be read/written directly.
	Sets map[string]bool

	// List of nodes in graph in topological order. All operations, read and write, must be performed using methods below.
	iterator []*Node
	// Generates UIDs for this graph. Should not be accessed externally.
	idgen id.Generator
}

// Iterate through graph in topological order. Order is not deterministic, but order for
// a single instance of a template graph will be the same eveyr time.
func (t *Graph) Iterator() []*Node {
	return append(t.iterator, t.Graph.Last.(*Node))
}

// Get new (empty) graph with single source and single sink node.
func newGraph(name string, idgen id.Generator) (*Graph, error) {
	t := &Graph{
		Sets:  map[string]bool{},
		idgen: idgen,
	}
	err := t.initGraph(name)
	if err != nil {
		return nil, err
	}
	t.iterator = []*Node{t.Graph.First.(*Node)}
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

	first.Next[last.Id()] = last
	last.Prev[first.Id()] = first

	t.Graph = graph.Graph{
		Name:     name,
		First:    first,
		Last:     last,
		Vertices: map[string]graph.Node{first.Id(): first, last.Id(): last},
		Edges:    map[string][]string{first.Id(): []string{last.Id()}},
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
	}
	err := t.Graph.InsertComponentBetween(subgraph, prev, t.Graph.Last)
	if err != nil {
		return err
	}
	t.iterator = append(t.iterator, node)
	return nil
}

// Generate a new node for this template graph with payload `node`.
func (t *Graph) newNode(node *spec.Node) (*Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	return &Node{
		Node:   node,
		NodeId: id,
		Next:   map[string]graph.Node{},
		Prev:   map[string]graph.Node{},
	}, nil
}

// Generate a new noop node for this template graph with name `name`.
func (t *Graph) newNoopNode(name string) (*Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	noopCategory := "job"
	noopType := "noop"
	return &Node{
		Node: &spec.Node{
			Name:     name,
			Category: &noopCategory,
			NodeType: &noopType,
		},
		NodeId: id,
		Next:   map[string]graph.Node{},
		Prev:   map[string]graph.Node{},
	}, nil
}
