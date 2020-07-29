// Copyright 2020, Square, Inc.

package chain

import (
	"fmt"

	"github.com/square/spincycle/v2/job"
	"github.com/square/spincycle/v2/request-manager/graph"
)

type Node struct {
	// payload
	Job               job.Job                // runnable job that this graph node represents
	Name              string                 // name of node
	Args              map[string]interface{} // the args the node was created with
	Retry             uint                   // the number of times to retry a node
	RetryWait         string                 // the time to sleep between retries
	SequenceId        string                 // ID for first node in sequence
	SequenceRetry     uint                   // Number of times to retry a sequence. Only set for first node in sequence.
	SequenceRetryWait string                 // the time to sleep between sequence retries

	// fields required by graph.Node interface
	Next map[string]graph.Node
	Prev map[string]graph.Node
}

func (g *Node) GetId() string {
	return g.Job.Id().Id
}

func (g *Node) GetNext() *map[string]graph.Node {
	return &g.Next
}

func (g *Node) GetPrev() *map[string]graph.Node {
	return &g.Prev
}

func (g *Node) GetName() string {
	return g.Name
}

func (g *Node) String() string {
	var s string
	s += fmt.Sprintf("Sequence ID: %s\\n ", g.SequenceId)
	s += fmt.Sprintf("Sequence Retry: %v\\n ", g.SequenceRetry)
	for k, v := range g.Args {
		s += fmt.Sprintf(" %s : %s \\n ", k, v)
	}
	return s
}

type Graph struct {
	Graph graph.Graph // graph of Nodes
}

func (g *Graph) setSequenceRetryInfo(retry uint, wait string) {
	n, _ := g.Graph.First.(*Node)
	n.SequenceRetry = retry
	n.SequenceRetryWait = wait
}

// Cast Graph.Vertices to map of chain.Nodes
func (g *Graph) getVertices() map[string]*Node {
	m := map[string]*Node{}
	for jobId, graphNode := range g.Graph.Vertices {
		node, _ := graphNode.(*Node)
		m[jobId] = node
	}
	return m
}
