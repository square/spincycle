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
}

// Node represents a single vertex within a Graph.
// Implementations should be pointers in order for graph modification functions to
// work properly, i.e. GetNext() should be a map of strings -> pointers.
type Node interface {
	// functions involving graph functionality
	Id() string                // get node's unique id within graph
	GetNext() *map[string]Node // get out edges (node id -> Node)
	GetPrev() *map[string]Node // get in edges (node id -> Node)

	// pretty printer functions (for use with PrintDot)
	Name() string   // get node's name
	String() string // get implementation-specific attributes
}

// returns true iff the graph has at least one cycle in it
func (g *Graph) HasCycles() bool {
	seen := map[string]Node{g.First.Id(): g.First}
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
		if n, ok := g.Vertices[vertexName]; !ok || n.Id() != node.Id() {
			return false
		}
	}
	for vertexName, node := range g.Vertices {
		if n, ok := vertices[vertexName]; !ok || n.Id() != node.Id() {
			return false
		}
	}

	// Check that the edges all match as well
	for source, sinks := range edges {
		if e := g.Edges[source]; !SlicesMatch(e, sinks) {
			return false
		}
	}
	for source, sinks := range g.Edges {
		if e := edges[source]; !SlicesMatch(e, sinks) {
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

// InsertComponentBetween will take a Graph as input, and insert it between the given prev and next nodes.
// Preconditions:
//      component and g are connected and acyclic
//      prev and next both are present in g
//      next "comes after" prev in the graph, when traversing from the source node
func (g *Graph) InsertComponentBetween(component *Graph, prev Node, next Node) error {
	// Cannot check for the adjacency list match here because of the way we insert components.
	if g.HasCycles() || component.HasCycles() ||
		!g.IsConnected() || !component.IsConnected() {
		return fmt.Errorf("Graph not valid!")
	}

	// have component point to prev and next nodes
	(*component.First.GetPrev())[prev.Id()] = prev
	(*component.Last.GetNext())[next.Id()] = next

	// have prev and next nodes point to component
	(*prev.GetNext())[component.First.Id()] = component.First
	(*next.GetPrev())[component.Last.Id()] = component.Last

	// Remove edges between prev and next if it exists
	delete(*prev.GetNext(), next.Id())
	delete(*next.GetPrev(), prev.Id())

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
	g.Edges[prev.Id()] = append(g.Edges[prev.Id()], component.First.Id())

	// for the edges of the last node of this component, add the next node
	if find(g.Edges[component.Last.Id()], next.Id()) < 0 {
		g.Edges[component.Last.Id()] = append(g.Edges[component.Last.Id()], next.Id())
	}

	// Remove all occurences next from the adjacency list of prev
	i := find(g.Edges[prev.Id()], next.Id())
	for i >= 0 {
		g.Edges[prev.Id()][i] = g.Edges[prev.Id()][len(g.Edges[prev.Id()])-1]
		g.Edges[prev.Id()] = g.Edges[prev.Id()][:len(g.Edges[prev.Id()])-1]
		i = find(g.Edges[prev.Id()], next.Id())
	}

	// verify resulting graph is ok
	if g.HasCycles() || !g.IsConnected() {
		return fmt.Errorf("graph not valid after insert")
	}
	return nil
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
		fmt.Printf("%v\n", vertex)
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

// Returns true if the last node in g is reachable from n
func (g *Graph) connectedToLastNodeDFS(n Node) bool {
	if n == nil {
		return false
	}
	if g.Last.Id() == n.Id() {
		return true
	}
	if g.Last.Id() != n.Id() && (n.GetNext() == nil || len(*n.GetNext()) == 0) {
		return false
	}
	for _, next := range *n.GetNext() {

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
	if g.First.Id() != n.Id() && (n.GetPrev() == nil || len(*n.GetPrev()) == 0) {
		return false
	}
	for _, prev := range *n.GetPrev() {

		// Every node before n must also be connected to the first node
		connected := g.connectedToFirstNodeDFS(prev)
		if !connected {
			return false
		}
	}
	return true
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
func (g *Graph) createAdjacencyList() (map[string][]string, map[string]Node) {
	edges := map[string][]string{}
	vertices := []string{}
	s := edges

	n := g.First

	// Classic BFS
	seen := map[string]Node{}
	frontier := map[string]Node{}

	for name, node := range *n.GetNext() {
		frontier[name] = node
	}
	seen[n.Id()] = n

	// Search while the frontier set is non-empty
	for len(frontier) > 0 {

		// For every node in the frontier
		for name, next := range frontier {

			// Look at the edges connecting that node to a node in the seen set.
			for n, _ := range *next.GetPrev() {

				// If this edge has not been seen yet, add it to the edge list
				if _, ok := s[n]; !ok {
					s[n] = []string{}
				}
				s[n] = append(s[n], name)
			}

			// add each "next" node to the frontier set
			for k, v := range *next.GetNext() {
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

// Determines if a graph has cycles, using dfs
// precondition: start node is already in seen list
func hasCyclesDFS(seen map[string]Node, start Node) bool {
	for _, next := range *start.GetNext() {

		// If the next node has already been seen, return true
		if _, ok := seen[next.Id()]; ok {
			return true
		}

		// Add next node to seen list
		seen[next.Id()] = next

		// Continue searching after next node
		if hasCyclesDFS(seen, next) {
			return true
		}

		// Remove next node from seen list
		delete(seen, next.Id())
	}
	return false
}
