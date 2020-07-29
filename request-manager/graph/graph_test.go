// Copyright 2017-2020, Square, Inc.

package graph

//// three nodes in a straight line
//func g1() *Graph {
//	tf := &testFactory{}
//	n1, _ := tf.Make(job.NewIdWithRequestId("g1n1", "g1n1", "g1n1", "reqId1"))
//	n2, _ := tf.Make(job.NewIdWithRequestId("g1n2", "g1n2", "g1n2", "reqId1"))
//	n3, _ := tf.Make(job.NewIdWithRequestId("g1n3", "g1n3", "g1n3", "reqId1"))
//	g1n1 := &Node{Datum: n1}
//	g1n2 := &Node{Datum: n2}
//	g1n3 := &Node{Datum: n3}
//
//	g1n1.Prev = map[string]*Node{}
//	g1n1.Next = map[string]*Node{"g1n2": g1n2}
//	g1n2.Next = map[string]*Node{"g1n3": g1n3}
//	g1n2.Prev = map[string]*Node{"g1n1": g1n1}
//	g1n3.Prev = map[string]*Node{"g1n2": g1n2}
//	g1n3.Next = map[string]*Node{}
//
//	return &Graph{
//		Name:  "test1",
//		First: g1n1,
//		Last:  g1n3,
//		Vertices: map[string]*Node{
//			"g1n1": g1n1,
//			"g1n2": g1n2,
//			"g1n3": g1n3,
//		},
//		Edges: map[string][]string{
//			"g1n1": []string{"g1n2"},
//			"g1n2": []string{"g1n3"},
//		},
//	}
//}
//
//// 4 node diamond graph
////       2
////     /   \
//// -> 1      4 ->
////     \   /
////       3
////
////
//func g3() *Graph {
//	tf := &testFactory{}
//	n1, _ := tf.Make(job.NewIdWithRequestId("g3n1", "g3n1", "g3n1", "reqId1"))
//	n2, _ := tf.Make(job.NewIdWithRequestId("g3n2", "g3n2", "g3n2", "reqId1"))
//	n3, _ := tf.Make(job.NewIdWithRequestId("g3n3", "g3n3", "g3n3", "reqId1"))
//	n4, _ := tf.Make(job.NewIdWithRequestId("g3n4", "g3n4", "g3n4", "reqId1"))
//	g3n1 := &Node{Datum: n1}
//	g3n2 := &Node{Datum: n2}
//	g3n3 := &Node{Datum: n3}
//	g3n4 := &Node{Datum: n4}
//
//	g3n1.Prev = map[string]*Node{}
//	g3n1.Next = map[string]*Node{"g3n2": g3n2, "g3n3": g3n3}
//	g3n2.Next = map[string]*Node{"g3n4": g3n4}
//	g3n3.Next = map[string]*Node{"g3n4": g3n4}
//	g3n4.Prev = map[string]*Node{"g3n2": g3n2, "g3n3": g3n3}
//	g3n2.Prev = map[string]*Node{"g3n1": g3n1}
//	g3n3.Prev = map[string]*Node{"g3n1": g3n1}
//	g3n4.Next = map[string]*Node{}
//
//	return &Graph{
//		Name:  "test1",
//		First: g3n1,
//		Last:  g3n4,
//		Vertices: map[string]*Node{
//			"g3n1": g3n1,
//			"g3n2": g3n2,
//			"g3n3": g3n3,
//			"g3n4": g3n4,
//		},
//		Edges: map[string][]string{
//			"g3n1": []string{"g3n2", "g3n3"},
//			"g3n2": []string{"g3n4"},
//			"g3n3": []string{"g3n4"},
//		},
//	}
//}
//
//// 18 node graph of something
//func g2() *Graph {
//	tf := &testFactory{}
//	n := [20]*Node{}
//	for i := 0; i < 20; i++ {
//		m := fmt.Sprintf("g2n%d", i)
//		p, _ := tf.Make(job.NewIdWithRequestId(m, m, m, "reqId1"))
//		n[i] = &Node{
//			Datum: p,
//			Next:  map[string]*Node{},
//			Prev:  map[string]*Node{},
//		}
//	}
//
//	// yeah good luck following this
//	n[0].Next["g2n1"] = n[1]
//	n[1].Next["g2n2"] = n[2]
//	n[1].Next["g2n5"] = n[5]
//	n[1].Next["g2n6"] = n[6]
//	n[2].Next["g2n3"] = n[3]
//	n[2].Next["g2n4"] = n[4]
//	n[3].Next["g2n7"] = n[7]
//	n[4].Next["g2n8"] = n[8]
//	n[7].Next["g2n8"] = n[8]
//	n[8].Next["g2n13"] = n[13]
//	n[13].Next["g2n14"] = n[14]
//	n[5].Next["g2n9"] = n[9]
//	n[9].Next["g2n12"] = n[12]
//	n[12].Next["g2n14"] = n[14]
//	n[6].Next["g2n10"] = n[10]
//	n[10].Next["g2n11"] = n[11]
//	n[10].Next["g2n19"] = n[19]
//	n[11].Next["g2n12"] = n[12]
//	n[19].Next["g2n16"] = n[16]
//	n[16].Next["g2n15"] = n[15]
//	n[16].Next["g2n17"] = n[17]
//	n[15].Next["g2n18"] = n[18]
//	n[17].Next["g2n18"] = n[18]
//	n[18].Next["g2n14"] = n[14]
//	n[1].Prev["g2n0"] = n[0]
//	n[2].Prev["g2n1"] = n[1]
//	n[3].Prev["g2n2"] = n[2]
//	n[4].Prev["g2n2"] = n[2]
//	n[5].Prev["g2n1"] = n[1]
//	n[6].Prev["g2n1"] = n[1]
//	n[7].Prev["g2n3"] = n[3]
//	n[8].Prev["g2n4"] = n[4]
//	n[8].Prev["g2n7"] = n[7]
//	n[9].Prev["g2n5"] = n[5]
//	n[10].Prev["g2n6"] = n[6]
//	n[11].Prev["g2n10"] = n[10]
//	n[12].Prev["g2n9"] = n[9]
//	n[12].Prev["g2n11"] = n[11]
//	n[13].Prev["g2n8"] = n[8]
//	n[14].Prev["g2n12"] = n[12]
//	n[14].Prev["g2n13"] = n[13]
//	n[14].Prev["g2n18"] = n[18]
//	n[15].Prev["g2n16"] = n[16]
//	n[16].Prev["g2n19"] = n[19]
//	n[17].Prev["g2n16"] = n[16]
//	n[18].Prev["g2n15"] = n[15]
//	n[18].Prev["g2n17"] = n[17]
//	n[19].Prev["g2n10"] = n[10]
//
//	return &Graph{
//		Name:  "g2",
//		First: n[0],
//		Last:  n[14],
//		Vertices: map[string]*Node{
//			"g2n0":  n[0],
//			"g2n1":  n[1],
//			"g2n2":  n[2],
//			"g2n3":  n[3],
//			"g2n4":  n[4],
//			"g2n5":  n[5],
//			"g2n6":  n[6],
//			"g2n7":  n[7],
//			"g2n8":  n[8],
//			"g2n9":  n[9],
//			"g2n10": n[10],
//			"g2n11": n[11],
//			"g2n12": n[12],
//			"g2n13": n[13],
//			"g2n14": n[14],
//			"g2n15": n[15],
//			"g2n16": n[16],
//			"g2n17": n[17],
//			"g2n18": n[18],
//			"g2n19": n[19],
//		},
//		Edges: map[string][]string{
//			"g2n0":  []string{"g2n1"},
//			"g2n1":  []string{"g2n2", "g2n5", "g2n6"},
//			"g2n2":  []string{"g2n3", "g2n4"},
//			"g2n3":  []string{"g2n7"},
//			"g2n4":  []string{"g2n8"},
//			"g2n5":  []string{"g2n9"},
//			"g2n6":  []string{"g2n10"},
//			"g2n7":  []string{"g2n8"},
//			"g2n8":  []string{"g2n13"},
//			"g2n9":  []string{"g2n12"},
//			"g2n10": []string{"g2n11", "g2n19"},
//			"g2n11": []string{"g2n12"},
//			"g2n12": []string{"g2n14"},
//			"g2n13": []string{"g2n14"},
//			"g2n15": []string{"g2n18"},
//			"g2n16": []string{"g2n15", "g2n17"},
//			"g2n17": []string{"g2n18"},
//			"g2n18": []string{"g2n14"},
//			"g2n19": []string{"g2n16"},
//		},
//	}
//}
//
//func TestCreateAdjacencyList1(t *testing.T) {
//	g := g1()
//	edges, vertices := g.createAdjacencyList()
//	// check that all the vertex lists match
//	for vertexName, node := range vertices {
//		if n, ok := g.Vertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing %v", n)
//		}
//	}
//	for vertexName, node := range g.Vertices {
//		if n, ok := vertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing %v", n)
//		}
//	}
//	for source, sinks := range edges {
//		if e := g.Edges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing %s -> %v", source, sinks)
//		}
//	}
//	for source, sinks := range g.Edges {
//		if e := edges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing %s -> %v", source, sinks)
//		}
//	}
//}
//func TestCreateAdjacencyList2(t *testing.T) {
//	g := g2()
//	edges, vertices := g.createAdjacencyList()
//	// check that all the vertex lists match
//	for vertexName, node := range vertices {
//		if n, ok := g.Vertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing %v", n)
//		}
//	}
//	for vertexName, node := range g.Vertices {
//		if n, ok := vertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing %v", n)
//		}
//	}
//	for source, sinks := range edges {
//		if e := g.Edges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing %s -> %v", source, sinks)
//		}
//	}
//	for source, sinks := range g.Edges {
//		if e := edges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing %s -> %v", source, sinks)
//		}
//	}
//}
//func TestCreateAdjacencyList3(t *testing.T) {
//	g := g3()
//	edges, vertices := g.createAdjacencyList()
//	// check that all the vertex lists match
//	for vertexName, node := range vertices {
//		if n, ok := g.Vertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing %v", n)
//		}
//	}
//	for vertexName, node := range g.Vertices {
//		if n, ok := vertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing %v", n)
//		}
//	}
//	for source, sinks := range edges {
//		if e := g.Edges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing %s -> %v", source, sinks)
//		}
//	}
//	for source, sinks := range g.Edges {
//		if e := edges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing %s -> %v", source, sinks)
//		}
//	}
//}
//
//func TestInsertComponentBetween1(t *testing.T) {
//	g1 := g1()
//	g3 := g3()
//	// insert g1 into g3 between nodes 2 -> 4
//	err := g3.insertComponentBetween(g1, g3.Vertices["g3n2"], g3.Vertices["g3n4"])
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	expectedVertices := map[string]*Node{
//		"g1n1": g1.Vertices["g1n1"],
//		"g1n2": g1.Vertices["g1n2"],
//		"g1n3": g1.Vertices["g1n3"],
//		"g3n1": g3.Vertices["g3n1"],
//		"g3n2": g3.Vertices["g3n2"],
//		"g3n3": g3.Vertices["g3n3"],
//		"g3n4": g3.Vertices["g3n4"],
//	}
//	expectedEdges := map[string][]string{
//		"g1n1": []string{"g1n2"},
//		"g1n2": []string{"g1n3"},
//		"g1n3": []string{"g3n4"},
//
//		"g3n1": []string{"g3n2", "g3n3"},
//		"g3n2": []string{"g1n1"},
//		"g3n3": []string{"g3n4"},
//	}
//
//	actualEdges, actualVertices := g3.createAdjacencyList()
//	for vertexName, node := range actualVertices {
//		if n, ok := expectedVertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing1 %v", n)
//		}
//	}
//	for vertexName, node := range expectedVertices {
//		if n, ok := actualVertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing2 %v", vertexName)
//		}
//	}
//	for source, sinks := range actualEdges {
//		if e := expectedEdges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing3 %s -> %v", source, sinks)
//		}
//	}
//	for source, sinks := range expectedEdges {
//		if e := actualEdges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing4 %s -> %v", source, sinks)
//		}
//	}
//}
//
//func TestInsertComponentBetween2(t *testing.T) {
//	g1 := g1()
//	g3 := g3()
//	// insert g1 into g3 between nodes 3 -> 4
//	err := g3.insertComponentBetween(g1, g3.Vertices["g3n3"], g3.Vertices["g3n4"])
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	expectedVertices := map[string]*Node{
//		"g1n1": g1.Vertices["g1n1"],
//		"g1n2": g1.Vertices["g1n2"],
//		"g1n3": g1.Vertices["g1n3"],
//		"g3n1": g3.Vertices["g3n1"],
//		"g3n2": g3.Vertices["g3n2"],
//		"g3n3": g3.Vertices["g3n3"],
//		"g3n4": g3.Vertices["g3n4"],
//	}
//	expectedEdges := map[string][]string{
//		"g1n1": []string{"g1n2"},
//		"g1n2": []string{"g1n3"},
//		"g1n3": []string{"g3n4"},
//
//		"g3n1": []string{"g3n2", "g3n3"},
//		"g3n2": []string{"g3n4"},
//		"g3n3": []string{"g1n1"},
//	}
//
//	actualEdges, actualVertices := g3.createAdjacencyList()
//	for vertexName, node := range actualVertices {
//		if n, ok := expectedVertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing1 %v", n)
//		}
//	}
//	for vertexName, node := range expectedVertices {
//		if n, ok := actualVertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing2 %v", vertexName)
//		}
//	}
//	for source, sinks := range actualEdges {
//		if e := expectedEdges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing3 %s -> %v", source, sinks)
//		}
//	}
//	for source, sinks := range expectedEdges {
//		if e := actualEdges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing4 %s -> %v", source, sinks)
//		}
//	}
//}
//
//func TestInsertComponentBetween3(t *testing.T) {
//	g1 := g1()
//	g3 := g3()
//	// insert g1 into g3 between nodes 1 -> 2
//	err := g3.insertComponentBetween(g1, g3.Vertices["g3n1"], g3.Vertices["g3n2"])
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	expectedVertices := map[string]*Node{
//		"g1n1": g1.Vertices["g1n1"],
//		"g1n2": g1.Vertices["g1n2"],
//		"g1n3": g1.Vertices["g1n3"],
//		"g3n1": g3.Vertices["g3n1"],
//		"g3n2": g3.Vertices["g3n2"],
//		"g3n3": g3.Vertices["g3n3"],
//		"g3n4": g3.Vertices["g3n4"],
//	}
//	expectedEdges := map[string][]string{
//		"g1n1": []string{"g1n2"},
//		"g1n2": []string{"g1n3"},
//		"g1n3": []string{"g3n2"},
//
//		"g3n1": []string{"g1n1", "g3n3"},
//		"g3n2": []string{"g3n4"},
//		"g3n3": []string{"g3n4"},
//	}
//
//	actualEdges, actualVertices := g3.createAdjacencyList()
//	for vertexName, node := range actualVertices {
//		if n, ok := expectedVertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing1 %v", n)
//		}
//	}
//	for vertexName, node := range expectedVertices {
//		if n, ok := actualVertices[vertexName]; !ok || n != node {
//			t.Fatalf("missing2 %v", vertexName)
//		}
//	}
//	for source, sinks := range actualEdges {
//		if e := expectedEdges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing3 %s -> %v", source, sinks)
//		}
//	}
//	for source, sinks := range expectedEdges {
//		if e := actualEdges[source]; !slicesMatch(e, sinks) {
//			t.Fatalf("missing4 %s -> %v", source, sinks)
//		}
//	}
//}
//
//func TestOptArgs001(t *testing.T) {
//	sequencesFile := "../test/specs/opt-args-001.yaml"
//	tf := &mock.JobFactory{
//		Created: map[string]*mock.Job{},
//	}
//	s, err := spec.ParseSpec(sequencesFile, func(s string, a ...interface{}) {})
//	if err != nil {
//		t.Fatal(err)
//	}
//	g := NewCreator(req, tf, s, id.NewGenerator(4, 100))
//
//	args := map[string]interface{}{
//		"cmd":  "sleep",
//		"args": "3",
//	}
//	got, err := g.CreateGraph("req", args)
//	if err != nil {
//		t.Fatal(err)
//	}
//	// Find the node we want
//	var j *Node
//	for _, node := range got.Vertices {
//		if node.Name == "job1name" {
//			j = node
//		}
//	}
//	if j == nil {
//		t.Logf("%#v", got.Vertices)
//		t.Fatal("graph.Vertices[job1name] not set")
//	}
//	if diff := deep.Equal(j.Args, args); diff != nil {
//		t.Logf("%#v\n", j.Args)
//		t.Error(diff)
//	}
//	job := tf.Created[j.Datum.Id().Name]
//	if job == nil {
//		t.Fatal("job job1name not created")
//	}
//	if diff := deep.Equal(job.CreatedWithArgs, args); diff != nil {
//		t.Logf("%#v\n", job)
//		t.Errorf("test job not created with args arg")
//	}
//
//	// Try again without "args", i.e. the optional arg is not given
//	delete(args, "args")
//
//	tf = &mock.JobFactory{
//		Created: map[string]*mock.Job{},
//	}
//	sequencesFile = "../test/specs/opt-args-001.yaml"
//	s, err = spec.ParseSpec(sequencesFile, func(s string, a ...interface{}) {})
//	if err != nil {
//		t.Fatal(err)
//	}
//	g = NewCreator(req, tf, s, id.NewGenerator(4, 100))
//
//	got, err = g.CreateGraph("req", args)
//	if err != nil {
//		t.Fatal(err)
//	}
//	// Find the node we want
//	for _, node := range got.Vertices {
//		if node.Name == "job1name" {
//			j = node
//		}
//	}
//	if j == nil {
//		t.Logf("%#v", got.Vertices)
//		t.Fatal("graph.Vertices[job1name] not set")
//	}
//	if diff := deep.Equal(j.Args, args); diff != nil {
//		t.Logf("%#v\n", j.Args)
//		t.Error(diff)
//	}
//	job = tf.Created[j.Datum.Id().Name]
//	if job == nil {
//		t.Fatal("job job1name not created")
//	}
//	if _, ok := job.CreatedWithArgs["args"]; !ok {
//		t.Error("jobArgs[args] does not exist, expected it to be set")
//	}
//	if diff := deep.Equal(job.CreatedWithArgs, args); diff != nil {
//		t.Logf("%#v\n", job)
//		t.Errorf("test job not created with args arg")
//	}
//}
//
//func TestBadEach001(t *testing.T) {
//	sequencesFile := "../test/specs/bad-each-001.yaml"
//	job := &mock.Job{
//		SetJobArgs: map[string]interface{}{
//			// This causes an error because the spec has each: instances:instannce,
//			// so instances should be a slice, but it's a string.
//			"instances": "foo",
//		},
//	}
//	tf := &mock.JobFactory{
//		MockJobs: map[string]*mock.Job{
//			"get-instances": job,
//		},
//	}
//	s, err := spec.ParseSpec(sequencesFile, func(s string, a ...interface{}) {})
//	if err != nil {
//		t.Fatal(err)
//	}
//	g := NewCreator(req, tf, s, id.NewGenerator(4, 100))
//
//	args := map[string]interface{}{
//		"host": "foo",
//	}
//	_, err = g.CreateGraph("bad-each", args)
//	// Should get "each:instances:instance: arg instances is a string, expected a slice"
//	if err == nil {
//		t.Error("err is nil, expected an error")
//	}
//}
//
//func TestAuthSpec(t *testing.T) {
//	// Test that the acl: part of a request is parsed properly
//	got, err := ReadConfig("../test/specs/auth-001.yaml")
//	if err != nil {
//		t.Fatal(err)
//	}
//	expect := Config{
//		Sequences: map[string]*SequenceSpec{
//			"req1": &SequenceSpec{
//				Name:    "req1",
//				Request: true,
//				Args: SequenceArgs{
//					Required: []*ArgSpec{
//						{
//							Name: "arg1",
//							Desc: "required arg 1",
//						},
//					},
//				},
//				ACL: []ACL{
//					{
//						Role:  "role1",
//						Admin: true,
//					},
//					{
//						Role:  "role2",
//						Admin: false,
//						Ops:   []string{"start", "stop"},
//					},
//				},
//				Nodes: map[string]*NodeSpec{
//					"node1": &NodeSpec{
//						Name:     "node1",
//						Category: "job",
//						NodeType: "job1type",
//					},
//				},
//			},
//		},
//	}
//	if diff := deep.Equal(got, expect); diff != nil {
//		t.Error(diff)
//	}
//}
//
//func TestConditionalIfOptionalArg(t *testing.T) {
//	// This spec has "if: foo" where "foo" is an optional arg with no value,
//	// so creator should use "default: defaultSeq", which we can see below in
//	// "sequence_defaultSeq_start/end" nodes.
//	sequencesFile := "../test/specs/cond-args-001.yaml"
//	specs, err := spec.ParseSpec(sequencesFile, func(s string, a ...interface{}) {})
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Mock ID gen so we get known numbering
//	idNo := 0
//	idgen := mock.IDGenerator{
//		UIDFunc: func() (string, error) {
//			idNo++
//			return fmt.Sprintf("id%d", idNo), nil
//		},
//	}
//
//	gr := NewCreator(req, &testFactory{}, specs, idgen)
//	args := map[string]interface{}{
//		"cmd": "cmd-val",
//	}
//	got, err := gr.CreateGraph("request-name", args)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Partial nodes from the spec. Just want to verify the name and sequence IDs
//	// are what we expect, and the order expressed by Edges. This let's us see/verify
//	// that defaultSeq is created.
//	id2 := &Node{
//		Name:       "sequence_request-name_end",
//		SequenceId: "id1",
//	}
//	id7 := &Node{
//		Name:       "conditional_job1name_end",
//		SequenceId: "id1",
//	}
//	id4 := &Node{
//		Name:       "sequence_defaultSeq_end",
//		SequenceId: "id3",
//	}
//	id5 := &Node{ // category: job, type: job1
//		Name:       "job1name",
//		SequenceId: "id3",
//	}
//	id3 := &Node{
//		Name:       "sequence_defaultSeq_start",
//		SequenceId: "id3",
//	}
//	id6 := &Node{
//		Name:       "conditional_job1name_start",
//		SequenceId: "id1",
//	}
//	id1 := &Node{
//		Name:       "sequence_request-name_start",
//		SequenceId: "id1",
//	}
//	verticies := map[string]*Node{
//		"id1": id1,
//		"id2": id2,
//		"id3": id3,
//		"id4": id4,
//		"id5": id5,
//		"id6": id6,
//		"id7": id7,
//	}
//	expect := &Graph{
//		//Name:     "sequence_request-name",
//		//First:    verticies["id1"],
//		//Last:     verticies["id7"],
//		//Vertices: verticies,
//		Edges: map[string][]string{
//			"id1": []string{"id6"},
//			"id6": []string{"id3"},
//			"id3": []string{"id5"},
//			"id5": []string{"id4"},
//			"id4": []string{"id7"},
//			"id7": []string{"id2"},
//		},
//	}
//	if diff := deep.Equal(got.Edges, expect.Edges); diff != nil {
//		t.Logf("   got: %#v", got.Edges)
//		t.Logf("expect: %#v", expect.Edges)
//		t.Error(diff)
//	}
//	for k, v := range got.Vertices {
//		t.Logf("vertex: %+v", v)
//		if v.Name != verticies[k].Name {
//			t.Errorf("node '%s'.Name = %s, expected %s", k, v.Name, verticies[k].Name)
//		}
//		if v.SequenceId != verticies[k].SequenceId {
//			t.Errorf("node '%s'.SequenceId = %s, expected %s", k, v.SequenceId, verticies[k].SequenceId)
//		}
//	}
//}
