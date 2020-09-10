package graph

import (
	"fmt"
	"strings"
	"testing"
)

// There's something wrong with the way this graph is output, but L can't recreate
// it with a smaller graph, and it doesn't it hasn't occurred in any other graphs
// (test or actual), and L doesn't have time to fix it now.
var edges1 = map[string][]string{
	"0":  []string{"1", "3"},
	"1":  []string{"2"},
	"2":  []string{"17"},
	"3":  []string{"4", "5"},
	"4":  []string{"6"},
	"5":  []string{"6"},
	"6":  []string{"7", "11", "12"},
	"7":  []string{"8", "9"},
	"8":  []string{"10"},
	"9":  []string{"10"},
	"10": []string{"13"},
	"11": []string{"13", "14"},
	"12": []string{"14"},
	"13": []string{"2", "14", "15"},
	"14": []string{"16"},
	"15": []string{"17"},
	"16": []string{"17"},
	"17": []string{"18"},
}

var edges2 = map[string][]string{
	"0": []string{"1", "2"},
	"1": []string{"3"},
	"2": []string{"4", "5"},
	"3": []string{"4", "6"},
	"4": []string{"6"},
	"5": []string{"6"},
	"6": []string{"7"},
}

var edges3 = map[string][]string{
	"0": []string{"1", "2"},
	"1": []string{"3", "4", "5"},
	"2": []string{"6"},
	"3": []string{"7"},
	"4": []string{"7"},
	"5": []string{"7"},
	"6": []string{"7"},
}

var edges4 = map[string][]string{
	"0": []string{"1", "2", "3"},
	"1": []string{"4"},
	"2": []string{"4", "5"},
	"3": []string{"5"},
	"4": []string{"5"},
}

func validateGraph(edges map[string][]string, expected string, t *testing.T) {
	vertices := map[string]bool{}
	for i := 0; i <= len(edges); i++ {
		vertices[fmt.Sprintf("%d", i)] = true
	}
	g := New(vertices, edges)

	for _, e := range strings.Split(expected, "\n") {
		if !g.NextLine() {
			t.Fatalf("Expected '%s', got end of graph", e)
		}
		graphline, id := g.GetLine()
		jobline := ""
		if id != nil {
			jobline = *id
		}
		a := fmt.Sprintf("%s %s", graphline, jobline)
		if e != a {
			t.Fatalf("Expected '%s', got '%s'", e, a)
		}
	}
	if g.NextLine() {
		graphline, id := g.GetLine()
		jobline := ""
		if id != nil {
			jobline = *id
		}
		a := fmt.Sprintf("%s %s", graphline, jobline)
		t.Fatalf("Expected end of graph, got '%s'", a)
	}
}

func Test1(t *testing.T) {
	expected := `*    0
|\   
| |  
| *    3
| |\   
| | |  
* | |  1
| | |  
| | *  5
| | |  
| * |  4
| |/   
| |    
| *-.    6
| |\ \   
| | | |  
| | | *  12
| | | |  
| | * |  11
| | |\|  
| | | |  
| * | |    7
| |\ \ \   
| | | | |  
| | * | |  9
| | | | |  
| * | | |  8
| |/ / /   
| | | |    
| * | |  10
| |/ /   
| | |    
| *-. |  13
|/|\ \   
| | |/   
| |/|    
| | |    
| | *  15
| | |  
| * |  14
| | |  
* | |  2
| |/   
|/|    
| |    
| *  16
|/   
|    
*  17
|  
*  18
   `
	validateGraph(edges1, expected, t)
}

func Test2(t *testing.T) {
	expected := `*    0
|\   
| |  
| *    2
| |\   
| | |  
* | |  1
| | |  
| | *  5
| | |  
* | |    3
|\ \ \   
| |/ /   
|/| /    
| |/     
| |      
* |  4
|/   
|    
*  6
|  
*  7
   `
	validateGraph(edges2, expected, t)
}

func Test3(t *testing.T) {
	expected := `*    0
|\   
| |  
| *  2
| |  
| |      
|  \     
*-. \    1
|\ \ \   
| | | |  
| | | *  6
| | | |  
| | * |  5
| | |/   
| | |    
| * |  4
| |/   
| |    
* |  3
|/   
|    
*  7
   `
	validateGraph(edges3, expected, t)
}

func Test4(t *testing.T) {
	expected := `*-.    0
|\ \   
| | |  
| | *  3
| | |  
| * |  2
| |\|  
| | |  
* | |  1
|/ /   
| |    
* |  4
|/   
|    
*  5
   `
	validateGraph(edges4, expected, t)
}
