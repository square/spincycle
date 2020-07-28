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

func (g Node) GetId() string {
	return g.Id
}

func (g Node) GetNext() *map[string]graph.Node {
	return &g.Next
}

func (g Node) GetPrev() *map[string]graph.Node {
	return &g.Prev
}

func (g Node) GetName() string {
	return g.NodeSpec.Name
}

// graph prety printer function already prints relevant info; there's nothing to add here
func (g Node) String() string {
	return ""
}

type Graph struct {
	Graph graph.Graph     // graph of Nodes
	Sets  map[string]bool // set of arguments set by sequence described by graph

	idgen id.Generator // generates UIDs for this graph
}

func NewGraph(name string, idgen id.Generator) (*Graph, error) {
	t := &Graph{
		Sets:  map[string]bool{},
		idgen: idgen,
	}
	err := t.initGraph(name)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Graph) NewSubgraph(node *spec.NodeSpec) (*graph.Graph, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	n := Node{
		NodeSpec: node,
		Id:       id,
		Next:     map[string]graph.Node{},
		Prev:     map[string]graph.Node{},
	}
	return &graph.Graph{
		Name:     node.Name,
		First:    n,
		Last:     n,
		Vertices: map[string]graph.Node{n.GetId(): n},
		Edges:    map[string][]string{},
	}, nil
}

func (t *Graph) initGraph(name string) error {
	var err error
	t.Graph = graph.Graph{
		Name: name,
	}

	t.Graph.First, err = t.newNoopNode(name + "_start")
	if err != nil {
		return err
	}

	t.Graph.Last, err = t.newNoopNode(name + "_end")
	if err != nil {
		return err
	}

	(*t.Graph.First.GetNext())[t.Graph.Last.GetId()] = t.Graph.Last
	(*t.Graph.Last.GetPrev())[t.Graph.First.GetId()] = t.Graph.First
	t.Graph.Vertices = map[string]graph.Node{
		t.Graph.First.GetId(): t.Graph.First,
		t.Graph.Last.GetId():  t.Graph.Last,
	}
	t.Graph.Edges = map[string][]string{t.Graph.First.GetId(): []string{t.Graph.Last.GetId()}}

	return nil
}

func (t *Graph) newNoopNode(name string) (graph.Node, error) {
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
