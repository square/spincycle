// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"
	"testing"
)

// three nodes in a straight line
func g1() *Graph {
	g1n1 := &Node{Id: "g1n1"}
	g1n2 := &Node{Id: "g1n2"}
	g1n3 := &Node{Id: "g1n3"}

	return &Graph{
		Name:   "test1",
		Source: g1n1,
		Sink:   g1n3,
		Nodes: map[string]*Node{
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
	n := [20]*Node{}
	for i := 0; i < 20; i++ {
		m := fmt.Sprintf("g2n%d", i)
		n[i] = &Node{Id: m}
	}

	return &Graph{
		Name:   "g2",
		Source: n[0],
		Sink:   n[14],
		Nodes: map[string]*Node{
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
	g3n1 := &Node{Id: "g3n1"}
	g3n2 := &Node{Id: "g3n2"}
	g3n3 := &Node{Id: "g3n3"}
	g3n4 := &Node{Id: "g3n4"}

	return &Graph{
		Name:   "test1",
		Source: g3n1,
		Sink:   g3n4,
		Nodes: map[string]*Node{
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
	g0n1 := &Node{Id: "g0n1"}
	g0n2 := &Node{Id: "g0n2"}
	g0n3 := &Node{Id: "g0n3"}
	g0n4 := &Node{Id: "g0n4"}

	// .---- 3
	// |     ^
	// V     |
	// 1 --> 2 --> 4
	//
	//
	g0 := &Graph{
		Name:   "g0",
		Source: g0n1,
		Sink:   g0n4,
		Nodes: map[string]*Node{
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
	g0n1 := &Node{Id: "g0n1"}
	g0n2 := &Node{Id: "g0n2"}
	g0n3 := &Node{Id: "g0n3"}

	//    -> 3
	//   /
	// 1 --> 2 (Last)
	g0 := &Graph{
		Name:   "g0",
		Source: g0n1,
		Sink:   g0n2,
		Nodes: map[string]*Node{
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

func TestInsertComponentBetween1(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 2 -> 4
	err := g3.InsertComponentBetween(g1, g3.Nodes["g3n2"], g3.Nodes["g3n4"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]*Node{
		"g1n1": g1.Nodes["g1n1"],
		"g1n2": g1.Nodes["g1n2"],
		"g1n3": g1.Nodes["g1n3"],
		"g3n1": g3.Nodes["g3n1"],
		"g3n2": g3.Nodes["g3n2"],
		"g3n3": g3.Nodes["g3n3"],
		"g3n4": g3.Nodes["g3n4"],
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
	actualVertices := g3.Nodes
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
	err := g3.InsertComponentBetween(g1, g3.Nodes["g3n3"], g3.Nodes["g3n4"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]*Node{
		"g1n1": g1.Nodes["g1n1"],
		"g1n2": g1.Nodes["g1n2"],
		"g1n3": g1.Nodes["g1n3"],
		"g3n1": g3.Nodes["g3n1"],
		"g3n2": g3.Nodes["g3n2"],
		"g3n3": g3.Nodes["g3n3"],
		"g3n4": g3.Nodes["g3n4"],
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
	actualVertices := g3.Nodes
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
	err := g3.InsertComponentBetween(g1, g3.Nodes["g3n1"], g3.Nodes["g3n2"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]*Node{
		"g1n1": g1.Nodes["g1n1"],
		"g1n2": g1.Nodes["g1n2"],
		"g1n3": g1.Nodes["g1n3"],
		"g3n1": g3.Nodes["g3n1"],
		"g3n2": g3.Nodes["g3n2"],
		"g3n3": g3.Nodes["g3n3"],
		"g3n4": g3.Nodes["g3n4"],
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
	actualVertices := g3.Nodes
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
