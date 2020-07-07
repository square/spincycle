// Copyright 2017-2018, Square, Inc.

package grapher

import (
	"fmt"

	"github.com/square/spincycle/v2/job"
)

// Graph represents a graph. It represents a graph via
// Vertices, a map of vertex name -> Node, and Edges, an
// adjacency list. Also contained in Graph are the First and
// Last Nodes in the graph.
type Graph struct {
	Name     string              // Name of the Graph
	First    *Node               // The source node of the graph
	Last     *Node               // The sink node of the graph
	Vertices map[string]*Node    // All vertices in the graph (node id -> node)
	Edges    map[string][]string // All edges (source node id -> sink node id)
}

// Node represents a single vertex within a Graph.
// Each node consists of a Payload (i.e. the data that the
// user cares about), a list of next and prev Nodes, and other
// information about the node such as the number of times it
// should be retried on error. Next defines all the out edges
// from Node, and Prev defines all the in edges to Node.
type Node struct {
	Datum             job.Job                // Data stored at this Node
	Next              map[string]*Node       // out edges ( node id -> Node )
	Prev              map[string]*Node       // in edges ( node id -> Node )
	Name              string                 // the name of the node
	Args              map[string]interface{} // the args the node was created with
	Retry             uint                   // the number of times to retry a node
	RetryWait         string                 // the time to sleep between retries
	SequenceId        string                 // ID for first node in sequence
	SequenceRetry     uint                   // Number of times to retry a sequence. Only set for first node in sequence.
	SequenceRetryWait string                 // the time to sleep between sequence retries
}

// returns true iff the graph has at least one cycle in it
func (g *Graph) HasCycles() bool {
	seen := map[string]*Node{g.First.Datum.Id().Id: g.First}
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
		if n, ok := g.Vertices[vertexName]; !ok || n != node {
			return false
		}
	}
	for vertexName, node := range g.Vertices {
		if n, ok := vertices[vertexName]; !ok || n != node {
			return false
		}
	}

	// Check that the edges all match as well
	for source, sinks := range edges {
		if e := g.Edges[source]; !slicesMatch(e, sinks) {
			return false
		}
	}
	for source, sinks := range g.Edges {
		if e := edges[source]; !slicesMatch(e, sinks) {
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
		fmt.Printf("\t\"%s\" [label=\"%s\\n ", vertexName, vertex.Name)
		fmt.Printf("Vertex ID: %s\\n ", vertex.Datum.Id().Id)
		fmt.Printf("Sequence ID: %s\\n ", vertex.SequenceId)
		fmt.Printf("Sequence Retry: %v\\n ", vertex.SequenceRetry)
		for k, v := range vertex.Args {
			fmt.Printf(" %s : %s \\n ", k, v)
		}
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
func (g *Graph) connectedToLastNodeDFS(n *Node) bool {
	if n == nil {
		return false
	}
	if g.Last == n {
		return true
	}
	if g.Last != n && (n.Next == nil || len(n.Next) == 0) {
		return false
	}
	for _, next := range n.Next {

		// Every node after n must also be connected to the last node
		connected := g.connectedToLastNodeDFS(next)
		if !connected {
			return false
		}
	}
	return true
}

// Returns true if n is reachable from the first node in g
func (g *Graph) connectedToFirstNodeDFS(n *Node) bool {
	if n == nil {
		return false
	}
	if g.First == n {
		return true
	}
	if g.First != n && (n.Prev == nil || len(n.Prev) == 0) {
		return false
	}
	for _, prev := range n.Prev {

		// Every node before n must also be connected to the first node
		connected := g.connectedToFirstNodeDFS(prev)
		if !connected {
			return false
		}
	}
	return true
}

// InsertComponentBetween will take a Graph as input, and insert it between the given prev and next nodes.
// Preconditions:
//      component and g are connected and acyclic
//      prev and next both are present in g
//      next "comes after" prev in the graph, when traversing from the source node
func (g *Graph) insertComponentBetween(component *Graph, prev *Node, next *Node) error {
	// Cannot check for the adjacency list match here because of the way we insert components.
	if g.HasCycles() || component.HasCycles() ||
		!g.IsConnected() || !component.IsConnected() {
		return fmt.Errorf("Graph not valid!")
	}

	// have component point to prev and next nodes
	component.First.Prev[prev.Datum.Id().Id] = prev
	component.Last.Next[next.Datum.Id().Id] = next

	// have prev and next nodes point to component
	prev.Next[component.First.Datum.Id().Id] = component.First
	next.Prev[component.Last.Datum.Id().Id] = component.Last

	// Remove edges between prev and next if it exists
	delete(prev.Next, next.Datum.Id().Id)
	delete(next.Prev, prev.Datum.Id().Id)

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
	g.Edges[prev.Datum.Id().Id] = append(g.Edges[prev.Datum.Id().Id], component.First.Datum.Id().Id)

	// for the edges of the last node of this component, add the next node
	if find(g.Edges[component.Last.Datum.Id().Id], next.Datum.Id().Id) < 0 {
		g.Edges[component.Last.Datum.Id().Id] = append(g.Edges[component.Last.Datum.Id().Id], next.Datum.Id().Id)
	}

	// Remove all occurences next from the adjacency list of prev
	i := find(g.Edges[prev.Datum.Id().Id], next.Datum.Id().Id)
	for i >= 0 {
		g.Edges[prev.Datum.Id().Id][i] = g.Edges[prev.Datum.Id().Id][len(g.Edges[prev.Datum.Id().Id])-1]
		g.Edges[prev.Datum.Id().Id] = g.Edges[prev.Datum.Id().Id][:len(g.Edges[prev.Datum.Id().Id])-1]
		i = find(g.Edges[prev.Datum.Id().Id], next.Datum.Id().Id)
	}

	// verify resulting graph is ok
	if g.HasCycles() || !g.IsConnected() {
		return fmt.Errorf("graph not valid after insert")
	}
	return nil
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
func (g *Graph) createAdjacencyList() (map[string][]string, map[string]*Node) {
	edges := map[string][]string{}
	vertices := []string{}
	s := edges

	n := g.First

	// Classic BFS
	seen := map[string]*Node{}
	frontier := map[string]*Node{}

	for name, node := range n.Next {
		frontier[name] = node
	}
	seen[n.Datum.Id().Id] = n

	// Search while the frontier set is non-empty
	for len(frontier) > 0 {

		// For every node in the frontier
		for name, next := range frontier {

			// Look at the edges connecting that node to a node in the seen set.
			for n, _ := range next.Prev {

				// If this edge has not been seen yet, add it to the edge list
				if _, ok := s[n]; !ok {
					s[n] = []string{}
				}
				s[n] = append(s[n], name)
			}

			// add each "next" node to the frontier set
			for k, v := range next.Next {
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
func hasCyclesDFS(seen map[string]*Node, start *Node) bool {
	for _, next := range start.Next {

		// If the next node has already been seen, return true
		if _, ok := seen[next.Datum.Id().Id]; ok {
			return true
		}

		// Add next node to seen list
		seen[next.Datum.Id().Id] = next

		// Continue searching after next node
		if hasCyclesDFS(seen, next) {
			return true
		}

		// Remove next node from seen list
		delete(seen, next.Datum.Id().Id)
	}
	return false
}

// Returns true if a matches b, regardless of ordering
func slicesMatch(a, b []string) bool {
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
