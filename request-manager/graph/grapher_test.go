// Copyright 2017-2020, Square, Inc.

package graph_test

import (
	"testing"

	. "github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
	rmtest "github.com/square/spincycle/v2/request-manager/test"
)

func MakeGrapher(t *testing.T, sequencesFile string) *Grapher {
	specs, result := spec.ParseSpec(rmtest.SpecPath + "/" + sequencesFile)
	if len(result.Errors) != 0 {
		t.Fatal(result.Errors)
	}
	spec.ProcessSpecs(&specs)

	return NewGrapher(specs, id.NewGeneratorFactory(4, 100))
}

func verifyDecomGraph(t *testing.T, seqGraphs map[string]*Graph) {
	var g *Graph
	var ok bool
	var sequence, startNode string
	var currentStep []string

	/* Verify sequence: `decommission-cluster`. */
	sequence = "decommission-cluster"
	g, ok = seqGraphs[sequence]
	if !ok {
		t.Fatalf("could not find sequence graph for sequence %s", sequence)
	}

	startNode = g.Source.Id
	currentStep = g.Edges[startNode]
	verifyStep(t, g, currentStep, []string{"get-instances"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"pre-flight-checks"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"prep-1"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"decommission-instances"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"first-cleanup-job"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"second-cleanup-job"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"third-cleanup-job", "fourth-cleanup-job"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"decommission-cluster_end"})

	/* Verify sequence: `check-instance-is-ok`. */
	sequence = "check-instance-is-ok"
	g, ok = seqGraphs[sequence]
	if !ok {
		t.Fatalf("could not find sequence graph for sequence %s", sequence)
	}

	startNode = g.Source.Id
	currentStep = g.Edges[startNode]
	verifyStep(t, g, currentStep, []string{"check-ok"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"check-ok-again"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"check-instance-is-ok_end"})

	/* Verify sequence: `decommission-instance`. */
	sequence = "decommission-instance"
	g, ok = seqGraphs[sequence]
	if !ok {
		t.Fatalf("could not find sequence graph for sequence %s", sequence)
	}

	startNode = g.Source.Id
	currentStep = g.Edges[startNode]
	verifyStep(t, g, currentStep, []string{"decom-1"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"decom-2"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"decom-3"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"decommission-instance_end"})
}

func TestCreateDecomGraph(t *testing.T) {
	sequenceFile := "decomm.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	seqGraphs, seqResults := grapher.CheckSequences()
	if seqResults.AnyError {
		t.Fatal("unexpected errors creating sequence graphs")
	}

	verifyDecomGraph(t, seqGraphs)
}

// Try to create twice with same grapher
func TestCreateDecomGraphTwice(t *testing.T) {
	sequenceFile := "decomm.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	seqGraphs, seqResults := grapher.CheckSequences()
	if seqResults.AnyError {
		t.Fatal("unexpected error creating sequence graphs")
	}

	verifyDecomGraph(t, seqGraphs)

	verifyDecomGraph(t, seqGraphs)
	seqGraphs, seqResults = grapher.CheckSequences()
	if seqResults.AnyError {
		t.Fatal("unexpected error creating sequence graphs")
	}

	verifyDecomGraph(t, seqGraphs)
}

func TestCreateDecomSetsGraph(t *testing.T) {
	sequenceFile := "decomm-sets.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	seqGraphs, seqResults := grapher.CheckSequences()
	if seqResults.AnyError {
		t.Fatal("unexpected error creating sequence graphs")
	}

	verifyDecomGraph(t, seqGraphs)
}

func TestCreateDestroyConditionalGraph(t *testing.T) {
	sequenceFile := "destroy-conditional.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	seqGraphs, seqResults := grapher.CheckSequences()
	if seqResults.AnyError {
		t.Log(seqResults)
		t.Fatal("unexpected error creating sequence graphs")
	}

	var g *Graph
	var sequence, startNode string
	var currentStep []string

	/* Verify sequence: `destroy-conditional`. */
	sequence = "destroy-conditional"
	g, ok := seqGraphs[sequence]
	if !ok {
		t.Fatalf("could not find sequence graph for sequence %s", sequence)
	}

	startNode = g.Source.Id
	currentStep = g.Edges[startNode]
	verifyStep(t, g, currentStep, []string{"prep-1"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"destroy-container"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"cleanup-job"})

}

func TestFailMissingSetsGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with incorrectly specified `sets`, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "missing-sets-subsequence-1"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) != 0 {
		t.Fatalf("error creating subsequence graph for sequence %s, expected no error: %v", subsequence, errs)
	}
	subsequence = "missing-sets"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func TestFailMissingSetsConditionalGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with incorrectly specified `sets`, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "missing-sets-subsequence-1"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) != 0 {
		t.Fatalf("error creating subsequence graph for sequence %s, expected no error: %v", subsequence, errs)
	}
	subsequence = "missing-sets-subsequence-2"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) != 0 {
		t.Fatalf("error creating subsequence graph for sequence %s, expected no error: %v", subsequence, errs)
	}
	subsequence = "missing-sets-subsequence-3"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) != 0 {
		t.Fatalf("error creating subsequence graph for sequence %s, expected no error: %v", subsequence, errs)
	}
	subsequence = "missing-sets-conditional"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func TestFailMissingJobArgsGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with missing job args, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "missing-job-args-subsequence"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) != 0 {
		t.Fatalf("error creating subsequence graph for sequence %s, expected no error: %v", subsequence, errs)
	}
	subsequence = "missing-job-args"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func TestFailCircularDependenciesGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with circular dependencies, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "circular-dependencies"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func TestFailPropagate(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with circular dependencies, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "propagate-subsequence-1"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
	subsequence = "propagate"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func TestFailPropagateConditional(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with circular dependencies, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "propagate-subsequence-1"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
	subsequence = "propagate-subsequence-2"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
	subsequence = "propagate-subsequence-3"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) != 0 {
		t.Fatalf("error creating subsequence graph for sequence %s, expected no error: %v", subsequence, errs)
	}
	subsequence = "propagate-conditional"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func TestFailCyclicalSequence(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile)
	_, seqResults := grapher.CheckSequences()
	if !seqResults.AnyError {
		t.Fatal("no error creating subsequence graph with circular dependencies among sequences, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "circular-sequences-1"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
	subsequence = "circular-sequences-2"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
	subsequence = "circular-sequences-3"
	if errs := getSeqErrors(subsequence, seqResults); len(errs) == 0 {
		t.Fatalf("no error creating subsequence graph for sequence %s, expected error", subsequence)
	}
}

func getSeqErrors(sequenceName string, seqResults *spec.CheckResults) []error {
	if result, ok := seqResults.Get(sequenceName); ok && len(result.Errors) > 0 {
		return result.Errors
	}
	return nil
}

func getNextStep(g *Graph, nodes []string) []string {
	seen := map[string]bool{}
	nextStep := []string{}
	for _, n := range nodes {
		for _, e := range g.Edges[n] {
			seen[e] = true
		}
	}
	for k, _ := range seen {
		nextStep = append(nextStep, k)
	}
	// remove duplicates
	return nextStep
}

func verifyStep(t *testing.T, g *Graph, nodeIds, expectedNodeNames []string) {
	expected := map[string]int{}
	for _, name := range expectedNodeNames {
		expected[name]++
	}
	actual := []string{}
	for _, id := range nodeIds {
		name := g.Nodes[id].Name
		expected[name]--
		actual = append(actual, name)
	}
	for name, count := range expected {
		if count > 0 {
			t.Fatalf("expected node %s, got: %v", name, actual)
		}
		if count < 0 {
			t.Fatalf("expected: %v, got node %s", expectedNodeNames, name)
		}
	}
}
