package grapher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/square/spincycle/job"
)

type testFactory struct{}

func (f testFactory) Make(t, n string) (job.Job, error) {
	j := testJob{
		name:      n,
		jobtype:   t,
		givenArgs: map[string]interface{}{},
	}
	return j, nil
}

type testJob struct {
	name      string
	jobtype   string
	givenArgs map[string]interface{}
}

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

	tj.givenArgs = map[string]interface{}{}
	for k, v := range args {
		tj.givenArgs[k] = v
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
	return NewGrapher(tf, cfg)
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

	for name, node := range g.Vertices {
		// Verify that noop nodes do not have Args.
		if strings.HasPrefix(name, "sequence_") || strings.HasPrefix(name, "repeat_") {
			if len(node.Args) != 0 {
				t.Errorf("node %s args = %#v, expected an empty map", name, node.Args)
			}
		}

		// Check the Args on some nodes.
		if strings.HasPrefix(name, "get-instances@") {
			expectedArgs := map[string]interface{}{
				"cluster": "test-cluster-001",
			}
			if diff := deep.Equal(node.Args, expectedArgs); diff != nil {
				t.Error(diff)
			}
		}
		if strings.HasPrefix(name, "prep-1@") {
			expectedArgs := map[string]interface{}{
				"cluster":   "test-cluster-001",
				"env":       "testing",
				"instances": []string{"node1", "node2", "node3", "node4"},
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
	for name, node := range g.Vertices {
		if strings.HasPrefix(name, "get-instances@") {
			found = true
			if node.Retry != 3 {
				t.Errorf("%s node retries = %d, expected %d", name, node.Retry, 3)
			}
			if node.RetryWait != 10 {
				t.Errorf("%s node retry delay = %d, expected %d", name, node.RetryWait, 10)
			}
		} else {
			if node.Retry != 0 {
				t.Errorf("%s node retries = %d, expected %d", name, node.Retry, 0)
			}
			if node.RetryWait != 0 {
				t.Errorf("%s node retry delay = %d, expected %d", name, node.RetryWait, 0)
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

// //////////////////////////////////////////////////////////////////////////
// Graph tests
// //////////////////////////////////////////////////////////////////////////

// three nodes in a straight line
func g1() *Graph {
	tf := testFactory{}
	n1, _ := tf.Make("g1n1", "g1n1")
	n2, _ := tf.Make("g1n2", "g1n2")
	n3, _ := tf.Make("g1n3", "g1n3")
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
	tf := testFactory{}
	n1, _ := tf.Make("g3n1", "g3n1")
	n2, _ := tf.Make("g3n2", "g3n2")
	n3, _ := tf.Make("g3n3", "g3n3")
	n4, _ := tf.Make("g3n4", "g3n4")
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
	tf := testFactory{}
	n := [20]*Node{}
	for i := 0; i < 20; i++ {
		m := fmt.Sprintf("g2n%d", i)
		p, _ := tf.Make(m, m)
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
