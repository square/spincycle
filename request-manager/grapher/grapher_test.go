package grapher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/square/spincycle/job"
)

type testFactory struct{}

func (f testFactory) Make(t, n string) (job.Job, error) {
	j := testJob{
		name:    n,
		jobtype: t,
	}
	return j, nil
}

func newTestFactory() *testFactory {
	tf := &testFactory{}
	return tf
}

type testJob struct {
	name    string
	jobtype string
}

// This job will get crazy
func (tj testJob) Create(args map[string]interface{}) error {
	switch tj.jobtype {
	case "get-cluster-instances":
		return createGetClusterMembers(args)
	case "prep-job-1":
		return createPrepJob1(args)
	case "cleanup-job":
		return createCleanupJob(args)
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
	case "no-op":
		return createEndNode(args)
	default:
		return fmt.Errorf("warning job not found")
	}
	return nil
}

func (tj testJob) Serialize() ([]byte, error) { return nil, nil }
func (tj testJob) Type() string               { return tj.jobtype }
func (tj testJob) Name() string               { return tj.name }
func (tj testJob) Deserialize(b []byte) error { return nil }
func (tj testJob) Stop() error                { return nil }
func (tj testJob) Status() string             { return "" }
func (tj testJob) Run(map[string]interface{}) (job.Return, error) {
	return job.Return{}, nil
}

func testGrapher() *Grapher {
	tf := testFactory{}
	sequencesFile := "./test/example-requests.yaml"

	cfg, _ := ReadConfig(sequencesFile)

	o := &Grapher{
		nextNodeId:   0,
		JobFactory:   tf,
		AllSequences: cfg.Sequences,
		NoopNode:     cfg.NoopNode,
	}
	return o
}

func TestNodeRetries(t *testing.T) {
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
	for name, node := range g.Vertices {
		if strings.HasPrefix(name, "get-instances@") {
			found = true
			if node.Retries != 3 {
				t.Errorf("%s node retries = %d, expected %d", name, node.Retries, 3)
			}
			if node.RetryDelay != 10 {
				t.Errorf("%s node retry delay = %d, expected %d", name, node.RetryDelay, 10)
			}
		} else {
			if node.Retries != 0 {
				t.Errorf("%s node retries = %d, expected %d", name, node.Retries, 0)
			}
			if node.RetryDelay != 0 {
				t.Errorf("%s node retry delay = %d, expected %d", name, node.RetryDelay, 0)
			}
		}
	}
	if !found {
		t.Error("couldn't find vertix with node name 'get-instances@*'")
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
	startNode := g.First.Datum.Name()

	verifyStep(g.Edges[startNode], 1, "get-instances@", t)

	currentStep := getNextStep(g.Edges, g.Edges[startNode])
	verifyStep(currentStep, 1, "repeat_pre-flight-checks_start@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "sequence_pre-flight-checks_start@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "check-ok@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "check-ok-again@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "sequence_pre-flight-checks_end@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "repeat_pre-flight-checks_end@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "prep-1@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "repeat_decommission-instances_start@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "sequence_decommission-instances_start@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "decom-1@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "decom-2@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "decom-3@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 4, "sequence_decommission-instances_end@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "repeat_decommission-instances_end@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "first-cleanup-job@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "second-cleanup-job@", t)

	currentStep = getNextStep(g.Edges, currentStep)
	if len(currentStep) != 2 {
		t.Fatalf("Expected %s to have 2 out edges", "second-cleanup-job@")
	}
	if !strings.HasPrefix(currentStep[0], "third-cleanup-job@") &&
		!strings.HasPrefix(currentStep[1], "third-cleanup-job@") {
		t.Fatalf("third-cleanup-job@ missing")
	}

	if !strings.HasPrefix(currentStep[0], "fourth-cleanup-job@") &&
		!strings.HasPrefix(currentStep[1], "fourth-cleanup-job@") {
		t.Fatalf("fourth-cleanup-job@ missing")
	}

	currentStep = getNextStep(g.Edges, currentStep)
	verifyStep(currentStep, 1, "sequence_decommission-cluster_end@", t)
}

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

func verifyStep(nodes []string, expectedCount int, expectedPrefix string, t *testing.T) {
	fmt.Println(nodes)
	if len(nodes) != expectedCount {
		t.Fatalf("%v: expected %d out edges, but got %d", nodes, expectedCount, len(nodes))
	}
	for _, n := range nodes {
		if !strings.HasPrefix(n, expectedPrefix) {
			t.Fatalf(" %v: unexpected node: %v, expecting: %s*", nodes, n, expectedPrefix)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////
func createGetClusterMembers(args map[string]interface{}) error {
	cluster, ok := args["cluster"]
	if !ok {
		return fmt.Errorf("job get-cluster-members expected a cluster arg")
	}
	if cluster != "test-cluster-001" {
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

func createEndNode(args map[string]interface{}) error {
	return nil
}
