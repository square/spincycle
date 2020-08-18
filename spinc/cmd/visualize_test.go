// Copyright 2020, Square, Inc.

package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/app"
	"github.com/square/spincycle/v2/spinc/cmd"
	"github.com/square/spincycle/v2/spinc/config"
	"github.com/square/spincycle/v2/test/mock"
)

// Used in all job chains (they don't care if there are extra jobs)
var visualizeJobs map[string]proto.Job

func visualizeSetup() {
	if visualizeJobs == nil {
		visualizeJobs = map[string]proto.Job{
			"start": proto.Job{
				Id:   "id-start",
				Name: "name-start",
				Type: "type-start",
			},
			"end": proto.Job{
				Id:   "id-end",
				Name: "name-end",
				Type: "type-end",
			},
		}
		for i := 0; i < 20; i++ {
			str := fmt.Sprintf("%d", i)
			visualizeJobs[str] = proto.Job{
				Id:   "id-" + str,
				Name: "name-" + str,
				Type: "type-" + str,
			}
		}
	}
}

func TestVisualizeReqArgs(t *testing.T) {
	output := &bytes.Buffer{}
	request := proto.Request{
		Type:  "request-type",
		State: proto.STATE_COMPLETE,
		Args: []proto.RequestArg{
			proto.RequestArg{
				Name:  "required-arg",
				Type:  "required",
				Value: 304,
			},
			proto.RequestArg{
				Name:  "static-arg",
				Type:  "static", // static args shouldn't be output (they're always the same)
				Value: "three oh four",
			},
			proto.RequestArg{
				Name:  "optional-arg",
				Type:  "optional",
				Value: []int{3, 0, 4},
			},
		},
	}
	expected := ` REQUEST TYPE: request-type
          ARG: required-arg=304
          ARG: optional-arg=[3 0 4]
     START |->
     END ->| COMPLETE
`

	rmc := &mock.RMClient{
		GetRequestFunc: func(id string) (proto.Request, error) {
			return request, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command: config.Command{
			Args: []string{"id"},
		},
	}
	visualize := cmd.NewVisualize(ctx)
	err := visualize.Prepare()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}
	err = visualize.Run()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}

	if output.String() != expected {
		t.Errorf("Expected output (see code) differs from actual output:\n%s", output.String())
	}
}

func TestVisualizeSimple(t *testing.T) {
	visualizeSetup()

	output := &bytes.Buffer{}

	// start -> 0 -> 1 -> 2 -> end
	adjList := map[string][]string{
		"start": []string{"0"},
		"0":     []string{"1"},
		"1":     []string{"2"},
		"2":     []string{"end"},
	}
	expected := ` REQUEST TYPE: 
     START |->
   UNKNOWN |->name-start (type-start) [id-start]
   UNKNOWN |  name-0 (type-0) [id-0]
   UNKNOWN |  name-1 (type-1) [id-1]
   UNKNOWN |  name-2 (type-2) [id-2]
   UNKNOWN |  name-end (type-end) [id-end]
     END ->| UNKNOWN
`

	rmc := &mock.RMClient{
		GetJobChainFunc: func(id string) (proto.JobChain, error) {
			return proto.JobChain{
				Jobs:          visualizeJobs,
				AdjacencyList: adjList,
			}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command: config.Command{
			Args: []string{"id", "--no-color"},
		},
	}
	visualize := cmd.NewVisualize(ctx)
	err := visualize.Prepare()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}
	err = visualize.Run()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}

	if output.String() != expected {
		t.Errorf("Expected output (see code) differs from actual output:\n%s", output.String())
	}
}

func TestVisualizeBranch(t *testing.T) {
	visualizeSetup()

	output := &bytes.Buffer{}

	//              -> 1 --> 2 -
	//             /            \
	// start --> 0 --> 3 --> 4 --> 7 --> end
	//             \            /
	//              -> 5 --> 6 -
	adjList := map[string][]string{
		"start": []string{"0"},
		"0":     []string{"1", "3", "5"},
		"1":     []string{"2"},
		"3":     []string{"4"},
		"5":     []string{"6"},
		"2":     []string{"7"},
		"4":     []string{"7"},
		"6":     []string{"7"},
		"7":     []string{"end"},
	}
	expected := ` REQUEST TYPE: 
     START |->
   UNKNOWN |->name-start (type-start) [id-start]
   UNKNOWN |  name-0 (type-0) [id-0]
   UNKNOWN |  |  |->name-1 (type-1) [id-1]
   UNKNOWN |  |  |  name-2 (type-2) [id-2]
   UNKNOWN |  |  |->name-3 (type-3) [id-3]
   UNKNOWN |  |  |  name-4 (type-4) [id-4]
   UNKNOWN |  |  |->name-5 (type-5) [id-5]
   UNKNOWN |  |  |  name-6 (type-6) [id-6]
   UNKNOWN |  name-7 (type-7) [id-7]
   UNKNOWN |  name-end (type-end) [id-end]
     END ->| UNKNOWN
`

	rmc := &mock.RMClient{
		GetJobChainFunc: func(id string) (proto.JobChain, error) {
			return proto.JobChain{
				Jobs:          visualizeJobs,
				AdjacencyList: adjList,
			}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command: config.Command{
			Args: []string{"id", "--no-color"},
		},
	}
	visualize := cmd.NewVisualize(ctx)
	err := visualize.Prepare()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}
	err = visualize.Run()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}

	if output.String() != expected {
		t.Errorf("Expected output (see code) differs from actual output:\n%s", output.String())
	}
}

// TODO
func testVisualizeMerge(t *testing.T) {
	visualizeSetup()

	output := &bytes.Buffer{}

	//              -> 1 --> 2 -
	//             /            \
	// start --> 0 --> 3 --> 4 --> 5 --> 9 --> end
	//             \                  /
	//              -> 6 --> 7 --> 8 -
	adjList := map[string][]string{
		"start": []string{"0"},
		"0":     []string{"1", "3", "6"},
		"1":     []string{"2"},
		"2":     []string{"5"},
		"3":     []string{"4"},
		"4":     []string{"5"},
		"5":     []string{"9"},
		"6":     []string{"7"},
		"7":     []string{"8"},
		"8":     []string{"9"},
		"9":     []string{"end"},
	}
	expected := ""

	rmc := &mock.RMClient{
		GetJobChainFunc: func(id string) (proto.JobChain, error) {
			return proto.JobChain{
				Jobs:          visualizeJobs,
				AdjacencyList: adjList,
			}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command: config.Command{
			Args: []string{"id", "--no-color"},
		},
	}
	visualize := cmd.NewVisualize(ctx)
	err := visualize.Prepare()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}
	err = visualize.Run()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}

	if output.String() != expected {
		t.Errorf("Expected output (see code) differs from actual output:\n%s", output.String())
	}
}

// TODO: this shouldn't be pushed either (maybe?)
func TestVisualize1(t *testing.T) {
	visualizeSetup()

	output := &bytes.Buffer{}

	adjList := map[string][]string{
		"start": []string{"0"},
		"0":     []string{"3", "1"},
		"1":     []string{"2"},
		"2":     []string{"17"},
		"3":     []string{"4", "5"},
		"4":     []string{"6"},
		"5":     []string{"6"},
		"6":     []string{"7", "11", "12"},
		"7":     []string{"8", "9"},
		"8":     []string{"10"},
		"9":     []string{"10"},
		"10":    []string{"13", "14"},
		"11":    []string{"13", "14"},
		"12":    []string{"14"},
		"13":    []string{"2", "15"},
		"14":    []string{"16"},
		"15":    []string{"17"},
		"16":    []string{"17"},
		"17":    []string{"18"},
		"18":    []string{"end"},
	}
	expected := ` REQUEST TYPE: 
     START |->
   UNKNOWN |->name-start (type-start) [id-start]
   UNKNOWN |  name-0 (type-0) [id-0]
   UNKNOWN |  |->name-1 (type-1) [id-1]
   UNKNOWN |  |->name-3 (type-3) [id-3]
   UNKNOWN |  |  |->name-4 (type-4) [id-4]
   UNKNOWN |  |  |->name-5 (type-5) [id-5]
   UNKNOWN |  |  name-6 (type-6) [id-6]
   UNKNOWN |  |  |  |->name-7 (type-7) [id-7]
   UNKNOWN |  |  |  |  |->name-8 (type-8) [id-8]
   UNKNOWN |  |  |  |  |->name-9 (type-9) [id-9]
   UNKNOWN |  |  |  |  name-10 (type-10) [id-10]
   UNKNOWN |  |  |  |->name-11 (type-11) [id-11]
   UNKNOWN |  |  |  |  name-13 (type-13) [id-13]
   UNKNOWN |  |  |  |  |  name-2 (type-2) [id-2]
   UNKNOWN |  |  |  |  |  |->name-15 (type-15) [id-15]
   UNKNOWN |  |  |  |->name-12 (type-12) [id-12]
   UNKNOWN |  name-14 (type-14) [id-14]
   UNKNOWN |  name-16 (type-16) [id-16]
   UNKNOWN |  name-17 (type-17) [id-17]
   UNKNOWN |  name-18 (type-18) [id-18]
   UNKNOWN |  name-end (type-end) [id-end]
     END ->| UNKNOWN
`

	rmc := &mock.RMClient{
		GetJobChainFunc: func(id string) (proto.JobChain, error) {
			return proto.JobChain{
				Jobs:          visualizeJobs,
				AdjacencyList: adjList,
			}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command: config.Command{
			Args: []string{"id", "--no-color"},
		},
	}
	visualize := cmd.NewVisualize(ctx)
	err := visualize.Prepare()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}
	err = visualize.Run()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}

	if output.String() != expected {
		t.Errorf("Expected output (see code) differs from actual output:\n%s", output.String())
	}
}

// TODO: L don't push this okay? okay
// If this ends up on remote it's L's fault, you can yell at them
func TestVisualize0(t *testing.T) {
	testDir := "/Users/laurenl/Documents/visualizer-test/"

	output := &bytes.Buffer{}
	rmc := &mock.RMClient{
		GetJobChainFunc: func(id string) (proto.JobChain, error) {
			var jc proto.JobChain
			data, err := ioutil.ReadFile(testDir + "jc.json")
			if err != nil {
				return jc, err
			}
			err = json.Unmarshal(data, &jc)
			if err != nil {
				return jc, err
			}
			return jc, nil
		},
		GetRequestFunc: func(id string) (proto.Request, error) {
			var req proto.Request
			data, err := ioutil.ReadFile(testDir + "req.json")
			if err != nil {
				return req, err
			}
			err = json.Unmarshal(data, &req)
			if err != nil {
				return req, err
			}
			return req, nil
		},
		GetJLFunc: func(id string) ([]proto.JobLog, error) {
			var jl []proto.JobLog
			data, err := ioutil.ReadFile(testDir + "log.json")
			if err != nil {
				return jl, err
			}
			err = json.Unmarshal(data, &jl)
			if err != nil {
				return jl, err
			}
			return jl, nil
		},
		RunningFunc: func(f proto.StatusFilter) (proto.RunningStatus, error) {
			return proto.RunningStatus{}, nil
		},
	}
	ctx := app.Context{
		Out:      output,
		RMClient: rmc,
		Command: config.Command{
			Args: []string{"id"},
		},
	}
	visualize := cmd.NewVisualize(ctx)
	err := visualize.Prepare()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}
	err = visualize.Run()
	if err != nil {
		t.Fatalf("got err '%s', expected nil", err)
	}

	expected, err := ioutil.ReadFile(testDir + "comp.txt")
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != bytes.NewBuffer(expected).String() {
		t.Errorf("Expected output (see %scomp.txt) differs from actual output:\n%s", testDir, output.String())
	}
}
