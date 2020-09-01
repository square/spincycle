// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"
	"strings"

	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Grapher builds sequence graphs. A single node in a sequence graph (SG) represents
// a node spec, and edges indicate dependencies between nodes in the spec. Each
// sequence graphs describes a single sequence spec.
// Sequence, conditional, and expandable sequence nodes are represented by one
// node in a sequence graph, and are later resolved by the Resolver.
type Grapher struct {
	// User-provided
	allSequences map[string]*spec.Sequence // All sequences read in from request specs
	idGenFactory id.GeneratorFactory       // Generate per-graph unique IDs for nodes
}

func NewGrapher(specs spec.Specs, idGenFactory id.GeneratorFactory) *Grapher {
	return &Grapher{
		allSequences: specs.Sequences,
		idGenFactory: idGenFactory,
	}
}

// DoChecks performs graph checks for all sequences.
func (gr *Grapher) DoChecks() (seqGraphs map[string]*Graph, seqErrors map[string]error) {
	seqGraphs = map[string]*Graph{}
	seqSets := map[string]map[string]bool{}
	seqErrors = map[string]error{}

	// First, build all the sequence graphs.
	// Since sequence graphs are just a graph representation of dependencies
	// between a single sequence's nodes, this is per sequence; there's no
	// need to look at any subsequences.
	for seqName, seqSpec := range gr.allSequences {
		// Generates IDs unique within sequence graph
		idgen := gr.idGenFactory.Make()
		seqGraph, sets, err := buildSeqGraph(seqSpec, idgen)
		if err != nil {
			seqErrors[seqName] = err
			continue
		}
		seqGraphs[seqName] = seqGraph
		seqSets[seqName] = sets
	}

	// Now check that a sequence/conditional node's `sets` field only lists
	// job args that its component nodes set, i.e. that a subsequence
	// actually sets the job args declared in `sets`.
	// Only check graphs that built properly. If a graph failed to build,
	// we prioritize the build error, not this sets error.
	for seqName, _ := range seqGraphs {
		// seqName guaranteed to be in allSequences, since the set of
		// keys of seqGraphs is necessarily a subset of the set of keys of
		// allSequences
		seqSpec := gr.allSequences[seqName]

		missingSets := map[string][]string{} // node name -> list of missing `sets` args
		for nodeName, nodeSpec := range seqSpec.Nodes {
			if nodeSpec.IsJob() {
				continue
			}

			// Check that subsequences actually built
			// If any subsequence failed, we don't know what
			// args it sets
			subsequences := getSubsequences(nodeSpec, gr.allSequences)
			err := checkSubsequences(subsequences, seqGraphs)
			if err != nil {
				seqErrors[seqName] = err
				delete(seqGraphs, seqName)
				continue
			}

			// Compare declared `sets` with what the node
			// actually sets
			sets := getSets(subsequences, seqSets)
			missing := getMissingSets(sets, nodeSpec.Sets)
			if len(missing) != 0 {
				missingSets[nodeName] = missing
			}
		}

		// If any node failed the `sets` check, log it in seqErrors and remove the graph
		// from seqGraphs so the caller can't use it
		if len(missingSets) > 0 {
			msg := []string{}
			for nodeName, missing := range missingSets {
				msg = append(msg, fmt.Sprintf("%s (failed to set %s)", nodeName, strings.Join(missing, ", ")))
			}
			multiple := ""
			if len(missingSets) > 1 {
				multiple = "s"
			}
			seqErrors[seqName] = fmt.Errorf("node%s did not actually set job args declared in 'sets': %s", multiple, strings.Join(msg, "; "))
			delete(seqGraphs, seqName)
		}
	}

	return seqGraphs, seqErrors
}

// getSubsequences gets all subsequences called by a given node.
// For a sequence node, this is just the node's type.
// For a conditional node, this is all possible paths it can take.
func getSubsequences(nodeSpec *spec.Node, allSequences map[string]*spec.Sequence) []string {
	subseq := []string{}
	if nodeSpec.IsSequence() {
		subseq = append(subseq, *nodeSpec.NodeType)
	} else if nodeSpec.IsConditional() {
		for _, seq := range nodeSpec.Eq {
			//  add it only if it's a sequence
			if _, ok := allSequences[seq]; ok {
				subseq = append(subseq, seq)
			}
		}
	}
	return subseq
}

// checkSubsequences checks that all subsequences were built properly.
// It does not check that subsequences were specified in allSequences; static
// checks have already done this.
func checkSubsequences(subseqs []string, seqGraphs map[string]*Graph) error {
	missing := []string{}
	for _, subseq := range subseqs {
		if _, ok := seqGraphs[subseq]; !ok {
			missing = append(missing, subseq)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("subsequences failed to build: %s", strings.Join(missing, ", "))
	}
	return nil
}

// getSets gets all job args that the node described by `nodeSpec` is guaranteed to set.
// `seqSets` is a map of sequence name -> set of job args it sets
// For a sequence node, this computation of is straightforward--it's just the args it sets.
// For a conditional node, this is the intersection of the args set by the possible
// subsequences: a job arg mus tbe set no matter which conditional path was taken.
func getSets(subseqs []string, seqSets map[string]map[string]bool) map[string]bool {
	setsCount := map[string]int{} // output job arg --> # of subsequences that output it
	for _, seq := range subseqs {
		if subseqSets, ok := seqSets[seq]; ok {
			for arg, _ := range subseqSets {
				setsCount[arg]++
			}
		}
	}

	setsIntersection := map[string]bool{}
	for arg, count := range setsCount {
		if count == len(subseqs) {
			setsIntersection[arg] = true
		}
	}

	return setsIntersection
}

// getMissingSets returns a list of job args that are present in `declared` but not in `actual`.
// These are job args that were supposed to have been set, but were not.
func getMissingSets(actual map[string]bool, declared []*spec.NodeSet) []string {
	missing := []string{}
	for _, nodeSet := range declared {
		if !actual[*nodeSet.Arg] {
			missing = append(missing, *nodeSet.Arg)
		}
	}
	return missing
}

// Builds a sequence graph and performs checks on that sequence.
// It does not analyze any subsequences. As such, it can't e.g. check validity
// of `sets` declarations in sequence and conditional nodes.
// Returns the graph, the set of job args its component nodes set, and an error if any should occur.
func buildSeqGraph(seqSpec *spec.Sequence, idgen id.Generator) (seqGraph *Graph, sets map[string]bool, err error) {
	// The graph we'll be filling in
	seqGraph, err = newGraph(seqSpec.Name, idgen)
	if err != nil {
		return nil, nil, err
	}
	// newGraph doesn't manager `Order` for us; we'll have to maintain it ourselves in this function
	seqGraph.Order = append(seqGraph.Order, seqGraph.Source)

	// Set of job args available, i.e. sequence args + args set by nodes in graph so far
	jobArgs := getAllSequenceArgs(seqSpec)

	// Create a graph with a single node for every node in the spec
	// Key on node names--they should be unique within a sequence (otherwise, dependencies
	// are ill-defined)
	nodes := map[string]*Graph{}
	for _, nodeSpec := range seqSpec.Nodes {
		id, err := idgen.UID()
		if err != nil {
			return nil, nil, err
		}
		n := &Node{
			Id:   id,
			Name: nodeSpec.Name,
			Spec: nodeSpec,
		}
		nodes[nodeSpec.Name] = &Graph{
			Name:     n.Name,
			Source:   n,
			Sink:     n,
			Nodes:    map[string]*Node{n.Id: n},
			Edges:    map[string][]string{},
			RevEdges: map[string][]string{},
		}
	}

	// Nodes we've yet to add
	nodesToAdd := map[string]*Graph{}
	for k, v := range nodes {
		nodesToAdd[k] = v
	}
	nodesAdded := map[string]bool{}

	// Build graph by adding nodes, starting from the source node, and then
	// adding all adjacent nodes to the source node, and so on.
	// We cannot add nodes in any order because we do not know the reverse dependencies
	for len(nodesToAdd) > 0 {

		componentAdded := false

		for nodeName, component := range nodesToAdd {
			nodeSpec := seqSpec.Nodes[nodeName]
			if dependenciesSatisfied(nodesAdded, nodeSpec.Dependencies) {
				// Dependencies for node have been satisfied; presumably, all input job args are
				// present in job args map. If not, it's an error.
				missingArgs, err := getMissingArgs(nodeSpec, jobArgs)
				if err != nil {
					return nil, nil, err
				}
				if len(missingArgs) > 0 {
					return nil, nil, fmt.Errorf("node %s missing job args: %s", nodeSpec.Name, strings.Join(missingArgs, ", "))
				}

				// Insert component into graph
				if len(nodeSpec.Dependencies) == 0 {
					// Case: no dependencies; insert directly after start node
					err := seqGraph.InsertComponentBetween(component, seqGraph.Source, seqGraph.Sink)
					if err != nil {
						return nil, nil, err
					}
				} else {
					// Case: dependencies exist; insert between all its dependencies and the end node
					for _, dependencyName := range nodeSpec.Dependencies {
						prevComponent := nodes[dependencyName]
						err := seqGraph.InsertComponentBetween(component, prevComponent.Source, seqGraph.Sink)
						if err != nil {
							return nil, nil, err
						}
					}
				}

				// Add to topological ordering
				seqGraph.Order = append(seqGraph.Order, component.Source)

				// Update job args map
				for _, nodeSet := range nodeSpec.Sets {
					jobArgs[*nodeSet.As] = true
				}

				delete(nodesToAdd, nodeName)
				nodesAdded[nodeName] = true
				componentAdded = true
			}

		}

		// If we were unable to add nodes, there must be a cyclical dependency, which is an error
		if !componentAdded {
			ns := []string{}
			for n, _ := range nodesToAdd {
				ns = append(ns, n)
			}
			return nil, nil, fmt.Errorf("cyclical dependencies found amongst: %v", ns)
		}
	}

	// Append sink node to ordering
	seqGraph.Order = append(seqGraph.Order, seqGraph.Sink)

	// Make sure we haven't created a deformed graph
	// If we do, it's a bug in the code, not a problem with the specs
	if !seqGraph.IsValidGraph() {
		return nil, nil, fmt.Errorf("malformed graph created")
	}

	return seqGraph, jobArgs, nil
}

func newGraph(name string, idgen id.Generator) (*Graph, error) {
	id, err := idgen.UID()
	if err != nil {
		return nil, err
	}
	source := newNoopNode(name+"_start", id)

	id, err = idgen.UID()
	if err != nil {
		return nil, err
	}
	sink := newNoopNode(name+"_end", id)

	return &Graph{
		Name:   name,
		Source: source,
		Sink:   sink,

		Nodes:    map[string]*Node{source.Id: source, sink.Id: sink},
		Edges:    map[string][]string{source.Id: []string{sink.Id}},
		RevEdges: map[string][]string{sink.Id: []string{source.Id}},
		Order:    []*Node{},
	}, nil
}

func newNoopNode(name, id string) *Node {
	noopSpec := spec.NoopNode // copy
	noopSpec.Name = name
	return &Node{
		Id:   id,
		Name: name,
		Spec: &noopSpec,
	}
}

// Get the minimal set of job args that the sequence starts with.
// In the context of a wider request of which this sequence is a part, there may be more job
// args available, but this sequence should not access them, so they are irrelevant for our purposes.
func getAllSequenceArgs(seq *spec.Sequence) map[string]bool {
	jobArgs := map[string]bool{}
	for _, arg := range seq.Args.Required {
		jobArgs[*arg.Name] = true
	}
	for _, arg := range seq.Args.Optional {
		jobArgs[*arg.Name] = true
	}
	for _, arg := range seq.Args.Static {
		jobArgs[*arg.Name] = true
	}
	return jobArgs
}

// Check whether the set of nodes in graph (`inGraph`) satisfies all `dependencies`.
func dependenciesSatisfied(inGraph map[string]bool, dependencies []string) bool {
	for _, dep := range dependencies {
		if _, ok := inGraph[dep]; !ok {
			return false
		}
	}
	return true
}

// Returns a list of node args that aren't present in the job args map.
func getMissingArgs(n *spec.Node, jobArgs map[string]bool) ([]string, error) {
	missing := []string{}

	// Assert that the iterable variable is present.
	for _, each := range n.Each {
		if each == "" {
			continue
		}
		if len(strings.Split(each, ":")) != 2 { // this is malformed input
			return missing, fmt.Errorf("in node %s: malformed input to `each:`", n.Name)
		}
		iterateSet := strings.Split(each, ":")[0]
		if !jobArgs[iterateSet] {
			missing = append(missing, iterateSet)
		}
	}

	// Assert that the conditional variable is present.
	if n.If != nil {
		if !jobArgs[*n.If] {
			missing = append(missing, *n.If)
		}
	}

	// Assert all other defined args are present
	for _, arg := range n.Args {
		if !jobArgs[*arg.Given] {
			missing = append(missing, *arg.Given)
		}
	}

	return missing, nil
}
