// Copyright 2020, Square, Inc.

package template

import (
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

type TemplateNode struct {
	// payload
	NodeSpec *spec.NodeSpec // sequence node that this graph node represents

	// fields required by graph.Node interface
	Id   string
	Next map[string]graph.Node
	Prev map[string]graph.Node
}

func (g TemplateNode) GetId() string {
	return g.Id
}

func (g TemplateNode) GetNext() *map[string]graph.Node {
	return &g.Next
}

func (g TemplateNode) GetPrev() *map[string]graph.Node {
	return &g.Prev
}

func (g TemplateNode) GetName() string {
	return g.NodeSpec.Name
}

// graph prety printer function already prints relevant info; there's nothing to add here
func (g TemplateNode) String() string {
	return ""
}

type TemplateGraph struct {
	Graph graph.Graph     // graph of TemplateNodes
	Sets  map[string]bool // set of arguments set by sequence described by graph

	idgen id.Generator // generates UIDs for this graph
}

func NewTemplateGraph(name string, idgen id.Generator) (*TemplateGraph, error) {
	t := &TemplateGraph{
		Sets:  map[string]bool{},
		idgen: idgen,
	}
	err := t.initGraph(name)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TemplateGraph) NewSubgraph(node *spec.NodeSpec) (*graph.Graph, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	n := TemplateNode{
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

func (t *TemplateGraph) initGraph(name string) error {
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

func (t *TemplateGraph) newNoopNode(name string) (graph.Node, error) {
	id, err := t.idgen.UID()
	if err != nil {
		return nil, err
	}
	noopCategory := "job"
	noopType := "noop"
	return &TemplateNode{
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
