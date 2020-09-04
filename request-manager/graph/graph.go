// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"

	"github.com/square/spincycle/v2/request-manager/spec"
)

// Graph is a DAG where nodes are node specs (if a sequence graph) or jobs (if a
// request graph), and a directed edge (u, v) exists iff u is a dependency of v.
// It has one source and one sink node.
type Graph struct {
	Name   string // Name of the graph
	Source *Node  // The single source node of the graph
	Sink   *Node  // The single sink node of the graph

	// Nodes are keyed on node id, which is a string
	Nodes    map[string]*Node    // All nodes
	Edges    map[string][]string // All edges
	RevEdges map[string][]string // All reverse edges (sink -> source)

	Order []*Node // Topological ordering of nodes (only used in sequence graphs)
}

// Node represents a node spec (if a sequence graph) or a job (if a request
// graph). Validity of fields depends on use case.
type Node struct {
	// Always valid
	Id   string // UID within graph
	Name string // Descriptive name for debugging and visualizing graph

	// Used when node represents a node spec
	Spec *spec.Node // Node spec that this graph node represents

	// Used when node represents a job
	JobBytes          []byte                 // return value of Job.Serialize method
	Args              map[string]interface{} // The args the node was created with
	Retry             uint                   // The number of times to retry a node
	RetryWait         string                 // The time to sleep between retries
	SequenceId        string                 // ID for first node in sequence
	SequenceRetry     uint                   // Number of times to retry a sequence. Only set for first node in sequence.
	SequenceRetryWait string                 // The time to sleep between sequence retries
}

// IsValidGraph asserts that g is a valid graph by ensuring that
// g is acyclic and connected (not fully connected), and that the edge
// map matches the reverse edge map.
func (g *Graph) IsValidGraph() error {
	if g.hasCycles() {
		return fmt.Errorf("cycles found in graph")
	}
	if !g.isConnected() {
		return fmt.Errorf("graph not connected")
	}
	if !g.edgesMatchesRevEdges() {
		return fmt.Errorf("adjacency list does not match reverse adjacency list")
	}
	return nil
}

// GetNext returns the out edges of a node (node id --> Node)
func (g *Graph) GetNext(n *Node) []*Node {
	next := make([]*Node, len(g.Edges[n.Id]))
	for i, nextId := range g.Edges[n.Id] {
		next[i] = g.Nodes[nextId]
	}
	return next
}

// GetPrev returns the in edges of a node (node id --> Node)
func (g *Graph) GetPrev(n *Node) []*Node {
	prev := make([]*Node, len(g.RevEdges[n.Id]))
	for i, prevId := range g.RevEdges[n.Id] {
		prev[i] = g.Nodes[prevId]
	}
	return prev
}

// InsertComponentBetween takes a Graph as input and inserts it between the given
// prev and next nodes.
// Preconditions:
//      component and g are connected and acyclic
//      prev and next both are present in g
//      next "comes after" prev in the graph, when traversing from the source node
func (g *Graph) InsertComponentBetween(component *Graph, prev *Node, next *Node) error {
	if err := g.IsValidGraph(); err != nil {
		return fmt.Errorf("graph to insert component into: %s", err)
	}
	if err := component.IsValidGraph(); err != nil {
		return fmt.Errorf("component to be inserted: %s", err)
	}

	var source, sink string

	// Add in the new vertices
	for k, v := range component.Nodes {
		g.Nodes[k] = v
	}

	// Update adjacency lists
	for k, v := range component.Edges {
		g.Edges[k] = v
	}
	for k, v := range component.RevEdges {
		g.RevEdges[k] = v
	}

	// Connect previous node and start node of component
	source = prev.Id
	sink = component.Source.Id
	if find(g.Edges[source], sink) < 0 {
		g.Edges[source] = append(g.Edges[source], sink)
		g.RevEdges[sink] = append(g.RevEdges[sink], source)
	}

	// Connect last node of component and next node
	source = component.Sink.Id
	sink = next.Id
	if find(g.Edges[source], sink) < 0 {
		g.Edges[source] = append(g.Edges[source], sink)
		g.RevEdges[sink] = append(g.RevEdges[sink], source)
	}

	// Remove all connections between prev and next
	source = prev.Id
	sink = next.Id
	i := find(g.Edges[source], sink)
	for i >= 0 {
		g.Edges[source][i] = g.Edges[source][len(g.Edges[source])-1]
		g.Edges[source] = g.Edges[source][:len(g.Edges[source])-1]
		i = find(g.Edges[source], sink)
	}
	i = find(g.RevEdges[sink], source)
	for i >= 0 {
		g.RevEdges[sink][i] = g.RevEdges[sink][len(g.RevEdges[sink])-1]
		g.RevEdges[sink] = g.RevEdges[sink][:len(g.RevEdges[sink])-1]
		i = find(g.Edges[sink], source)
	}

	// Verify resulting graph is ok
	if err := g.IsValidGraph(); err != nil {
		return fmt.Errorf("graph not valid after insert: %s", err)
	}
	return nil
}

// PrintDot prints out g in DOT graph format.
// Copy and paste output into http://www.webgraphviz.com/
// It's not used anywhere in the code, but it is a very useful debugging tool.
func (g *Graph) PrintDot() {
	fmt.Printf("digraph {\n")
	fmt.Printf("\trankdir=UD;\n")
	fmt.Printf("\tlabelloc=\"t\";\n")
	fmt.Printf("\tlabel=\"%s\"\n", g.Name)
	fmt.Printf("\tfontsize=22\n")
	for vertexName, vertex := range g.Nodes {
		fmt.Printf("\tnode [style=filled,color=\"%s\",shape=box]\n", "#86cedf")
		fmt.Printf("\t\"%s\" [label=\"%s\\n ", vertexName, vertex.Name)
		fmt.Printf("Vertex ID: %s\\n ", vertex.Id)
		fmt.Printf("\"]\n")
	}
	for out, ins := range g.Edges {
		for _, in := range ins {
			fmt.Printf("\t\"%s\" -> \"%s\";\n", out, in)
		}
	}
	fmt.Println("}")
}

// --------------------------------------------------------------------------

// hasCycles returns true iff the graph has at least one cycle in it.
func (g *Graph) hasCycles() bool {
	seen := map[string]*Node{g.Source.Id: g.Source}
	return g.hasCyclesHelper(seen, g.Source)
}

// isConnected returns true iff every node is reachable from the start node, and
// every path terminates at the end node.
func (g *Graph) isConnected() bool {
	// Check forwards connectivity and backwards connectivity
	return g.connectedToLastNode(g.Source) && g.connectedToFirstNode(g.Sink)
}

// connectedToLastNode returns true iff the last node in g is reachable from n.
func (g *Graph) connectedToLastNode(n *Node) bool {
	if n == nil {
		return false
	}
	if g.Sink.Id == n.Id {
		return true
	}
	if g.Sink.Id != n.Id && len(g.GetNext(n)) == 0 {
		return false
	}
	for _, next := range g.GetNext(n) {

		// Every node after n must also be connected to the last node
		connected := g.connectedToLastNode(next)
		if !connected {
			return false
		}
	}
	return true
}

// connectedToFirstNode returns true if n is reachable from the first node in g.
func (g *Graph) connectedToFirstNode(n *Node) bool {
	if n == nil {
		return false
	}
	if g.Source.Id == n.Id {
		return true
	}
	if g.Source.Id != n.Id && len(g.GetPrev(n)) == 0 {
		return false
	}
	for _, prev := range g.GetPrev(n) {

		// Every node before n must also be connected to the first node
		connected := g.connectedToFirstNode(prev)
		if !connected {
			return false
		}
	}
	return true
}

// edgesMatchesRevEdges returns true iff `Edges` represents exactly the same set
// of edges as `RevEdges`.
func (g *Graph) edgesMatchesRevEdges() bool {
	for source, sinks := range g.Edges {
		for _, sink := range sinks {
			revSources, ok := g.RevEdges[sink]
			if !ok || find(revSources, source) < 0 {
				return false
			}
		}
	}
	for revSink, revSources := range g.RevEdges {
		for _, revSource := range revSources {
			sinks, ok := g.Edges[revSource]
			if !ok || find(sinks, revSink) < 0 {
				return false
			}
		}
	}
	return true
}

// hasCyclesHelper determines if a graph has cycles, using DFS.
// Precondition: start node is already in seen list.
func (g *Graph) hasCyclesHelper(seen map[string]*Node, start *Node) bool {
	for _, next := range g.GetNext(start) {

		// If the next node has already been seen, return true
		if _, ok := seen[next.Id]; ok {
			return true
		}

		// Add next node to seen list
		seen[next.Id] = next

		// Continue searching after next node
		if g.hasCyclesHelper(seen, next) {
			return true
		}

		// Remove next node from seen list
		delete(seen, next.Id)
	}
	return false
}

// find returns the index of s in ss, returns -1 if s is not found in ss.
func find(ss []string, s string) int {
	for i, j := range ss {
		if j == s {
			return i
		}
	}
	return -1
}
