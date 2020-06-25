// Copyright 2017-2018, Square, Inc.

package grapher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/id"
	"github.com/square/spincycle/test/mock"
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

var req = proto.Request{Id: "reqABC"}

func testGrapher() *Grapher {
	tf := &testFactory{}
	sequencesFile := "../test/specs/decomm.yaml"
	cfg, _ := ReadConfig(sequencesFile)
	return NewGrapher(req, tf, cfg, id.NewGenerator(4, 100))
}

func testConditionalGrapher() *Grapher {
	tf := &testFactory{}
	sequencesFile := "../test/specs/destroy-conditional.yaml"
	cfg, _ := ReadConfig(sequencesFile)
	return NewGrapher(req, tf, cfg, id.NewGenerator(4, 100))
}

func TestNodeArgs(t *testing.T) {
	omg := testGrapher()
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("decommission-cluster", args)
	if err != nil {
		t.Fatal(err)
	}

	for _, node := range g.Vertices {
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
	omg := testGrapher()
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("decommission-cluster", args)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the retries are set correctly on all nodes. Only the "get-instances" node should have retries.
	found := false
	for _, node := range g.Vertices {
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
	omg := testGrapher()
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	// create the graph
	// decommission-cluster-seq-retry has a sequence retry configured
	g, err := omg.CreateGraph("decommission-cluster", args)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the sequence retries are set correctly on all nodes. Only the "sequence_decommission-cluster_start" node should have retries.
	found := false
	sequenceStartNodeName := "sequence_pre-flight-checks_start"
	for _, node := range g.Vertices {
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

func TestCreateDecomGraph(t *testing.T) {
	omg := testGrapher()
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("decommission-cluster", args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.First.Datum.Id().Id

	verifyStep(g, g.Edges[startNode], 1, "get-instances", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "sequence_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "check-ok", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "check-ok-again", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "sequence_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "sequence_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "decom-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "decom-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "decom-3", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 4, "sequence_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "first-cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "second-cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	if len(currentStep) != 2 {
		t.Fatalf("Expected %s to have 2 out edges", "second-cleanup-job")
	}
	if g.Vertices[currentStep[0]].Name != "third-cleanup-job" &&
		g.Vertices[currentStep[1]].Name != "third-cleanup-job" {
		t.Fatalf("third-cleanup-job@ missing")
	}

	if g.Vertices[currentStep[0]].Name != "fourth-cleanup-job" &&
		g.Vertices[currentStep[1]].Name != "fourth-cleanup-job" {
		t.Fatalf("fourth-cleanup-job@ missing")
	}

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_decommission-cluster_end", t)
}

// Test creating a graph that runs an "each" on an empty slice. This used to
// cause a panic, so all we're testing here is that it no longer does that.
func TestCreateDecomGraphNoNodes(t *testing.T) {
	tf := &testFactory{}
	sequencesFile := "../test/specs/empty-cluster.yaml"
	cfg, _ := ReadConfig(sequencesFile)
	omg := NewGrapher(req, tf, cfg, id.NewGenerator(4, 100))

	args := map[string]interface{}{
		"cluster": "empty-cluster-001",
		"env":     "testing",
	}

	// create the graph
	_, err := omg.CreateGraph("empty-cluster-test", args)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateDestroyConditionalGraph(t *testing.T) {
	omg := testConditionalGrapher()
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("destroy-conditional", args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.First.Datum.Id().Id

	verifyStep(g, g.Edges[startNode], 1, "prep-1", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(g, currentStep, 1, "conditional_destroy-container_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-lxc_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-lxc_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-conditional_end", t)
}

func TestCreateDoubleConditionalGraph(t *testing.T) {
	omg := testConditionalGrapher()
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("double-conditional", args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.First.Datum.Id().Id

	verifyStep(g, g.Edges[startNode], 1, "prep-1", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(g, currentStep, 1, "conditional_archive-container_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_archive_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "archive-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "archive-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_archive_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_archive-container_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_destroy-container_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-lxc_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-lxc_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_double-conditional_end", t)
}

func TestCreateNestedConditionalGraph(t *testing.T) {
	omg := testConditionalGrapher()
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("conditional-in-conditional", args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.First.Datum.Id().Id

	verifyStep(g, g.Edges[startNode], 1, "prep-outer", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(g, currentStep, 1, "conditional_handle-container_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-conditional_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_destroy-container_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-lxc_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-lxc_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-conditional_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_handle-container_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "cleanup-job-outer", t)
}

func TestCreateDefaultConditionalGraph(t *testing.T) {
	omg := testConditionalGrapher()
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("conditional-default", args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.First.Datum.Id().Id

	verifyStep(g, g.Edges[startNode], 1, "prep-1", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(g, currentStep, 1, "conditional_destroy-container_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-docker_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "destroy-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_destroy-docker_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "conditional_destroy-container_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_conditional-default_end", t)
}

func TestFailCreateNoDefaultConditionalGraph(t *testing.T) {
	omg := testConditionalGrapher()
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	// create the graph
	_, err := omg.CreateGraph("no-default-fail", args)
	if err == nil {
		t.Errorf("no error creating grapher without default conditional, expected an error")
	}
}

func TestFailCreateBadIfConditionalGraph(t *testing.T) {
	omg := testConditionalGrapher()
	args := map[string]interface{}{
		"container": "test-container-001",
		"env":       "testing",
	}

	// create the graph
	_, err := omg.CreateGraph("bad-if-fail", args)
	if err == nil {
		t.Errorf("no error creating grapher without default conditional, expected an error")
	}
}

func TestLimitParallel(t *testing.T) {
	tf := &testFactory{}
	sequencesFile := "../test/specs/decomm-limit-parallel.yaml"
	cfg, _ := ReadConfig(sequencesFile)
	omg := NewGrapher(req, tf, cfg, id.NewGenerator(4, 100))
	args := map[string]interface{}{
		"cluster": "test-cluster-001",
		"env":     "testing",
	}

	// create the graph
	g, err := omg.CreateGraph("decommission-cluster", args)
	if err != nil {
		t.Fatal(err)
	}

	// validate the adjacency list
	startNode := g.First.Datum.Id().Id

	verifyStep(g, g.Edges[startNode], 1, "get-instances", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 3, "sequence_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 3, "check-ok", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 3, "check-ok-again", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 3, "sequence_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_pre-flight-checks_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "check-ok", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "check-ok-again", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_pre-flight-checks_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "prep-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "sequence_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "decom-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "decom-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "decom-3", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "sequence_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "sequence_decommission-instances_start", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "decom-1", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "decom-2", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "decom-3", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 2, "sequence_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "repeat_decommission-instances_end", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "first-cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "second-cleanup-job", t)

	currentStep = getNextStep(g.Edges, currentStep)
	if len(currentStep) != 2 {
		t.Fatalf("Expected %s to have 2 out edges", "second-cleanup-job")
	}
	if g.Vertices[currentStep[0]].Name != "third-cleanup-job" &&
		g.Vertices[currentStep[1]].Name != "third-cleanup-job" {
		t.Fatalf("third-cleanup-job@ missing")
	}

	if g.Vertices[currentStep[0]].Name != "fourth-cleanup-job" &&
		g.Vertices[currentStep[1]].Name != "fourth-cleanup-job" {
		t.Fatalf("fourth-cleanup-job@ missing")
	}

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(g, currentStep, 1, "sequence_decommission-cluster_end", t)
}

/////////////////////////////////////////////////////////////////////////////
// Util functions
/////////////////////////////////////////////////////////////////////////////

func getNextStep(edges map[string][]string, nodes []string) []string {
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

func verifyStep(g *Graph, nodes []string, expectedCount int, expectedName string, t *testing.T) {
	if len(nodes) != expectedCount {
		t.Fatalf("%v: expected %d out edges, but got %d", nodes, expectedCount, len(nodes))
	}
	for _, n := range nodes {
		if g.Vertices[n].Name != expectedName {
			t.Fatalf("unexpected node: %v, expecting: %s*", g.Vertices[n].Name, expectedName)
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

// //////////////////////////////////////////////////////////////////////////
// Graph tests
// //////////////////////////////////////////////////////////////////////////

// three nodes in a straight line
func g1() *Graph {
	tf := &testFactory{}
	n1, _ := tf.Make(job.NewIdWithRequestId("g1n1", "g1n1", "g1n1", "reqId1"))
	n2, _ := tf.Make(job.NewIdWithRequestId("g1n2", "g1n2", "g1n2", "reqId1"))
	n3, _ := tf.Make(job.NewIdWithRequestId("g1n3", "g1n3", "g1n3", "reqId1"))
	g1n1 := &Node{Datum: n1}
	g1n2 := &Node{Datum: n2}
	g1n3 := &Node{Datum: n3}

	g1n1.Prev = map[string]*Node{}
	g1n1.Next = map[string]*Node{"g1n2": g1n2}
	g1n2.Next = map[string]*Node{"g1n3": g1n3}
	g1n2.Prev = map[string]*Node{"g1n1": g1n1}
	g1n3.Prev = map[string]*Node{"g1n2": g1n2}
	g1n3.Next = map[string]*Node{}

	return &Graph{
		Name:  "test1",
		First: g1n1,
		Last:  g1n3,
		Vertices: map[string]*Node{
			"g1n1": g1n1,
			"g1n2": g1n2,
			"g1n3": g1n3,
		},
		Edges: map[string][]string{
			"g1n1": []string{"g1n2"},
			"g1n2": []string{"g1n3"},
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
	tf := &testFactory{}
	n1, _ := tf.Make(job.NewIdWithRequestId("g3n1", "g3n1", "g3n1", "reqId1"))
	n2, _ := tf.Make(job.NewIdWithRequestId("g3n2", "g3n2", "g3n2", "reqId1"))
	n3, _ := tf.Make(job.NewIdWithRequestId("g3n3", "g3n3", "g3n3", "reqId1"))
	n4, _ := tf.Make(job.NewIdWithRequestId("g3n4", "g3n4", "g3n4", "reqId1"))
	g3n1 := &Node{Datum: n1}
	g3n2 := &Node{Datum: n2}
	g3n3 := &Node{Datum: n3}
	g3n4 := &Node{Datum: n4}

	g3n1.Prev = map[string]*Node{}
	g3n1.Next = map[string]*Node{"g3n2": g3n2, "g3n3": g3n3}
	g3n2.Next = map[string]*Node{"g3n4": g3n4}
	g3n3.Next = map[string]*Node{"g3n4": g3n4}
	g3n4.Prev = map[string]*Node{"g3n2": g3n2, "g3n3": g3n3}
	g3n2.Prev = map[string]*Node{"g3n1": g3n1}
	g3n3.Prev = map[string]*Node{"g3n1": g3n1}
	g3n4.Next = map[string]*Node{}

	return &Graph{
		Name:  "test1",
		First: g3n1,
		Last:  g3n4,
		Vertices: map[string]*Node{
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
	}
}

// 18 node graph of something
func g2() *Graph {
	tf := &testFactory{}
	n := [20]*Node{}
	for i := 0; i < 20; i++ {
		m := fmt.Sprintf("g2n%d", i)
		p, _ := tf.Make(job.NewIdWithRequestId(m, m, m, "reqId1"))
		n[i] = &Node{
			Datum: p,
			Next:  map[string]*Node{},
			Prev:  map[string]*Node{},
		}
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
		Vertices: map[string]*Node{
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
	}
}

func TestCreateAdjacencyList1(t *testing.T) {
	g := g1()
	edges, vertices := g.createAdjacencyList()
	// check that all the vertex lists match
	for vertexName, node := range vertices {
		if n, ok := g.Vertices[vertexName]; !ok || n != node {
			t.Fatalf("missing %v", n)
		}
	}
	for vertexName, node := range g.Vertices {
		if n, ok := vertices[vertexName]; !ok || n != node {
			t.Fatalf("missing %v", n)
		}
	}
	for source, sinks := range edges {
		if e := g.Edges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing %s -> %v", source, sinks)
		}
	}
	for source, sinks := range g.Edges {
		if e := edges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing %s -> %v", source, sinks)
		}
	}
}
func TestCreateAdjacencyList2(t *testing.T) {
	g := g2()
	edges, vertices := g.createAdjacencyList()
	// check that all the vertex lists match
	for vertexName, node := range vertices {
		if n, ok := g.Vertices[vertexName]; !ok || n != node {
			t.Fatalf("missing %v", n)
		}
	}
	for vertexName, node := range g.Vertices {
		if n, ok := vertices[vertexName]; !ok || n != node {
			t.Fatalf("missing %v", n)
		}
	}
	for source, sinks := range edges {
		if e := g.Edges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing %s -> %v", source, sinks)
		}
	}
	for source, sinks := range g.Edges {
		if e := edges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing %s -> %v", source, sinks)
		}
	}
}
func TestCreateAdjacencyList3(t *testing.T) {
	g := g3()
	edges, vertices := g.createAdjacencyList()
	// check that all the vertex lists match
	for vertexName, node := range vertices {
		if n, ok := g.Vertices[vertexName]; !ok || n != node {
			t.Fatalf("missing %v", n)
		}
	}
	for vertexName, node := range g.Vertices {
		if n, ok := vertices[vertexName]; !ok || n != node {
			t.Fatalf("missing %v", n)
		}
	}
	for source, sinks := range edges {
		if e := g.Edges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing %s -> %v", source, sinks)
		}
	}
	for source, sinks := range g.Edges {
		if e := edges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing %s -> %v", source, sinks)
		}
	}
}

func TestInsertComponentBetween1(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 2 -> 4
	err := g3.insertComponentBetween(g1, g3.Vertices["g3n2"], g3.Vertices["g3n4"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]*Node{
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

	actualEdges, actualVertices := g3.createAdjacencyList()
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
		if e := expectedEdges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing3 %s -> %v", source, sinks)
		}
	}
	for source, sinks := range expectedEdges {
		if e := actualEdges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing4 %s -> %v", source, sinks)
		}
	}
}

func TestInsertComponentBetween2(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 3 -> 4
	err := g3.insertComponentBetween(g1, g3.Vertices["g3n3"], g3.Vertices["g3n4"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]*Node{
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

	actualEdges, actualVertices := g3.createAdjacencyList()
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
		if e := expectedEdges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing3 %s -> %v", source, sinks)
		}
	}
	for source, sinks := range expectedEdges {
		if e := actualEdges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing4 %s -> %v", source, sinks)
		}
	}
}

func TestInsertComponentBetween3(t *testing.T) {
	g1 := g1()
	g3 := g3()
	// insert g1 into g3 between nodes 1 -> 2
	err := g3.insertComponentBetween(g1, g3.Vertices["g3n1"], g3.Vertices["g3n2"])
	if err != nil {
		t.Fatal(err)
	}

	expectedVertices := map[string]*Node{
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

	actualEdges, actualVertices := g3.createAdjacencyList()
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
		if e := expectedEdges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing3 %s -> %v", source, sinks)
		}
	}
	for source, sinks := range expectedEdges {
		if e := actualEdges[source]; !slicesMatch(e, sinks) {
			t.Fatalf("missing4 %s -> %v", source, sinks)
		}
	}
}

func TestOptArgs001(t *testing.T) {
	tf := &mock.JobFactory{
		Created: map[string]*mock.Job{},
	}
	s, err := ReadConfig("../test/specs/opt-args-001.yaml")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGrapher(req, tf, s, id.NewGenerator(4, 100))

	args := map[string]interface{}{
		"cmd":  "sleep",
		"args": "3",
	}
	got, err := g.CreateGraph("req", args)
	if err != nil {
		t.Fatal(err)
	}
	// Find the node we want
	var j *Node
	for _, node := range got.Vertices {
		if node.Name == "job1name" {
			j = node
		}
	}
	if j == nil {
		t.Logf("%#v", got.Vertices)
		t.Fatal("graph.Vertices[job1name] not set")
	}
	if diff := deep.Equal(j.Args, args); diff != nil {
		t.Logf("%#v\n", j.Args)
		t.Error(diff)
	}
	job := tf.Created[j.Datum.Id().Name]
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
	s, err = ReadConfig("../test/specs/opt-args-001.yaml")
	if err != nil {
		t.Fatal(err)
	}
	g = NewGrapher(req, tf, s, id.NewGenerator(4, 100))

	got, err = g.CreateGraph("req", args)
	if err != nil {
		t.Fatal(err)
	}
	// Find the node we want
	for _, node := range got.Vertices {
		if node.Name == "job1name" {
			j = node
		}
	}
	if j == nil {
		t.Logf("%#v", got.Vertices)
		t.Fatal("graph.Vertices[job1name] not set")
	}
	if diff := deep.Equal(j.Args, args); diff != nil {
		t.Logf("%#v\n", j.Args)
		t.Error(diff)
	}
	job = tf.Created[j.Datum.Id().Name]
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
	s, err := ReadConfig("../test/specs/bad-each-001.yaml")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGrapher(req, tf, s, id.NewGenerator(4, 100))

	args := map[string]interface{}{
		"host": "foo",
	}
	_, err = g.CreateGraph("bad-each", args)
	// Should get "each:instances:instance: arg instances is a string, expected a slice"
	if err == nil {
		t.Error("err is nil, expected an error")
	}
}

func TestAuthSpec(t *testing.T) {
	// Test that the acl: part of a request is parsed properly
	got, err := ReadConfig("../test/specs/auth-001.yaml")
	if err != nil {
		t.Fatal(err)
	}
	expect := Config{
		Sequences: map[string]*SequenceSpec{
			"req1": &SequenceSpec{
				Name:    "req1",
				Request: true,
				Args: SequenceArgs{
					Required: []*ArgSpec{
						{
							Name: "arg1",
							Desc: "required arg 1",
						},
					},
				},
				ACL: []ACL{
					{
						Role:  "role1",
						Admin: true,
					},
					{
						Role:  "role2",
						Admin: false,
						Ops:   []string{"start", "stop"},
					},
				},
				Nodes: map[string]*NodeSpec{
					"node1": &NodeSpec{
						Name:     "node1",
						Category: "job",
						NodeType: "job1type",
					},
				},
			},
		},
	}
	if diff := deep.Equal(got, expect); diff != nil {
		t.Error(diff)
	}
}

func TestConditionalIfOptionalArg(t *testing.T) {
	// This spec has "if: foo" where "foo" is an optional arg with no value,
	// so grapher should use "default: defaultSeq", which we can see below in
	// "sequence_defaultSeq_start/end" nodes.
	cfg, err := ReadConfig("../test/specs/cond-args-001.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// Mock ID gen so we get known numbering
	idNo := 0
	idgen := mock.IDGenerator{
		UIDFunc: func() (string, error) {
			idNo++
			return fmt.Sprintf("id%d", idNo), nil
		},
	}

	gr := NewGrapher(req, &testFactory{}, cfg, idgen)
	args := map[string]interface{}{
		"cmd": "cmd-val",
	}
	got, err := gr.CreateGraph("request-name", args)
	if err != nil {
		t.Fatal(err)
	}

	// Partial nodes from the spec. Just want to verify the name and sequence IDs
	// are what we expect, and the order expressed by Edges. This let's us see/verify
	// that defaultSeq is created.
	id2 := &Node{
		Name:       "sequence_request-name_end",
		SequenceId: "id1",
	}
	id7 := &Node{
		Name:       "conditional_job1name_end",
		SequenceId: "id1",
	}
	id4 := &Node{
		Name:       "sequence_defaultSeq_end",
		SequenceId: "id3",
	}
	id5 := &Node{ // category: job, type: job1
		Name:       "job1name",
		SequenceId: "id3",
	}
	id3 := &Node{
		Name:       "sequence_defaultSeq_start",
		SequenceId: "id3",
	}
	id6 := &Node{
		Name:       "conditional_job1name_start",
		SequenceId: "id1",
	}
	id1 := &Node{
		Name:       "sequence_request-name_start",
		SequenceId: "id1",
	}
	verticies := map[string]*Node{
		"id1": id1,
		"id2": id2,
		"id3": id3,
		"id4": id4,
		"id5": id5,
		"id6": id6,
		"id7": id7,
	}
	expect := &Graph{
		//Name:     "sequence_request-name",
		//First:    verticies["id1"],
		//Last:     verticies["id7"],
		//Vertices: verticies,
		Edges: map[string][]string{
			"id1": []string{"id6"},
			"id6": []string{"id3"},
			"id3": []string{"id5"},
			"id5": []string{"id4"},
			"id4": []string{"id7"},
			"id7": []string{"id2"},
		},
	}
	if diff := deep.Equal(got.Edges, expect.Edges); diff != nil {
		t.Logf("   got: %#v", got.Edges)
		t.Logf("expect: %#v", expect.Edges)
		t.Error(diff)
	}
	for k, v := range got.Vertices {
		t.Logf("vertex: %+v", v)
		if v.Name != verticies[k].Name {
			t.Errorf("node '%s'.Name = %s, expected %s", k, v.Name, verticies[k].Name)
		}
		if v.SequenceId != verticies[k].SequenceId {
			t.Errorf("node '%s'.SequenceId = %s, expected %s", k, v.SequenceId, verticies[k].SequenceId)
		}
	}
}

// //////////////////////////////////////////////////////////////////////////
// Spec tests
// //////////////////////////////////////////////////////////////////////////

func TestFailSpecBadParallel(t *testing.T) {
	sequencesFile := "../test/specs/spec-bad-parallel.yaml"
	_, err := ReadConfig(sequencesFile)
	if err == nil {
		t.Errorf("successfully read Parallel: 0, expected an error")
	}
}
