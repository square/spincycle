// Copyright 2017-2020, Square, Inc.

package template

import (
	"fmt"
	"testing"

	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
	rmtest "github.com/square/spincycle/v2/request-manager/test"
)

var (
	SequenceNotFoundError = fmt.Errorf("sequence not found in SequenceTemplates or SequenceErrors")
)

func MakeGrapher(t *testing.T, sequencesFile string, logFunc func(string, ...interface{})) *Grapher {
	specs, err := spec.ParseSpec(rmtest.SpecPath+"/"+sequencesFile, logFunc)
	if err != nil {
		t.Fatal(err)
	}
	spec.ProcessSpecs(&specs)

	return NewGrapher(specs, id.NewGeneratorFactory(4, 100), logFunc)
}

func verifyDecomGraph(t *testing.T, grapher *Grapher) {
	var g *graph.Graph
	var template *Graph
	var ok bool
	var sequence, startNode string
	var currentStep []string

	/* Verify sequence: `decommission-cluster`. */
	sequence = "decommission-cluster"
	template, ok = grapher.sequenceTemplates[sequence]
	if !ok {
		t.Fatalf("could not find template for sequence %s", sequence)
	}
	g = &template.graph

	startNode = g.First.Id()
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
	template, ok = grapher.sequenceTemplates[sequence]
	if !ok {
		t.Fatalf("could not find template for sequence %s", sequence)
	}
	g = &template.graph

	startNode = g.First.Id()
	currentStep = g.Edges[startNode]
	verifyStep(t, g, currentStep, []string{"check-ok"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"check-ok-again"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"check-instance-is-ok_end"})

	/* Verify sequence: `decommission-instance`. */
	sequence = "decommission-instance"
	template, ok = grapher.sequenceTemplates[sequence]
	if !ok {
		t.Fatalf("could not find template for sequence %s", sequence)
	}
	g = &template.graph

	startNode = g.First.Id()
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
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if !ok {
		t.Fatal("unexpected error creating templates")
	}

	verifyDecomGraph(t, grapher)
}

// Try to create twice with same grapher
func TestCreateDecomGraphTwice(t *testing.T) {
	sequenceFile := "decomm.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if !ok {
		t.Fatal("unexpected error creating templates")
	}

	verifyDecomGraph(t, grapher)
}

func TestCreateDecomSetsGraph(t *testing.T) {
	sequenceFile := "decomm.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if !ok {
		t.Fatal("unexpected error creating templates")
	}
	verifyDecomGraph(t, grapher)
	_, ok = grapher.CreateTemplates()
	if !ok {
		t.Fatal("unexpected error creating templates")
	}
	verifyDecomGraph(t, grapher)
}

func TestCreateDestroyConditionalGraph(t *testing.T) {
	sequenceFile := "destroy-conditional.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if !ok {
		t.Fatal("unexpected error creating templates")
	}

	var g *graph.Graph
	var template *Graph
	var sequence, startNode string
	var currentStep []string

	/* Verify sequence: `destroy-conditional`. */
	sequence = "destroy-conditional"
	template, ok = grapher.sequenceTemplates[sequence]
	if !ok {
		t.Fatalf("could not find template for sequence %s", sequence)
	}
	g = &template.graph

	startNode = g.First.Id()
	currentStep = g.Edges[startNode]
	verifyStep(t, g, currentStep, []string{"prep-1"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"destroy-container"})

	currentStep = getNextStep(g, currentStep)
	verifyStep(t, g, currentStep, []string{"cleanup-job"})

}

func TestFailMissingSetsGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if ok {
		t.Fatal("no error creating template with incorrectly specified `sets`, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "missing-sets-subsequence-1"
	if err := getTemplateError(subsequence, grapher); err != nil {
		t.Fatalf("error creating template for sequence %s, expected no error: %s", subsequence, err)
	}
	subsequence = "missing-sets"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
}

func TestFailMissingSetsConditionalGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if ok {
		t.Fatal("no error creating template with incorrectly specified `sets`, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "missing-sets-subsequence-1"
	if err := getTemplateError(subsequence, grapher); err != nil {
		t.Fatalf("error creating template for sequence %s, expected no error: %s", subsequence, err)
	}
	subsequence = "missing-sets-subsequence-2"
	if err := getTemplateError(subsequence, grapher); err != nil {
		t.Fatalf("error creating template for sequence %s, expected no error: %s", subsequence, err)
	}
	subsequence = "missing-sets-subsequence-3"
	if err := getTemplateError(subsequence, grapher); err != nil {
		t.Fatalf("error creating template for sequence %s, expected no error: %s", subsequence, err)
	}
	subsequence = "missing-sets-conditional"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
}

func TestFailMissingJobArgsGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if ok {
		t.Fatal("no error creating template with missing job args, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "missing-job-args-subsequence"
	if err := getTemplateError(subsequence, grapher); err != nil {
		t.Fatalf("error creating template for sequence %s, expected no error: %s", subsequence, err)
	}
	subsequence = "missing-job-args"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
}

func TestFailCircularDependenciesGraphCheck(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if ok {
		t.Fatal("no error creating template with circular dependencies, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "circular-dependencies"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
}

func TestFailPropagate(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if ok {
		t.Fatal("no error creating template with circular dependencies, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "propagate-subsequence-1"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
	subsequence = "propagate"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
}

func TestFailPropagateConditional(t *testing.T) {
	sequenceFile := "graph-checks.yaml"
	grapher := MakeGrapher(t, sequenceFile, t.Logf)
	_, ok := grapher.CreateTemplates()
	if ok {
		t.Fatal("no error creating template with circular dependencies, expected error")
	}

	var subsequence string

	// verify that error occurred in expected subsequence
	subsequence = "propagate-subsequence-1"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
	subsequence = "propagate-subsequence-2"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
	subsequence = "propagate-subsequence-3"
	if err := getTemplateError(subsequence, grapher); err != nil {
		t.Fatalf("error creating template for sequence %s, expected no error: %s", subsequence, err)
	}
	subsequence = "propagate-conditional"
	if err := getTemplateError(subsequence, grapher); err == nil {
		t.Fatalf("no error creating template for sequence %s, expected error", subsequence)
	}
}

func getTemplateError(sequenceName string, grapher *Grapher) error {
	if _, ok := grapher.sequenceTemplates[sequenceName]; ok {
		return nil
	}
	if err, ok := grapher.sequenceErrors[sequenceName]; ok {
		return err
	}
	return SequenceNotFoundError
}

func getNextStep(g *graph.Graph, nodes []string) []string {
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

func verifyStep(t *testing.T, g *graph.Graph, nodeIds, expectedNodeNames []string) {
	expected := map[string]int{}
	for _, name := range expectedNodeNames {
		expected[name]++
	}
	actual := []string{}
	for _, id := range nodeIds {
		name := g.Vertices[id].Name()
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
