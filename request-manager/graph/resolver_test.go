// Copyright 2017-2018, Square, Inc.

package graph_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/v2/job"
	"github.com/square/spincycle/v2/proto"
	. "github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
	rmtest "github.com/square/spincycle/v2/request-manager/test"
	"github.com/square/spincycle/v2/test/mock"
)

type testFactory struct{}

func (f *testFactory) Make(id job.Id) (job.Job, error) {
	j := testJob{
		id:        id,
		givenArgs: map[string]interface{}{},
	}
	return j, nil
}

type testJob struct {
	id        job.Id
	givenArgs map[string]interface{}
}

func (tj testJob) Create(args map[string]interface{}) error {
	tj.givenArgs = map[string]interface{}{}
	for k, v := range args {
		tj.givenArgs[k] = v
	}

	switch tj.id.Type {
	case "get-cluster-instances":
		return createGetClusterMembers(args)
	case "prep-job-1":
		return createPrepJob1(args)
	case "prep-job-2":
		return createPrepJob2(args)
	case "cleanup-job":
		return createCleanupJob(args)
	case "cleanup-job-2":
		return createCleanupJob2(args)
	case "check-ok-1":
		return createCheckOK1(args)
	case "check-ok-2":
		return createCheckOK2(args)
	case "decom-step-1":
		return createDecom1(args)
	case "decom-step-2":
		return createDecom2(args)
	case "decom-step-3":
		return createDecom3(args)
	case "destroy-step-1":
		return createDestroy1(args)
	case "destroy-step-2":
		return createDestroy2(args)
	case "no-op", "noop":
		return createEndNode(args)
	}

	return nil
}

func (tj testJob) Id() job.Id                 { return tj.id }
func (tj testJob) Serialize() ([]byte, error) { return nil, nil }
func (tj testJob) Deserialize(b []byte) error { return nil }
func (tj testJob) Stop() error                { return nil }
func (tj testJob) Status() string             { return "" }
func (tj testJob) Run(map[string]interface{}) (job.Return, error) {
	return job.Return{}, nil
}

func createGraph(t *testing.T, sequencesFile, requestName string, jobArgs map[string]interface{}) (*Graph, error) {
	return createGraph0(t, sequencesFile, requestName, jobArgs, &testFactory{}, id.NewGeneratorFactory(4, 100))
}

func createGraph1(t *testing.T, sequencesFile, requestName string, jobArgs map[string]interface{}, tf job.Factory) (*Graph, error) {
	return createGraph0(t, sequencesFile, requestName, jobArgs, tf, id.NewGeneratorFactory(4, 100))
}

func createGraph2(t *testing.T, sequencesFile, requestName string, jobArgs map[string]interface{}, idgenFactory id.GeneratorFactory) (*Graph, error) {
	return createGraph0(t, sequencesFile, requestName, jobArgs, &testFactory{}, idgenFactory)
}

func createGraph0(t *testing.T, sequencesFile, requestName string, jobArgs map[string]interface{}, tf job.Factory, idgenFactory id.GeneratorFactory) (*Graph, error) {
	req := proto.Request{
		Id:   "reqABC",
		Type: requestName,
	}

	specs, err, _ := spec.ParseSpec(rmtest.SpecPath + "/" + sequencesFile)
	if err != nil {
		t.Fatal(err)
	}
	spec.ProcessSpecs(&specs)

	// Add optional+static args to job args map
	seqSpec, ok := specs.Sequences[requestName]
	if !ok {
		t.Fatalf("request %s not found in specs", requestName)
	}
	for _, arg := range seqSpec.Args.Optional {
		if _, ok := jobArgs[*arg.Name]; !ok {
			jobArgs[*arg.Name] = *arg.Default
		}
	}
	for _, arg := range seqSpec.Args.Static {
		if _, ok := jobArgs[*arg.Name]; !ok {
			jobArgs[*arg.Name] = *arg.Default
		}
	}

	gr := NewGrapher(specs, id.NewGeneratorFactory(4, 100))
	seqGraphs, seqErrors := gr.CheckSequences()
	if len(seqErrors) != 0 {
		t.Fatalf("failed to create sequence graphs: %v", seqErrors)
	}

	rf := NewResolverFactory(tf, specs.Sequences, seqGraphs, idgenFactory)
	r := rf.Make(req)

	return r.BuildRequestGraph(jobArgs)
}

func TestNodeArgs(t *testing.T) {
	sequencesFile := "decomm.yaml"
	requestName := "decommission-cluster"
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	reqGraph, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	for _, node := range reqGraph.Nodes {
		// Verify that noop nodes do not have Args.
		if strings.HasPrefix(node.Name, "sequence_") || strings.HasPrefix(node.Name, "repeat_") {
			if len(node.Args) != 0 {
				t.Errorf("node %s args = %#v, expected an empty map", node.Name, node.Args)
			}
		}

		// Check the Args on some nodes.
		if node.Name == "get-instances" {
			expectedArgs := map[string]interface{}{
				"cluster": "test-cluster-001",
			}
			if diff := deep.Equal(node.Args, expectedArgs); diff != nil {
				t.Error(diff)
			}
		}
		if node.Name == "prep-1" {
			expectedArgs := map[string]interface{}{
				"cluster":   "test-cluster-001",
				"env":       "testing",
				"instances": []string{"node1", "node2", "node3", "node4"},
			}
			if diff := deep.Equal(node.Args, expectedArgs); diff != nil {
				t.Error(diff)
			}
		}
		if node.Name == "third-cleanup-job" {
			expectedArgs := map[string]interface{}{
				"cluster": "test-cluster-001",
			}
			if diff := deep.Equal(node.Args, expectedArgs); diff != nil {
				t.Error(diff)
			}
		}
	}
}

func TestNodeRetry(t *testing.T) {
	sequencesFile := "decomm.yaml"
	requestName := "decommission-cluster"
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	reqGraph, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the retries are set correctly on all nodes.
	// Only the "get-instances" node should have retries.
	found := false
	for _, node := range reqGraph.Nodes {
		if node.Name == "get-instances" {
			found = true
			if node.Retry != 3 {
				t.Errorf("%s node retries = %d, expected %d", node.Name, node.Retry, 3)
			}
			if node.RetryWait != "10s" {
				t.Errorf("%s node retryWait = %s, expected 10s", node.Name, node.RetryWait)
			}
		} else {
			if node.Retry != 0 {
				t.Errorf("%s node retries = %d, expected 0", node.Name, node.Retry)
			}
			if node.RetryWait != "" {
				t.Errorf("%s node retryWait = %s, expected empty string", node.Name, node.RetryWait)
			}
		}
	}
	if !found {
		t.Error("couldn't find vertix with node name 'get-instances'")
	}
}

func TestSequenceRetry(t *testing.T) {
	sequencesFile := "decomm.yaml"
	requestName := "decommission-cluster"
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	reqGraph, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the sequence retries are set correctly on all nodes.
	// Only the "sequence_decommission-cluster_begin" node should have retries.
	found := false
	sequenceStartNodeName := "sequence_pre-flight-checks_begin"
	for _, node := range reqGraph.Nodes {
		if node.Name == sequenceStartNodeName {
			found = true
			if node.SequenceRetry != 3 {
				t.Errorf("%s node sequence retries = %d, expected %d", node.Name, node.SequenceRetry, 2)
			}
		} else {
			if node.SequenceRetry != 0 {
				t.Errorf("%s node sequence retries = %d, expected %d", node.Name, node.SequenceRetry, 0)
			}
		}
	}
	if !found {
		t.Errorf("couldn't find vertix with node name %s", sequenceStartNodeName)
	}
}

func TestCreateDecomRequestGraph(t *testing.T) {
	sequencesFile := "decomm.yaml"
	requestName := "decommission-cluster"
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	reqGraph, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}
	reqVerifyDecomGraph(t, reqGraph)
}

func TestCreateDecomSetsRequestGraph(t *testing.T) {
	sequencesFile := "decomm-sets.yaml"
	requestName := "decommission-cluster"
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	reqGraph, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}
	reqVerifyDecomGraph(t, reqGraph)
}

func reqVerifyDecomGraph(t *testing.T, g *Graph) {
	startNode := g.Source.Id
	currentStep := g.Edges[startNode]
	reqVerifyStep(g, currentStep, 1, "decommission-cluster_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "get-instances", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "sequence_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "check-instance-is-ok_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "check-ok", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "check-ok-again", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "check-instance-is-ok_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "sequence_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "sequence_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "decommission-instance_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "decom-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "decom-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "decom-3", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "decommission-instance_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 4, "sequence_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "first-cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "second-cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	if len(currentStep) != 2 {
		t.Fatalf("Expected %s to have 2 out edges", "second-cleanup-job")
	}
	if g.Nodes[currentStep[0]].Name != "third-cleanup-job" &&
		g.Nodes[currentStep[1]].Name != "third-cleanup-job" {
		t.Fatalf("third-cleanup-job@ missing")
	}

	if g.Nodes[currentStep[0]].Name != "fourth-cleanup-job" &&
		g.Nodes[currentStep[1]].Name != "fourth-cleanup-job" {
		t.Fatalf("fourth-cleanup-job@ missing")
	}

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "decommission-cluster_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "request_decommission-cluster_end", t)
}

// Test creating a graph that runs an "each" on an empty slice. This used to
// cause a panic, so all we're testing here is that it no longer does that.
func TestCreateDecomGraphNoNodes(t *testing.T) {
	sequencesFile := "empty-cluster.yaml"
	requestName := "empty-cluster-test"
	args := map[string]interface{}{
		"cluster": "empty-cluster-001",
		"env":     "testing",
	}

	_, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateDestroyConditionalRequestGraph(t *testing.T) {
	sequencesFile := "destroy-conditional.yaml"
	requestName := "destroy-conditional"
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	g, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.Source.Id
	currentStep := g.Edges[startNode]
	reqVerifyStep(g, currentStep, 1, "destroy-conditional_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-lxc_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-lxc_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-conditional_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "request_destroy-conditional_end", t)
}

func TestCreateDoubleConditionalGraph(t *testing.T) {
	sequencesFile := "destroy-conditional.yaml"
	requestName := "double-conditional"
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	g, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.Source.Id
	currentStep := g.Edges[startNode]
	reqVerifyStep(g, currentStep, 1, "double-conditional_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_archive-container_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "archive_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "archive-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "archive-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "archive_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_archive-container_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-lxc_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-lxc_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "double-conditional_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "request_double-conditional_end", t)
}

func TestCreateNestedConditionalGraph(t *testing.T) {
	sequencesFile := "destroy-conditional.yaml"
	requestName := "conditional-in-conditional"
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	g, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.Source.Id
	currentStep := g.Edges[startNode]
	reqVerifyStep(g, currentStep, 1, "conditional-in-conditional_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-outer", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_handle-container_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-conditional_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-lxc_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-lxc_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-conditional_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_handle-container_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "cleanup-job-outer", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional-in-conditional_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "request_conditional-in-conditional_end", t)
}

func TestCreateDefaultConditionalGraph(t *testing.T) {
	sequencesFile := "destroy-conditional.yaml"
	requestName := "conditional-default"
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	g, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.Source.Id
	currentStep := g.Edges[startNode]
	reqVerifyStep(g, currentStep, 1, "conditional-default_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-docker_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "destroy-docker_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "conditional-default_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "request_conditional-default_end", t)
}

func TestFailCreateNoDefaultConditionalGraph(t *testing.T) {
	sequencesFile := "destroy-conditional.yaml"
	requestName := "no-default-fail"
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	_, err := createGraph(t, sequencesFile, requestName, args)
	if err == nil {
		t.Errorf("no error creating creator without default conditional, expected an error")
	}
}

func TestCreateLimitParallel(t *testing.T) {
	sequencesFile := "decomm-limit-parallel.yaml"
	requestName := "decommission-cluster"
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	g, err := createGraph(t, sequencesFile, requestName, args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.Source.Id
	currentStep := g.Edges[startNode]
	reqVerifyStep(g, currentStep, 1, "decommission-cluster_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "get-instances", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 3, "sequence_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 3, "check-instance-is-ok_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 3, "check-ok", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 3, "check-ok-again", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 3, "check-instance-is-ok_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 3, "sequence_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "sequence_pre-flight-checks_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "check-instance-is-ok_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "check-ok", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "check-ok-again", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "check-instance-is-ok_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "sequence_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "sequence_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decommission-instance_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decom-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decom-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decom-3", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decommission-instance_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "sequence_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "sequence_decommission-instances_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decommission-instance_begin", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decom-1", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decom-2", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decom-3", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "decommission-instance_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 2, "sequence_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "first-cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "second-cleanup-job", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	if len(currentStep) != 2 {
		t.Fatalf("Expected %s to have 2 out edges", "second-cleanup-job")
	}
	if g.Nodes[currentStep[0]].Name != "third-cleanup-job" &&
		g.Nodes[currentStep[1]].Name != "third-cleanup-job" {
		t.Fatalf("third-cleanup-job missing")
	}

	if g.Nodes[currentStep[0]].Name != "fourth-cleanup-job" &&
		g.Nodes[currentStep[1]].Name != "fourth-cleanup-job" {
		t.Fatalf("fourth-cleanup-job missing")
	}

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "decommission-cluster_end", t)

	currentStep = reqGetNextStep(g.Edges, currentStep)
	reqVerifyStep(g, currentStep, 1, "request_decommission-cluster_end", t)
}

func TestOptArgs(t *testing.T) {
	sequencesFile := "opt-args.yaml"
	requestName := "req"
	args := map[string]interface{}{
		"cmd":  "sleep",
		"args": "3",
	}
	tf := &mock.JobFactory{
		Created: map[string]*mock.Job{},
	}

	reqGraph, err := createGraph1(t, sequencesFile, requestName, args, tf)
	if err != nil {
		t.Fatal(err)
	}

	// Find the node we want
	var j *Node
	for _, node := range reqGraph.Nodes {
		if node.Name == "job1name" {
			j = node
		}
	}
	if j == nil {
		t.Logf("%#v", reqGraph.Nodes)
		t.Fatal("graph.Nodes[job1name] not set")
	}
	if diff := deep.Equal(j.Args, args); diff != nil {
		t.Logf("%#v\n", j.Args)
		t.Error(diff)
	}
	job := tf.Created[j.Name]
	if job == nil {
		t.Fatal("job job1name not created")
	}
	if diff := deep.Equal(job.CreatedWithArgs, args); diff != nil {
		t.Logf("%#v\n", job)
		t.Errorf("test job not created with args arg")
	}

	// Try again without "args", i.e. the optional arg is not given
	delete(args, "args")

	tf = &mock.JobFactory{
		Created: map[string]*mock.Job{},
	}

	reqGraph, err = createGraph1(t, sequencesFile, requestName, args, tf)
	if err != nil {
		t.Fatal(err)
	}

	// Find the node we want
	for _, node := range reqGraph.Nodes {
		if node.Name == "job1name" {
			j = node
		}
	}
	if j == nil {
		t.Logf("%#v", reqGraph.Nodes)
		t.Fatal("graph.Nodes[job1name] not set")
	}
	if diff := deep.Equal(j.Args, args); diff != nil {
		t.Logf("%#v\n", j.Args)
		t.Error(diff)
	}
	job = tf.Created[j.Name]
	if job == nil {
		t.Fatal("job job1name not created")
	}
	if _, ok := job.CreatedWithArgs["args"]; !ok {
		t.Error("jobArgs[args] does not exist, expected it to be set")
	}
	if diff := deep.Equal(job.CreatedWithArgs, args); diff != nil {
		t.Logf("%#v\n", job)
		t.Errorf("test job not created with args arg")
	}
}

func TestBadEach001(t *testing.T) {
	sequencesFile := "bad-each.yaml"
	requestName := "bad-each"
	args := map[string]interface{}{
		"host": "foo",
	}

	job := &mock.Job{
		SetJobArgs: map[string]interface{}{
			// This causes an error because the spec has each: instances:instannce,
			// so instances should be a slice, but it's a string.
			"instances": "foo",
		},
	}
	tf := &mock.JobFactory{
		MockJobs: map[string]*mock.Job{
			"get-instances": job,
		},
	}

	_, err := createGraph1(t, sequencesFile, requestName, args, tf)
	// Should get "each:instances:instance: arg instances is a string, expected a slice"
	if err == nil {
		t.Error("err is nil, expected an error")
	}
}

func TestConditionalIfOptionalArg(t *testing.T) {
	// This spec has "if: foo" where "foo" is an optional arg with no value,
	// so creator should use "default: defaultSeq", which we can see below in
	// "sequence_defaultSeq_begin/end" nodes.
	sequencesFile := "cond-args.yaml"
	requestName := "request-name"
	args := map[string]interface{}{
		"cmd": "cmd-val",
	}

	// Mock ID gen so we get known numbering
	idNo := 0
	idgen := mock.IDGenerator{
		UIDFunc: func() (string, error) {
			idNo++
			return fmt.Sprintf("id%d", idNo), nil
		},
	}
	idgenFactory := mock.IDGeneratorFactory{
		MakeFunc: func() id.Generator {
			return idgen
		},
	}
	got, err := createGraph2(t, sequencesFile, requestName, args, idgenFactory)
	if err != nil {
		t.Fatal(err)
	}

	// Partial nodes from the spec. Just want to verify the name and sequence IDs
	// are what we expect, and the order expressed by Edges. This lets us see/verify
	// that defaultSeq is created.
	id1 := &Node{
		Name:       "request_request-name_begin",
		SequenceId: "id1",
	}
	id3 := &Node{
		Name:       "request-name_begin",
		SequenceId: "id1",
	}
	id4 := &Node{
		Name:       "conditional_job1name_begin",
		SequenceId: "id4",
	}
	id6 := &Node{
		Name:       "defaultSeq_begin",
		SequenceId: "id4",
	}
	id7 := &Node{ // category: job, type: job1
		Name:       "job1name",
		SequenceId: "id4",
	}
	id8 := &Node{
		Name:       "defaultSeq_end",
		SequenceId: "id4",
	}
	id5 := &Node{
		Name:       "conditional_job1name_end",
		SequenceId: "id4",
	}
	id9 := &Node{
		Name:       "request-name_end",
		SequenceId: "id1",
	}
	id2 := &Node{
		Name:       "request_request-name_end",
		SequenceId: "id1",
	}
	vertices := map[string]*Node{
		"id1": id1,
		"id2": id2,
		"id3": id3,
		"id4": id4,
		"id5": id5,
		"id6": id6,
		"id7": id7,
		"id8": id8,
		"id9": id9,
	}
	expect := &Graph{
		//Name:     "sequence_request-name",
		//First:    vertices["id1"],
		//Last:     vertices["id7"],
		//Vertices: vertices,
		Edges: map[string][]string{
			"id1": []string{"id3"},
			"id3": []string{"id4"},
			"id4": []string{"id6"},
			"id6": []string{"id7"},
			"id7": []string{"id8"},
			"id8": []string{"id5"},
			"id5": []string{"id9"},
			"id9": []string{"id2"},
		},
		RevEdges: map[string][]string{
			"id3": []string{"id1"},
			"id4": []string{"id3"},
			"id6": []string{"id4"},
			"id7": []string{"id6"},
			"id8": []string{"id7"},
			"id5": []string{"id8"},
			"id9": []string{"id5"},
			"id2": []string{"id9"},
		},
	}
	if diff := deep.Equal(got.Edges, expect.Edges); diff != nil {
		t.Logf("   got: %#v", got.Edges)
		t.Logf("expect: %#v", expect.Edges)
		t.Error(diff)
	}
	for k, v := range got.Nodes {
		t.Logf("vertex: %+v", v)
		if v.Name != vertices[k].Name {
			t.Errorf("node '%s'.Name = %s, expected %s", k, v.Name, vertices[k].Name)
		}
		if v.SequenceId != vertices[k].SequenceId {
			t.Errorf("node '%s'.SequenceId = %s, expected %s", k, v.SequenceId, vertices[k].SequenceId)
		}
	}
}

/////////////////////////////////////////////////////////////////////////////
// Util functions
/////////////////////////////////////////////////////////////////////////////

func reqGetNextStep(edges map[string][]string, nodes []string) []string {
	seen := map[string]bool{}
	nextStep := []string{}
	for _, n := range nodes {
		for _, e := range edges[n] {
			seen[e] = true
		}
	}
	for k, _ := range seen {
		nextStep = append(nextStep, k)
	}
	// remove duplicates
	return nextStep
}

func reqVerifyStep(g *Graph, nodes []string, expectedCount int, expectedName string, t *testing.T) {
	if len(nodes) != expectedCount {
		t.Fatalf("%v: expected %d out edges, but got %d", nodes, expectedCount, len(nodes))
	}
	for _, n := range nodes {
		if g.Nodes[n].Name != expectedName {
			t.Fatalf("unexpected node: %v, expecting: %s", g.Nodes[n].Name, expectedName)
		}
	}
}

func createGetClusterMembers(args map[string]interface{}) error {
	cluster, ok := args["cluster"]
	if !ok {
		return fmt.Errorf("job get-cluster-members expected a cluster arg")
	}
	if cluster == "empty-cluster-001" {
		args["instances"] = []string{}
		return nil
	} else if cluster != "test-cluster-001" {
		return fmt.Errorf("job get-cluster-members given '%s' but wanted 'test-cluster-001'", cluster)
	}

	args["instances"] = []string{"node1", "node2", "node3", "node4"}

	return nil
}

func createPrepJob1(args map[string]interface{}) error {
	cluster, ok := args["cluster"]
	if !ok {
		return fmt.Errorf("job prep-job-1 expected a cluster arg")
	}
	if cluster != "test-cluster-001" {
		return fmt.Errorf("job prep-job-1 given '%s' but wanted 'test-cluster-001'", cluster)
	}
	env, ok := args["env"]
	if !ok {
		return fmt.Errorf("job prep-job-1 expected a env arg")
	}
	if env != "testing" {
		return fmt.Errorf("job prep-job-1 given '%s' but wanted 'testing'", env)
	}
	i, ok := args["instances"]
	if !ok {
		return fmt.Errorf("job prep-job-1 expected a env arg")
	}
	expected := []string{"node1", "node2", "node3", "node4"}
	instances, ok := i.([]string)
	if !ok || !slicesMatch(instances, expected) {
		return fmt.Errorf("job prep-job-1 given '%v' but wanted '%v'", i, expected)
	}
	return nil
}

func createPrepJob2(args map[string]interface{}) error {
	container, ok := args["container"]
	if !ok {
		return fmt.Errorf("job prep-job-2 expected a cluster arg")
	}
	if container != "test-container-001" {
		return fmt.Errorf("job prep-job-2 given '%s' but wanted 'test-container-001'", container)
	}
	env, ok := args["env"]
	if !ok {
		return fmt.Errorf("job prep-job-2 expected a env arg")
	}
	if env != "testing" {
		return fmt.Errorf("job prep-job-2 given '%s' but wanted 'testing'", env)
	}
	return nil
}

func createCleanupJob(args map[string]interface{}) error {
	cluster, ok := args["cluster"]
	if !ok {
		return fmt.Errorf("job cleanup-job expected a cluster arg")
	}
	if cluster != "test-cluster-001" {
		return fmt.Errorf("job cleanup-job given '%s' but wanted 'test-cluster-001'", cluster)
	}

	return nil
}

func createCleanupJob2(args map[string]interface{}) error {
	container, ok := args["container"]
	if !ok {
		return fmt.Errorf("job cleanup-job expected a cluster arg")
	}
	if container != "test-container-001" {
		return fmt.Errorf("job cleanup-job-2 given '%s' but wanted 'test-container-001'", container)
	}

	return nil
}

func createCheckOK1(args map[string]interface{}) error {
	container, ok := args["container"]
	if !ok {
		return fmt.Errorf("job check-ok-1 expected a container arg")
	}
	if container != "node1" && container != "node2" && container != "node3" && container != "node4" {
		return fmt.Errorf("job check-ok-1 given '%s' but wanted one of ['node1','node2','node3','node4']", container)
	}
	args["physicalhost"] = "physicalhost1"
	return nil
}

func createCheckOK2(args map[string]interface{}) error {
	container, ok := args["nodeAddr"]
	if !ok {
		return fmt.Errorf("job check-ok-1 expected a nodeAddr arg")
	}
	if container != "node1" && container != "node2" && container != "node3" && container != "node4" {
		return fmt.Errorf("job check-ok-2 given '%s' but wanted one of ['node1','node2','node3','node4']", container)
	}
	physicalhost, ok := args["hostAddr"]
	if !ok {
		return fmt.Errorf("job check-ok-1 expected a hostAddr arg")
	}
	if physicalhost != "physicalhost1" {
		return fmt.Errorf("job check-ok-2 given '%v' but wanted 'physicalhost1'", physicalhost)
	}
	return nil
}

func createDecom1(args map[string]interface{}) error {
	container, ok := args["container"]
	if !ok {
		return fmt.Errorf("job check-decom-1 expected a container arg")
	}
	if container != "node1" && container != "node2" && container != "node3" && container != "node4" {
		return fmt.Errorf("job check-decom-1 given '%s' but wanted one of ['node1','node2','node3','node4']", container)
	}
	args["physicalhost"] = "physicalhost1"
	return nil
}

func createDecom2(args map[string]interface{}) error {
	container, ok := args["dstAddr"]
	if !ok {
		return fmt.Errorf("job check-decom-2 expected a container arg")
	}
	if container != "node1" && container != "node2" && container != "node3" && container != "node4" {
		return fmt.Errorf("job check-decom-2 given '%s' but wanted one of ['node1','node2','node3','node4']", container)
	}
	args["physicalhost"] = "physicalhost1"
	return nil
}

func createDecom3(args map[string]interface{}) error {
	container, ok := args["container"]
	if !ok {
		return fmt.Errorf("job check-decom-3 expected a container arg")
	}
	if container != "node1" && container != "node2" && container != "node3" && container != "node4" {
		return fmt.Errorf("job check-decom-3 given '%s' but wanted one of ['node1','node2','node3','node4']", container)
	}
	return nil
}

func createDestroy1(args map[string]interface{}) error {
	container, ok := args["container"]
	if !ok {
		return fmt.Errorf("job check-destroy-1 expected a container arg")
	}
	if container != "test-container-001" {
		return fmt.Errorf("job check-destroy-1 given '%s' but wanted test-container-001", container)
	}
	args["physicalhost"] = "physicalhost1"
	return nil
}

func createDestroy2(args map[string]interface{}) error {
	physicalhost, ok := args["dstAddr"]
	if !ok {
		return fmt.Errorf("job check-destroy-2 expected a container arg")
	}
	if physicalhost != "physicalhost1" {
		return fmt.Errorf("job check-destroy-2 given '%s' but wanted physicalhost1", physicalhost)
	}
	return nil
}

func createEndNode(args map[string]interface{}) error {
	return nil
}
