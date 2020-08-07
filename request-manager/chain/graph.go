// Copyright 2020, Square, Inc.

package chain

import (
	"fmt"

	"github.com/square/spincycle/v2/job"
	"github.com/square/spincycle/v2/request-manager/graph"
)

// Node in job graph. Implements graph.Node.
type Node struct {
	// payload
	Job               job.Job                // runnable job that this graph node represents
	NodeName          string                 // name of node
	Args              map[string]interface{} // the args the node was created with
	Retry             uint                   // the number of times to retry a node
	RetryWait         string                 // the time to sleep between retries
	SequenceId        string                 // ID for first node in sequence
	SequenceRetry     uint                   // Number of times to retry a sequence. Only set for first node in sequence.
	SequenceRetryWait string                 // the time to sleep between sequence retries
}

func (g *Node) Id() string {
	return g.Job.Id().Id
}

func (g *Node) Name() string {
	return g.NodeName
}

func (g *Node) String() string {
	var s string
	s += fmt.Sprintf("Sequence ID: %s\\n ", g.SequenceId)
	s += fmt.Sprintf("Sequence Retry: %v ", g.SequenceRetry)
	for k, v := range g.Args {
		s += fmt.Sprintf("\\n %s : %s ", k, v)
	}
	return s
}

// Graph of actual jobs. Most useful functions are implemented in creator.go. These are just some
// convenience functions so that callers don't have to do typecasts.
// Functions assume that all vertices are in fact 'chain.Node's. There is no error checking--it just panics.
type Graph struct {
	Graph graph.Graph
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
