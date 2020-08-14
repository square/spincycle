// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

type mockNode struct {
	NodeId string
	Next   map[string]Node
	Prev   map[string]Node
}

func (n *mockNode) Id() string {
	return n.NodeId
}

func (n *mockNode) Name() string {
	return n.NodeId
}

func (n *mockNode) String() string {
	return n.NodeId
}

func newMockNode(id string) *mockNode {
	return &mockNode{
		NodeId: id,
		Next:   map[string]Node{},
		Prev:   map[string]Node{},
	}
}

// three nodes in a straight line
func g1() *Graph {
	g1n1 := newMockNode("g1n1")
	g1n2 := newMockNode("g1n2")
	g1n3 := newMockNode("g1n3")

	g1n1.Prev = map[string]Node{}
	g1n1.Next = map[string]Node{"g1n2": g1n2}
	g1n2.Next = map[string]Node{"g1n3": g1n3}
	g1n2.Prev = map[string]Node{"g1n1": g1n1}
	g1n3.Prev = map[string]Node{"g1n2": g1n2}
	g1n3.Next = map[string]Node{}

	return &Graph{
		Name:  "test1",
		First: g1n1,
		Last:  g1n3,
		Vertices: map[string]Node{
			"g1n1": g1n1,
			"g1n2": g1n2,
			"g1n3": g1n3,
		},
		Edges: map[string][]string{
			"g1n1": []string{"g1n2"},
			"g1n2": []string{"g1n3"},
		},
		RevEdges: map[string][]string{
			"g1n2": []string{"g1n1"},
			"g1n3": []string{"g1n2"},
		},
	}
}

// 18 node graph of something
func g2() *Graph {
	n := [20]*mockNode{}
	for i := 0; i < 20; i++ {
		m := fmt.Sprintf("g2n%d", i)
		n[i] = newMockNode(m)
	}

	// yeah good luck following this
	n[0].Next["g2n1"] = n[1]
	n[1].Next["g2n2"] = n[2]
	n[1].Next["g2n5"] = n[5]
	n[1].Next["g2n6"] = n[6]
	n[2].Next["g2n3"] = n[3]
	n[2].Next["g2n4"] = n[4]
	n[3].Next["g2n7"] = n[7]
	n[4].Next["g2n8"] = n[8]
	n[7].Next["g2n8"] = n[8]
	n[8].Next["g2n13"] = n[13]
	n[13].Next["g2n14"] = n[14]
	n[5].Next["g2n9"] = n[9]
	n[9].Next["g2n12"] = n[12]
	n[12].Next["g2n14"] = n[14]
	n[6].Next["g2n10"] = n[10]
	n[10].Next["g2n11"] = n[11]
	n[10].Next["g2n19"] = n[19]
	n[11].Next["g2n12"] = n[12]
	n[19].Next["g2n16"] = n[16]
	n[16].Next["g2n15"] = n[15]
	n[16].Next["g2n17"] = n[17]
	n[15].Next["g2n18"] = n[18]
	n[17].Next["g2n18"] = n[18]
	n[18].Next["g2n14"] = n[14]
	n[1].Prev["g2n0"] = n[0]
	n[2].Prev["g2n1"] = n[1]
	n[3].Prev["g2n2"] = n[2]
	n[4].Prev["g2n2"] = n[2]
	n[5].Prev["g2n1"] = n[1]
	n[6].Prev["g2n1"] = n[1]
	n[7].Prev["g2n3"] = n[3]
	n[8].Prev["g2n4"] = n[4]
	n[8].Prev["g2n7"] = n[7]
	n[9].Prev["g2n5"] = n[5]
	n[10].Prev["g2n6"] = n[6]
	n[11].Prev["g2n10"] = n[10]
	n[12].Prev["g2n9"] = n[9]
	n[12].Prev["g2n11"] = n[11]
	n[13].Prev["g2n8"] = n[8]
	n[14].Prev["g2n12"] = n[12]
	n[14].Prev["g2n13"] = n[13]
	n[14].Prev["g2n18"] = n[18]
	n[15].Prev["g2n16"] = n[16]
	n[16].Prev["g2n19"] = n[19]
	n[17].Prev["g2n16"] = n[16]
	n[18].Prev["g2n15"] = n[15]
	n[18].Prev["g2n17"] = n[17]
	n[19].Prev["g2n10"] = n[10]

	return &Graph{
		Name:  "g2",
		First: n[0],
		Last:  n[14],
		Vertices: map[string]Node{
			"g2n0":  n[0],
			"g2n1":  n[1],
			"g2n2":  n[2],
			"g2n3":  n[3],
			"g2n4":  n[4],
			"g2n5":  n[5],
			"g2n6":  n[6],
			"g2n7":  n[7],
			"g2n8":  n[8],
			"g2n9":  n[9],
			"g2n10": n[10],
			"g2n11": n[11],
			"g2n12": n[12],
			"g2n13": n[13],
			"g2n14": n[14],
			"g2n15": n[15],
			"g2n16": n[16],
			"g2n17": n[17],
			"g2n18": n[18],
			"g2n19": n[19],
		},
		Edges: map[string][]string{
			"g2n0":  []string{"g2n1"},
			"g2n1":  []string{"g2n2", "g2n5", "g2n6"},
			"g2n2":  []string{"g2n3", "g2n4"},
			"g2n3":  []string{"g2n7"},
			"g2n4":  []string{"g2n8"},
			"g2n5":  []string{"g2n9"},
			"g2n6":  []string{"g2n10"},
			"g2n7":  []string{"g2n8"},
			"g2n8":  []string{"g2n13"},
			"g2n9":  []string{"g2n12"},
			"g2n10": []string{"g2n11", "g2n19"},
			"g2n11": []string{"g2n12"},
			"g2n12": []string{"g2n14"},
			"g2n13": []string{"g2n14"},
			"g2n15": []string{"g2n18"},
			"g2n16": []string{"g2n15", "g2n17"},
			"g2n17": []string{"g2n18"},
			"g2n18": []string{"g2n14"},
			"g2n19": []string{"g2n16"},
		},
		RevEdges: map[string][]string{
			"g2n1":  []string{"g2n0"},
			"g2n2":  []string{"g2n1"},
			"g2n3":  []string{"g2n2"},
			"g2n4":  []string{"g2n2"},
			"g2n5":  []string{"g2n1"},
			"g2n6":  []string{"g2n1"},
			"g2n7":  []string{"g2n3"},
			"g2n8":  []string{"g2n4", "g2n7"},
			"g2n9":  []string{"g2n5"},
			"g2n10": []string{"g2n6"},
			"g2n11": []string{"g2n10"},
			"g2n12": []string{"g2n9", "g2n11"},
			"g2n13": []string{"g2n8"},
			"g2n14": []string{"g2n12", "g2n13", "g2n18"},
			"g2n15": []string{"g2n16"},
			"g2n16": []string{"g2n19"},
			"g2n17": []string{"g2n16"},
			"g2n18": []string{"g2n15", "g2n17"},
			"g2n19": []string{"g2n10"},
		},
	}
}

// 4 node diamond graph
//       2
//     /   \
// -> 1      4 ->
//     \   /
//       3
//
//
func g3() *Graph {
	g3n1 := newMockNode("g3n1")
	g3n2 := newMockNode("g3n2")
	g3n3 := newMockNode("g3n3")
	g3n4 := newMockNode("g3n4")

	g3n1.Prev = map[string]Node{}
	g3n1.Next = map[string]Node{"g3n2": g3n2, "g3n3": g3n3}
	g3n2.Next = map[string]Node{"g3n4": g3n4}
	g3n3.Next = map[string]Node{"g3n4": g3n4}
	g3n4.Prev = map[string]Node{"g3n2": g3n2, "g3n3": g3n3}
	g3n2.Prev = map[string]Node{"g3n1": g3n1}
	g3n3.Prev = map[string]Node{"g3n1": g3n1}
	g3n4.Next = map[string]Node{}

	return &Graph{
		Name:  "test1",
		First: g3n1,
		Last:  g3n4,
		Vertices: map[string]Node{
			"g3n1": g3n1,
			"g3n2": g3n2,
			"g3n3": g3n3,
			"g3n4": g3n4,
		},
		Edges: map[string][]string{
			"g3n1": []string{"g3n2", "g3n3"},
			"g3n2": []string{"g3n4"},
			"g3n3": []string{"g3n4"},
		},
		RevEdges: map[string][]string{
			"g3n2": []string{"g3n1"},
			"g3n3": []string{"g3n1"},
			"g3n4": []string{"g3n2", "g3n3"},
		},
	}
}

func TestFailHasCycles(t *testing.T) {
	g2 := g2()
	if g2.hasCycles() {
		t.Fatalf("g2.hasCycles() = true, expected false")
	}
}

func TestIsConnected(t *testing.T) {
	g2 := g2()
	if !g2.isConnected() {
		t.Fatalf("g2.isConnected() = false, expected true")
	}
}

func TestEdgesMatchesRevEdges(t *testing.T) {
	g2 := g2()
	if !g2.edgesMatchesRevEdges() {
		t.Fatalf("g2.edgesMatchesRevEdges() = false, expected true")
	}
}

func TestHasCycles(t *testing.T) {
	g0n1 := newMockNode("g0n1")
	g0n2 := newMockNode("g0n2")
	g0n3 := newMockNode("g0n3")
	g0n4 := newMockNode("g0n4")

	// .---- 3
	// |     ^
	// V     |
	// 1 --> 2 --> 4
	//
	//
	g0 := &Graph{
		Name:  "g0",
		First: g0n1,
		Last:  g0n4,
		Vertices: map[string]Node{
			"g0n1": g0n1,
			"g0n2": g0n2,
			"g0n3": g0n3,
			"g0n4": g0n4,
		},
		Edges: map[string][]string{
			"g0n1": []string{"g0n2"},
			"g0n2": []string{"g0n3", "g0n4"},
			"g0n3": []string{"g0n1"},
		},
		RevEdges: map[string][]string{
			"g0n1": []string{"g0n3"},
			"g0n2": []string{"g0n1"},
			"g0n3": []string{"g0n2"},
			"g0n4": []string{"g0n2"},
		},
	}

	if !g0.hasCycles() {
		t.Fatalf("g0.hasCycles() = false, expected true")
	}
}

func TestFailIsConnected(t *testing.T) {
	g0n1 := newMockNode("g0n1")
	g0n2 := newMockNode("g0n2")
	g0n3 := newMockNode("g0n3")

	//    -> 3
	//   /
	// 1 --> 2 (Last)
	g0 := &Graph{
		Name:  "g0",
		First: g0n1,
		Last:  g0n2,
		Vertices: map[string]Node{
			"g0n1": g0n1,
			"g0n2": g0n2,
			"g0n3": g0n3,
		},
		Edges: map[string][]string{
			"g0n1": []string{"g0n2", "g0n3"},
		},
		RevEdges: map[string][]string{
			"g0n2": []string{"g0n1"},
			"g0n3": []string{"g0n1"},
		},
	}

	if g0.isConnected() {
		t.Fatalf("g0.isConnected() = true, expected false")
	}
}

func TestFailEdgesMatchesRevEdges(t *testing.T) {
	g2 := g2()
	g2.RevEdges = map[string][]string{}

	if g2.edgesMatchesRevEdges() {
		t.Fatalf("g0.edgesMatchesRevEdges() = true, expected false")
	}
}

func testGetNext(t *testing.T, g *Graph) {
	for _, n := range g.Vertices {
		node, ok := n.(*mockNode)
		if !ok {
			t.Fatalf("Expected *mockNode, got %T", n)
		}

		next := g.GetNext(node)
		actualNext := node.Next
		if diff := deep.Equal(next, actualNext); diff != nil {
			t.Error(diff)
		}
	}
}

func TestGetNext1(t *testing.T) {
	g := g1()
	testGetNext(t, g)
}

func TestGetNext2(t *testing.T) {
	g := g2()
	testGetNext(t, g)
}

func TestGetNext3(t *testing.T) {
	g := g3()
	testGetNext(t, g)
}

func testGetPrev(t *testing.T, g *Graph) {
	for _, n := range g.Vertices {
		node, ok := n.(*mockNode)
		if !ok {
			t.Fatalf("Expected *mockNode, got %T", n)
		}

		prev := g.GetPrev(node)
		actualPrev := node.Prev
		if diff := deep.Equal(prev, actualPrev); diff != nil {
			t.Error(diff)
		}
	}
}

func TestGetPrev1(t *testing.T) {
	g := g1()
	testGetPrev(t, g)
}

func TestGetPrev2(t *testing.T) {
	g := g2()
	testGetPrev(t, g)
}

func TestGetPrev3(t *testing.T) {
	g := g3()
	testGetPrev(t, g)
}

func TestInsertComponentBetween1(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 2 -> 4
	err := g3.InsertComponentBetween(g1, g3.Vertices["g3n2"], g3.Vertices["g3n4"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]Node{
		"g1n1": g1.Vertices["g1n1"],
		"g1n2": g1.Vertices["g1n2"],
		"g1n3": g1.Vertices["g1n3"],
		"g3n1": g3.Vertices["g3n1"],
		"g3n2": g3.Vertices["g3n2"],
		"g3n3": g3.Vertices["g3n3"],
		"g3n4": g3.Vertices["g3n4"],
	}
	expectedEdges := map[string][]string{
		"g1n1": []string{"g1n2"},
		"g1n2": []string{"g1n3"},
		"g1n3": []string{"g3n4"},

		"g3n1": []string{"g3n2", "g3n3"},
		"g3n2": []string{"g1n1"},
		"g3n3": []string{"g3n4"},
	}

	actualEdges := g3.Edges
	actualVertices := g3.Vertices
	for vertexName, node := range actualVertices {
		if n, ok := expectedVertices[vertexName]; !ok || n != node {
			t.Fatalf("missing1 %v", n)
		}
	}
	for vertexName, node := range expectedVertices {
		if n, ok := actualVertices[vertexName]; !ok || n != node {
			t.Fatalf("missing2 %v", vertexName)
		}
	}
	for source, sinks := range actualEdges {
		if e := expectedEdges[source]; !SlicesMatch(e, sinks) {
			t.Fatalf("missing3 %s -> %v", source, sinks)
		}
	}
	for source, sinks := range expectedEdges {
		if e := actualEdges[source]; !SlicesMatch(e, sinks) {
			t.Fatalf("missing4 %s -> %v", source, sinks)
		}
	}
}

func TestInsertComponentBetween2(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 3 -> 4
	err := g3.InsertComponentBetween(g1, g3.Vertices["g3n3"], g3.Vertices["g3n4"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]Node{
		"g1n1": g1.Vertices["g1n1"],
		"g1n2": g1.Vertices["g1n2"],
		"g1n3": g1.Vertices["g1n3"],
		"g3n1": g3.Vertices["g3n1"],
		"g3n2": g3.Vertices["g3n2"],
		"g3n3": g3.Vertices["g3n3"],
		"g3n4": g3.Vertices["g3n4"],
	}
	expectedEdges := map[string][]string{
		"g1n1": []string{"g1n2"},
		"g1n2": []string{"g1n3"},
		"g1n3": []string{"g3n4"},

		"g3n1": []string{"g3n2", "g3n3"},
		"g3n2": []string{"g3n4"},
		"g3n3": []string{"g1n1"},
	}

	actualEdges := g3.Edges
	actualVertices := g3.Vertices
	for vertexName, node := range actualVertices {
		if n, ok := expectedVertices[vertexName]; !ok || n != node {
			t.Fatalf("missing1 %v", n)
		}
	}
	for vertexName, node := range expectedVertices {
		if n, ok := actualVertices[vertexName]; !ok || n != node {
			t.Fatalf("missing2 %v", vertexName)
		}
	}
	for source, sinks := range actualEdges {
		if e := expectedEdges[source]; !SlicesMatch(e, sinks) {
			t.Fatalf("missing3 %s -> %v", source, sinks)
		}
	}
	for source, sinks := range expectedEdges {
		if e := actualEdges[source]; !SlicesMatch(e, sinks) {
			t.Fatalf("missing4 %s -> %v", source, sinks)
		}
	}
}

func TestInsertComponentBetween3(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 1 -> 2
	err := g3.InsertComponentBetween(g1, g3.Vertices["g3n1"], g3.Vertices["g3n2"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]Node{
		"g1n1": g1.Vertices["g1n1"],
		"g1n2": g1.Vertices["g1n2"],
		"g1n3": g1.Vertices["g1n3"],
		"g3n1": g3.Vertices["g3n1"],
		"g3n2": g3.Vertices["g3n2"],
		"g3n3": g3.Vertices["g3n3"],
		"g3n4": g3.Vertices["g3n4"],
	}
	expectedEdges := map[string][]string{
		"g1n1": []string{"g1n2"},
		"g1n2": []string{"g1n3"},
		"g1n3": []string{"g3n2"},

		"g3n1": []string{"g1n1", "g3n3"},
		"g3n2": []string{"g3n4"},
		"g3n3": []string{"g3n4"},
	}

	actualEdges := g3.Edges
	actualVertices := g3.Vertices
	for vertexName, node := range actualVertices {
		if n, ok := expectedVertices[vertexName]; !ok || n != node {
			t.Fatalf("missing1 %v", n)
		}
	}
	for vertexName, node := range expectedVertices {
		if n, ok := actualVertices[vertexName]; !ok || n != node {
			t.Fatalf("missing2 %v", vertexName)
		}
	}
	for source, sinks := range actualEdges {
		if e := expectedEdges[source]; !SlicesMatch(e, sinks) {
			t.Fatalf("missing3 %s -> %v", source, sinks)
		}
	}
	for source, sinks := range expectedEdges {
		if e := actualEdges[source]; !SlicesMatch(e, sinks) {
			t.Fatalf("missing4 %s -> %v", source, sinks)
		}
	}
}
