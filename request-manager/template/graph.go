// Copyright 2020, Square, Inc.

package template

import (
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

type Node struct {
	// payload
	NodeSpec *spec.NodeSpec // sequence node that this graph node represents

	// fields required by graph.Node interface
	Id   string
	Next map[string]graph.Node
	Prev map[string]graph.Node
}

func (g *Node) GetId() string {
	return g.Id
}

func (g *Node) GetNext() *map[string]graph.Node {
	return &g.Next
}

func (g *Node) GetPrev() *map[string]graph.Node {
	return &g.Prev
}

func (g *Node) GetName() string {
	return g.NodeSpec.Name
}

// graph prety printer function already prints relevant info; there's nothing to add here
func (g *Node) String() string {
	return ""
}

type Graph struct {
	Graph graph.Graph     // Graph of Nodes. All modifications handled by methods below.
	Sets  map[string]bool // Set of arguments set by sequence described by graph. Should be directly managed by caller.

	iterator []*Node      // List of nodes in graph in topological order. All interactions handled by methods below.
	idgen    id.Generator // Generates UIDs for this graph. Should not be called or modified.
}

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

func (t *Graph) Iterator() []*Node {
	return append(t.iterator, t.Graph.Last.(*Node))
}

func (t *Graph) NewNode(node *spec.NodeSpec) (*Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	return &Node{
		NodeSpec: node,
		Id:       id,
		Next:     map[string]graph.Node{},
		Prev:     map[string]graph.Node{},
	}, nil
}

func (t *Graph) AddNodeAfter(node *Node, prev graph.Node) error {
	subgraph := &graph.Graph{
		Name:     node.GetName(),
		First:    node,
		Last:     node,
		Vertices: map[string]graph.Node{node.GetId(): node},
		Edges:    map[string][]string{},
	}
	err := t.Graph.InsertComponentBetween(subgraph, prev, t.Graph.Last)
	if err != nil {
		return err
	}
	t.iterator = append(t.iterator, node)
	return nil
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

	first.Next[last.GetId()] = last
	last.Prev[first.GetId()] = first

	t.Graph = graph.Graph{
		Name:     name,
		First:    first,
		Last:     last,
		Vertices: map[string]graph.Node{first.GetId(): first, last.GetId(): last},
		Edges:    map[string][]string{first.GetId(): []string{last.GetId()}},
	}

	return nil
}

func (t *Graph) newNoopNode(name string) (*Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	noopCategory := "job"
	noopType := "noop"
	return &Node{
		NodeSpec: &spec.NodeSpec{
			Name:     name,
			Category: &noopCategory,
			NodeType: &noopType,
		},
		Id:   id,
		Next: map[string]graph.Node{},
		Prev: map[string]graph.Node{},
	}, nil
}
