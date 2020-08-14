// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"
)

// Represents a graph via Vertices, a map of vertex name -> Node, and Edges, an
// adjacency list. Also contains the First and Last Nodes in the graph.
type Graph struct {
	Name     string              // Name of the Graph
	First    Node                // The source node of the graph
	Last     Node                // The sink node of the graph
	Vertices map[string]Node     // All vertices in the graph (node id -> node)
	Edges    map[string][]string // All edges (source node id -> sink node id)
	RevEdges map[string][]string // All reverse edges (sink node id -> source no id)
}

// Node represents a single vertex within a Graph.
type Node interface {
	// functions involving graph functionality
	Id() string // get node's unique id within graph

	// pretty printer functions (for use with PrintDot)
	Name() string   // get node's name
	String() string // get implementation-specific attributes
}

// Asserts that g is a valid graph (according to Grapher's use case).
// Ensures that g is acyclic, is connected (not fully connected),
// and edge map matches reverse edge map.
func (g *Graph) IsValidGraph() bool {
	return !g.hasCycles() && g.isConnected() && g.edgesMatchesRevEdges()
}

// Get out edges of node (node id --> Node)
func (g *Graph) GetNext(n Node) map[string]Node {
	next := map[string]Node{}
	for _, nextId := range g.Edges[n.Id()] {
		next[nextId] = g.Vertices[nextId]
	}
	return next
}

// Get in edges of node (node id --> Node)
func (g *Graph) GetPrev(n Node) map[string]Node {
	prev := map[string]Node{}
	for _, prevId := range g.RevEdges[n.Id()] {
		prev[prevId] = g.Vertices[prevId]
	}
	return prev
}

// InsertComponentBetween will take a Graph as input, and insert it between the given prev and next nodes.
// Preconditions:
//      component and g are connected and acyclic
//      prev and next both are present in g
//      next "comes after" prev in the graph, when traversing from the source node
func (g *Graph) InsertComponentBetween(component *Graph, prev Node, next Node) error {
	if !g.IsValidGraph() || !component.IsValidGraph() {
		return fmt.Errorf("Graph not valid!")
	}

	var source, sink string

	// Add in the new vertices
	for k, v := range component.Vertices {
		g.Vertices[k] = v
	}

	// update adjacency lists
	for k, v := range component.Edges {
		g.Edges[k] = v
	}
	for k, v := range component.RevEdges {
		g.RevEdges[k] = v
	}

	// connect previous node and start node of component
	source = prev.Id()
	sink = component.First.Id()
	if find(g.Edges[source], sink) < 0 {
		g.Edges[source] = append(g.Edges[source], sink)
		g.RevEdges[sink] = append(g.RevEdges[sink], source)
	}

	// connect last node of component and next node
	source = component.Last.Id()
	sink = next.Id()
	if find(g.Edges[source], sink) < 0 {
		g.Edges[source] = append(g.Edges[source], sink)
		g.RevEdges[sink] = append(g.RevEdges[sink], source)
	}

	// remove all connections between prev and next
	source = prev.Id()
	sink = next.Id()
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

	// verify resulting graph is ok
	if !g.IsValidGraph() {
		return fmt.Errorf("graph not valid after insert")
	}
	return nil
}

// Prints out g in DOT graph format.
// Copy and paste output into http://www.webgraphviz.com/
func (g *Graph) PrintDot() {
	fmt.Printf("digraph {\n")
	fmt.Printf("\trankdir=UD;\n")
	fmt.Printf("\tlabelloc=\"t\";\n")
	fmt.Printf("\tlabel=\"%s\"\n", g.Name)
	fmt.Printf("\tfontsize=22\n")
	for vertexName, vertex := range g.Vertices {
		fmt.Printf("\tnode [style=filled,color=\"%s\",shape=box]\n", "#86cedf")
		fmt.Printf("\t\"%s\" [label=\"%s\\n ", vertexName, vertex.Name())
		fmt.Printf("Vertex ID: %s\\n ", vertex.Id())
		fmt.Printf("%v\\n", vertex)
		fmt.Printf("\"]\n")
	}
	for out, ins := range g.Edges {
		for _, in := range ins {
			fmt.Printf("\t\"%s\" -> \"%s\";\n", out, in)
		}
	}
	fmt.Println("}")
}

// Returns true if a matches b, regardless of ordering
func SlicesMatch(a, b []string) bool {
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

// --------------------------------------------------------------------------

// returns true iff the graph has at least one cycle in it
func (g *Graph) hasCycles() bool {
	seen := map[string]Node{g.First.Id(): g.First}
	return g.hasCyclesDFS(seen, g.First)
}

// returns true iff every node is reachable from the start node, and every path
// terminates at the end node
func (g *Graph) isConnected() bool {
	// Check forwards connectivity and backwards connectivity
	return g.connectedToLastNodeDFS(g.First) && g.connectedToFirstNodeDFS(g.Last)
}

// Returns true if the last node in g is reachable from n
func (g *Graph) connectedToLastNodeDFS(n Node) bool {
	if n == nil {
		return false
	}
	if g.Last.Id() == n.Id() {
		return true
	}
	if g.Last.Id() != n.Id() && len(g.GetNext(n)) == 0 {
		return false
	}
	for _, next := range g.GetNext(n) {

		// Every node after n must also be connected to the last node
		connected := g.connectedToLastNodeDFS(next)
		if !connected {
			return false
		}
	}
	return true
}

// Returns true if n is reachable from the first node in g
func (g *Graph) connectedToFirstNodeDFS(n Node) bool {
	if n == nil {
		return false
	}
	if g.First.Id() == n.Id() {
		return true
	}
	if g.First.Id() != n.Id() && len(g.GetPrev(n)) == 0 {
		return false
	}
	for _, prev := range g.GetPrev(n) {

		// Every node before n must also be connected to the first node
		connected := g.connectedToFirstNodeDFS(prev)
		if !connected {
			return false
		}
	}
	return true
}

// Returns true iff `Edges` represents exactly the same set of edges as `RevEdges`.
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

// Determines if a graph has cycles, using dfs
// precondition: start node is already in seen list
func (g *Graph) hasCyclesDFS(seen map[string]Node, start Node) bool {
	for _, next := range g.GetNext(start) {

		// If the next node has already been seen, return true
		if _, ok := seen[next.Id()]; ok {
			return true
		}

		// Add next node to seen list
		seen[next.Id()] = next

		// Continue searching after next node
		if g.hasCyclesDFS(seen, next) {
			return true
		}

		// Remove next node from seen list
		delete(seen, next.Id())
	}
	return false
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
