// This is currently just a stub.
package grapher

// Graph represents a graph. It represents a graph via
// Vertices, a map of vertex name -> Node, and Edges, an
// adjacency list. Also contained in Graph are the First and
// Last Nodes in the graph.
type Graph struct {
	Name     string              // Name of the Graph
	First    *Node               // The source node of the graph
	Last     *Node               // The sink node of the graph
	Vertices map[string]*Node    // All vertices in the graph (node name -> node)
	Edges    map[string][]string // All edges (source node name -> sink node name)
}

// Node represents a single vertex within a Graph.
// Each node consists of a Payload (i.e. the data that the
// user cares about), and a list of next and prev Nodes.
// Next defines all the out edges from Node, and Prev
// defines all the in edges to Node.
type Node struct {
	Datum Payload          // Data stored at this Node
	Next  map[string]*Node // out edges ( node name -> Node )
	Prev  map[string]*Node // in edges ( node name -> Node )
}

// Payload defines the interface of structs that can be
// stored within Graphs. The Create() function will
// be used by Grapher to actually create the Payload.
type Payload interface {
	Create(map[string]interface{}) error
	Serialize() ([]byte, error)
	Type() string
	Name() string
}
