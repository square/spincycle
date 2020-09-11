// Copyright 2017-2020, Square, Inc.

package graph

import (
	"fmt"
	"strings"

	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Grapher builds sequence graphs. A single node in a sequence graph represents
// a node spec, and edges indicate dependencies between nodes in the spec. Each
// sequence graphs describes a single sequence spec.
// Sequence, conditional, and expandable sequence nodes correspond to a single
// node in a sequence graph, and are later resolved into a request subgraph
// consisting of only job nodes by the Resolver.
type Grapher struct {
	// User-provided
	sequenceSpecs map[string]*spec.Sequence // All sequences read in from request specs
	idGenFactory  id.GeneratorFactory       // Generator of per-graph unique IDs for nodes
}

// TODO: Grapher may soon have fields that need to be initialized, e.g. it might store
// the sequence graph map, and we want it to start off non-nil.
func NewGrapher(specs spec.Specs, idGenFactory id.GeneratorFactory) *Grapher {
	return &Grapher{
		sequenceSpecs: specs.Sequences,
		idGenFactory:  idGenFactory,
	}
}

// CheckSequences performs graph checks for all sequences and returns a map of
// sequence name -> sequence graph and a map of sequence name --> error. The two
// return values are mutually exclusive; if any error occurs, it is logged in the
// errors map, and the sequence graph map is nil. Else, all sequences have a
// corresponding entry in the sequence graph map, and the error map is nil.
func (gr *Grapher) CheckSequences() (seqGraphs map[string]*Graph, seqErrors map[string]error) {
	seqGraphs = map[string]*Graph{}
	seqErrors = map[string]error{}

	// sequence name -> set of job args that its nodes declare in `sets` field
	// i.e. the job args that the sequence sets
	seqSets := map[string]map[string]bool{}

	// First, build all the sequence graphs.
	// Since sequence graphs are just a graph representation of dependencies
	// between a single sequence's nodes, this is contained to one sequence;
	// this won't look at any subsequences.
	for seqName, seqSpec := range gr.sequenceSpecs {
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
	// job args that the subsequence(s) actually sets and that there are no
	// circular dependencies among sequences.
	// We need to do these in order, starting from sequences that don't call
	// any subsequences, then sequences that call only subsequences we've
	// already examined, etc.
	// We use the same algorithm as we do for building sequence graphs node
	// by node, except we don't actually build a graph.

	// Get "dependencies" for all sequences that built successfully.
	subsequences := map[string][]string{}
	for seqName, _ := range seqGraphs {
		seqSpec := gr.sequenceSpecs[seqName]
		subseqMap := map[string]bool{}

		for _, nodeSpec := range seqSpec.Nodes {
			if nodeSpec.IsJob() {
				continue
			}
			subseqs := getNodeSubsequences(nodeSpec, gr.sequenceSpecs)
			for _, s := range subseqs {
				subseqMap[s] = true
			}
		}
		for s, _ := range subseqMap {
			subsequences[seqName] = append(subsequences[seqName], s)
		}
	}

	// Keep checking sequences whose dependencies are satisfied until we can't
	// anymore, either because we've checked them all, or because impossible
	// dependencies exist.
	seqsChecked := map[string]bool{}
	seqsToCheck := map[string]*spec.Sequence{}
	for k, v := range gr.sequenceSpecs {
		seqsToCheck[k] = v
	}

	for len(seqsToCheck) != 0 {
		newSeqChecked := false

		for seqName, seqSpec := range seqsToCheck {
			if haveAllDeps(seqsChecked, subsequences[seqName]) {
				newSeqChecked = true
				delete(seqsToCheck, seqName)
				seqsChecked[seqName] = true

				// Only perform sets check if this sequence built
				// and all its subsequences have passed all checks.
				if _, ok := seqErrors[seqName]; ok {
					continue
				}
				failed := getFailedSubsequences(subsequences[seqName], seqErrors)
				if len(failed) != 0 {
					multiple := ""
					if len(failed) > 1 {
						multiple = "s"
					}
					seqErrors[seqName] = fmt.Errorf("subsequence%s failed checks: %s", multiple, strings.Join(failed, ", "))
					continue
				}

				// Do sets check.
				missingSets := map[string][]string{} // node name -> list of missing `sets` args
				for nodeName, nodeSpec := range seqSpec.Nodes {
					if nodeSpec.IsJob() {
						continue
					}
					// Compare declared `sets` with what the node
					// actually sets
					subseqs := getNodeSubsequences(nodeSpec, gr.sequenceSpecs)
					sets := getActualSets(subseqs, seqSets)
					missing := getMissingSets(sets, nodeSpec.Sets)
					if len(missing) != 0 {
						missingSets[nodeName] = missing
					}
				}
				if len(missingSets) > 0 {
					msg := []string{}
					for nodeName, missing := range missingSets {
						msg = append(msg, fmt.Sprintf("%s (failed to set %s)", nodeName, strings.Join(missing, ", ")))
					}
					multiple := ""
					if len(missingSets) > 1 {
						multiple = "s"
					}
					seqErrors[seqName] = fmt.Errorf("node%s did not set job args declared in 'sets': %s", multiple, strings.Join(msg, "; "))
				}
			}
		}

		// If we were unable to check any sequences, there must be a
		// cyclical dependency.
		if !newSeqChecked {
			for seqName, _ := range seqsToCheck {
				// This overwrites a build error, if there was one.
				// We want all sequences that were part of the
				// cyclical dependency to have this error; otherwise,
				// we imply that some sequence wasn't part of the
				// cyclical dependency when it actually was.
				seqErrors[seqName] = fmt.Errorf("part of cyclical dependency among sequences")
			}
			break
		}
	}

	if len(seqErrors) != 0 {
		return nil, seqErrors
	}

	return seqGraphs, nil
}

// getNodeSubsequences gets all subsequences called by a given node.
// For a sequence node, this is just the node's type.
// For a conditional node, this is all possible paths it can take.
func getNodeSubsequences(nodeSpec *spec.Node, sequenceSpecs map[string]*spec.Sequence) []string {
	subsequences := []string{}
	if nodeSpec.IsSequence() {
		subsequences = append(subsequences, *nodeSpec.NodeType)
	} else if nodeSpec.IsConditional() {
		for _, seq := range nodeSpec.Eq {
			//  add it only if it's a sequence
			if _, ok := sequenceSpecs[seq]; ok {
				subsequences = append(subsequences, seq)
			}
		}
	}
	return subsequences
}

// getFailedSubsequences returns a list of subsequences that didn't build,
// i.e. don't show up in the seqErrors map.
// It does not check that subsequences were specified in sequenceSpecs.
func getFailedSubsequences(subsequences []string, seqErrors map[string]error) []string {
	failed := []string{}
	for _, subseq := range subsequences {
		if _, ok := seqErrors[subseq]; ok {
			failed = append(failed, subseq)
		}
	}
	return failed
}

// getActualSets gets all job args that the node described by `nodeSpec` is
// guaranteed to set.
// `seqSets` is a map of sequence name -> set of job args it sets.
// For a sequence node, this computation of is straightforward--it's just the
// args it sets.
// For a conditional node, this is the intersection of the args set by the
// possible subsequences: all job args must be set no matter which conditional
// path was taken.
func getActualSets(subseqs []string, seqSets map[string]map[string]bool) map[string]bool {
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

// getMissingSets returns a list of job args that are present in `declared` but
// not in `actual`.
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

// buildSeqGraph builds a sequence graph and performs checks on that sequence.
// It does not analyze any subsequences. As such, it can't e.g. check validity
// of `sets` declarations in sequence and conditional nodes.
// Returns the graph, the set of job args its component nodes set, and an error
// if any should occur.
func buildSeqGraph(seqSpec *spec.Sequence, idgen id.Generator) (seqGraph *Graph, sets map[string]bool, err error) {
	// The graph we'll be filling in
	seqGraph, err = newSeqGraph(seqSpec.Name, idgen)
	if err != nil {
		return nil, nil, err
	}

	// We have to maintain `Order` ourselves
	seqGraph.Order = append(seqGraph.Order, seqGraph.Source)

	// Set of job args available, i.e. sequence args + args set by nodes in
	// graph so far.
	// We start out with just the sequence args.
	jobArgs := getAllSequenceArgs(seqSpec)

	// Create graph nodes for every node in the sequence spec. We're not connecting
	// the nodes yet (i.e. not building a graph), just initializing all the graph
	// nodes. In the next loop, we'll wire them up (i.e. build the sequence graph).
	//
	// Note: graph nodes can be any type (job, seq, or conditional), but for
	// sequence graphs we don't do anything (i.e. don't create job.Job for job node);
	// a Resolver does that when creating a request graph (RG). Sequence graphs
	// are only (graph) structure and metadata about that structure.
	//
	// Key on node names; they should be unique within a sequence (otherwise,
	// dependencies are ill-defined).
	nodes := map[string]*Graph{}
	nodesToAdd := map[string]*Graph{} // Nodes we've yet to add
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
		g := &Graph{
			Name:     n.Name,
			Source:   n,
			Sink:     n,
			Nodes:    map[string]*Node{n.Id: n},
			Edges:    map[string][]string{},
			RevEdges: map[string][]string{},
		}
		nodes[nodeSpec.Name] = g
		nodesToAdd[nodeSpec.Name] = g
	}

	nodesAdded := map[string]bool{}

	// Build graph by adding nodes, starting from the source node, and then
	// adding all adjacent nodes to the source node, and so on.
	// Nodes are not specified in any particular order in the specs; the
	// only ordering information we have is from the `deps` field for each
	// node. So, we continuously loop through the spec nodes, and add it to
	// the graph only once all its dependencies have been added.
	for len(nodesToAdd) > 0 {

		// True iff we find a new node to add in this iteration. Assume
		// false until we find such a node.
		nodeAdded := false

		// Each loop through the nodes, we should be able to add at least
		// one node. If not, the there's a cycle (or perhaps a typo in
		// deps: that wasn't caught earlier). For example, with A->B->C,
		// the loop will add A since it has no deps, then it'll add B since
		// it depends on A which has been built, then C. But if a cycle
		// existed between B and C, the second loop wouldn't be able to
		// build B and nodeAdded would be false and trigger the error
		// after this loop.
		for nodeName, node := range nodesToAdd {
			nodeSpec := seqSpec.Nodes[nodeName]
			if !haveAllDeps(nodesAdded, nodeSpec.Dependencies) {
				continue
			}

			// Dependencies for node have been satisfied;
			// presumably, all input job args are present in
			// job args map. If not, it's an error.
			err := checkNodeArgs(nodeSpec, jobArgs)
			if err != nil {
				return nil, nil, err
			}

			// Insert node into graph
			if len(nodeSpec.Dependencies) == 0 {
				// Case: no dependencies; insert directly after
				// the source node. No nodes that depend on this
				// one have been added yet, so we can put it right
				// before the sink node.
				err := seqGraph.InsertComponentBetween(node, seqGraph.Source, seqGraph.Sink)
				if err != nil {
					return nil, nil, err
				}
			} else {
				// Case: dependencies exist; insert between all its
				// dependencies and the sink node, since no nodes
				// depending on this one have been added yet.
				for _, dependencyName := range nodeSpec.Dependencies {
					prevComponent := nodes[dependencyName]
					err := seqGraph.InsertComponentBetween(node, prevComponent.Sink, seqGraph.Sink)
					if err != nil {
						return nil, nil, err
					}
				}
			}

			// Add this node to topological ordering
			seqGraph.Order = append(seqGraph.Order, node.Source)

			// Update job args map
			for _, nodeSet := range nodeSpec.Sets {
				jobArgs[*nodeSet.As] = true
			}

			delete(nodesToAdd, nodeName)
			nodesAdded[nodeName] = true
			nodeAdded = true
		}

		// If we were unable to add nodes on this iteration, there must
		// be a cyclical dependency, which is an error.
		if !nodeAdded {
			ns := []string{}
			for n, _ := range nodesToAdd {
				ns = append(ns, n)
			}
			return nil, nil, fmt.Errorf("cyclical dependencies found amongst: %v", ns)
		}
	}

	seqGraph.Order = append(seqGraph.Order, seqGraph.Sink)

	// Make sure we haven't created a deformed graph
	// If we do, it's a bug in the code, not a problem with the specs
	if err := seqGraph.IsValidGraph(); err != nil {
		return nil, nil, fmt.Errorf("sequence graph for sequence %s is not a valid directed acyclic graph: %s", seqSpec.Name, err)
	}

	return seqGraph, jobArgs, nil
}

func newSeqGraph(name string, idgen id.Generator) (*Graph, error) {
	id, err := idgen.UID()
	if err != nil {
		return nil, err
	}
	source := newNoopSeqNode(name+"_begin", id)

	id, err = idgen.UID()
	if err != nil {
		return nil, err
	}
	sink := newNoopSeqNode(name+"_end", id)

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

func newNoopSeqNode(name, id string) *Node {
	noopSpec := spec.NoopNode // copy
	noopSpec.Name = name
	return &Node{
		Id:   id,
		Name: name,
		Spec: &noopSpec,
	}
}

// getAllSequenceArgs returns all sequence args, i.e. required+optional+static.
// This is the minimal set of job args that the sequence starts with.
// In the context of a wider request of which this sequence is a part, there may
// be more job args available, but we only permit a sequence to access the job
// args that were declared in its sequence args, so the undeclared job args are
// irrelevant for our purposes.
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

// haveAllDeps checks whether the set of nodes in a graph
// satisfies all dependencies.
func haveAllDeps(inGraph map[string]bool, dependencies []string) bool {
	for _, dep := range dependencies {
		if _, ok := inGraph[dep]; !ok {
			return false
		}
	}
	return true
}

// checkNodeArgs checks whether all node args are present in the job args map.
func checkNodeArgs(n *spec.Node, jobArgs map[string]bool) error {
	// If this is a conditional node, assert that the "if" job arg is present
	// in the job args map.
	// Static checks assert that for conditional nodes, if != nil.
	if n.IsConditional() {
		if !jobArgs[*n.If] {
			return fmt.Errorf("in node %s: 'if: %s': job arg %s is not set", n.Name, *n.If, *n.If)
		}
	}

	missing := []string{}

	// Assert that the iterable variable is present
	for _, each := range n.Each {
		if each == "" {
			continue
		}
		if len(strings.Split(each, ":")) != 2 {
			return fmt.Errorf("in node %s: input to 'each:' %s; expected format 'list:element'", n.Name, each)
		}
		iterateSet := strings.Split(each, ":")[0]
		if !jobArgs[iterateSet] {
			missing = append(missing, iterateSet)
		}
	}

	// Assert all other defined args are present
	for _, arg := range n.Args {
		if !jobArgs[*arg.Given] {
			missing = append(missing, *arg.Given)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("node %s: expected job args to be set by some previous node: %s", n.Name, strings.Join(missing, ", "))
	}

	return nil
}
