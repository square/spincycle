// Copyright 2017-2020, Square, Inc.

package spec

import (
	"fmt"
	"strings"
	"time"
)

type NodeCheck interface {
	CheckNode(string, NodeSpec) error
}

/* ========================================================================== */
type HasCategoryNodeCheck struct{}

/* Nodes must specify a category. */
func (check HasCategoryNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.Category == nil {
		return MissingValueError{sequenceName, &node.Name, "category", ""}
	}

	return nil
}

/* ========================================================================== */
type ValidCategoryNodeCheck struct{}

/* `category: (job | sequence | conditional)` */
func (check ValidCategoryNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if !node.isJob() && !node.isSequence() && !node.isConditional() {
		return InvalidValueError{sequenceName, &node.Name, "category", *node.Category, "(job | sequence | conditional)"}
	}

	return nil
}

/* ========================================================================== */
type ValidEachNodeCheck struct{}

/* `each` values must be in the format `arg:alias`. */
func (check ValidEachNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	for _, each := range node.Each {
		if p := len(strings.Split(each, ":")); p != 2 {
			return InvalidValueError{sequenceName, &node.Name, "each", each, "value of form `arg:alias`"}
		}
	}

	return nil
}

/* ========================================================================== */
type ArgsAreNamedNodeCheck struct{}

/* Node `args` must be named, i.e. include an `expected`field. */
func (check ArgsAreNamedNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	for _, nodeArg := range node.Args {
		if nodeArg.Expected == nil {
			return MissingValueError{sequenceName, &node.Name, "expected", "required when `arg/given` field set"}
		}
	}

	return nil
}

/* ========================================================================== */
type ArgsOnceNodeCheck struct{}

/* Args cannot be expected multiple times by one job. */
func (check ArgsOnceNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	seen := map[string]bool{}
	for _, nodeSet := range node.Args {
		if nodeSet.Expected == nil {
			continue
		}
		if seen[*nodeSet.Expected] {
			return DuplicateValueError{sequenceName, &node.Name, "args", *nodeSet.Expected}
		}
		seen[*nodeSet.Expected] = true
	}

	return nil
}

/* ========================================================================== */
type ArgsNotRenamedNodeCheck struct{}

/* A single arg cannot be renamed and used as multiple inputs to a node. */
func (check ArgsNotRenamedNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	aliases := map[string]string{}
	for _, nodeSet := range node.Args {
		if nodeSet.Given == nil || nodeSet.Expected == nil {
			continue
		}
		if alias, ok := aliases[*nodeSet.Given]; ok && alias != *nodeSet.Expected {
			return DuplicateValueError{sequenceName, &node.Name, "args/given", *nodeSet.Given}
		}
		aliases[*nodeSet.Given] = *nodeSet.Expected
	}

	return nil
}

/* ========================================================================== */
type SetsAreNamedNodeCheck struct{}

/* Node `sets` must be named, i.e. include an `arg`field. */
func (check SetsAreNamedNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	for _, nodeSet := range node.Sets {
		if nodeSet.Arg == nil {
			return MissingValueError{sequenceName, &node.Name, "arg", "required when `sets/as` field set"}
		}
	}

	return nil
}

/* ========================================================================== */
type SetsOnceNodeCheck struct{}

/* Args cannot be set multiple times by one job. */
func (check SetsOnceNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	seen := map[string]bool{}
	for _, nodeSet := range node.Sets {
		if nodeSet.As == nil {
			continue
		}
		if seen[*nodeSet.As] {
			return DuplicateValueError{sequenceName, &node.Name, "sets", *nodeSet.As}
		}
		seen[*nodeSet.As] = true
	}

	return nil
}

/* ========================================================================== */
type SetsNotRenamedNodeCheck struct{}

/* A single `sets` arg cannot be renamed. */
func (check SetsNotRenamedNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	aliases := map[string]string{}
	for _, nodeSet := range node.Sets {
		if nodeSet.Arg == nil || nodeSet.As == nil {
			continue
		}
		if alias, ok := aliases[*nodeSet.Arg]; ok && alias != *nodeSet.As {
			return DuplicateValueError{sequenceName, &node.Name, "sets/arg", *nodeSet.Arg}
		}
		aliases[*nodeSet.Arg] = *nodeSet.As
	}

	return nil
}

/* ========================================================================== */
type EachIfParallelNodeCheck struct{}

/* If `parallel` is set, `each` must be set. */
func (check EachIfParallelNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	/* `parallel` > 0. */
	if node.Parallel != nil {
		if node.Each == nil {
			return MissingValueError{sequenceName, &node.Name, "each", "required when `parallel` field set"}
		}
	}

	return nil
}

/* ========================================================================== */
type ValidParallelNodeCheck struct{}

/* `parallel` > 0. */
func (check ValidParallelNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.Parallel != nil {
		if *node.Parallel == 0 {
			return InvalidValueError{sequenceName, &node.Name, "parallel", "0", "> 0"}
		}
	}

	return nil
}

/* ========================================================================== */
type ConditionalNoTypeNodeCheck struct{}

/* Conditional nodes may not specify a type. */
func (check ConditionalNoTypeNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.isConditional() {
		if node.NodeType != nil {
			return InvalidValueError{sequenceName, &node.Name, "type", *node.NodeType, "no value; conditional nodes may not specify a type"}
		}
	}

	return nil
}

/* ========================================================================== */
type ConditionalHasIfNodeCheck struct{}

/* `Conditional nodes must specify `if`. */
func (check ConditionalHasIfNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.isConditional() {
		if node.If == nil {
			return MissingValueError{sequenceName, &node.Name, "if", "required for conditional nodes"}
		}
	}

	return nil
}

/* ========================================================================== */
type ConditionalHasEqNodeCheck struct{}

/* Conditional nodes must specify `eq`. */
func (check ConditionalHasEqNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.isConditional() {
		if len(node.Eq) == 0 {
			return MissingValueError{sequenceName, &node.Name, "eq", "required for conditional nodes"}
		}
	}

	return nil
}

/* ========================================================================== */
type NonconditionalHasTypeNodeCheck struct{}

/* Nonconditional nodes must specify a type. */
func (check NonconditionalHasTypeNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if !node.isConditional() {
		if node.NodeType == nil {
			return MissingValueError{sequenceName, &node.Name, "type", "required for nonconditional nodes"}
		}
	}

	return nil
}

/* ========================================================================== */
type NonconditionalNoIfNodeCheck struct{}

/* Nononditional nodes may not specify `if`. */
func (check NonconditionalNoIfNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if !node.isConditional() {
		if node.If != nil {
			return InvalidValueError{sequenceName, &node.Name, "if", *node.If, "no value; noncoditional nodes may not specify if"}
		}
	}

	return nil
}

/* ========================================================================== */
type NonconditionalNoEqNodeCheck struct{}

/* Nononditional nodes may not specify `eq`. */
func (check NonconditionalNoEqNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if !node.isConditional() {
		if len(node.Eq) != 0 {
			eq := fmt.Sprintf("%v", node.Eq)
			return InvalidValueError{sequenceName, &node.Name, "eq", eq, "no value; noncoditional nodes may not specify eq"}
		}
	}

	return nil
}

/* ========================================================================== */
type RetryIfRetryWaitNodeCheck struct{}

/* If `retryWait` is set, `retry` must be set (nonzero). */
func (check RetryIfRetryWaitNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.RetryWait != "" && node.Retry == 0 {
		return MissingValueError{sequenceName, &node.Name, "retry", "required when `retryWait` field set"}
	}

	return nil
}

/* ========================================================================== */
type ValidRetryWaitNodeCheck struct{}

/* `retryWait` should be a valid duration. */
func (check ValidRetryWaitNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	if node.RetryWait != "" {
		if _, err := time.ParseDuration(node.RetryWait); err != nil {
			return InvalidValueError{sequenceName, &node.Name, "retryWait", node.RetryWait, "valid duration string"}
		}
	}

	return nil
}

/* ========================================================================== */
type RequiredArgsProvidedNodeCheck struct {
	specs Specs
}

/* Sequence and conditional nodes must be provided with their required args. */
func (check RequiredArgsProvidedNodeCheck) CheckNode(sequenceName string, node NodeSpec) error {
	// List of sequences to check
	// (This will also include jobs, but we'll ignore them later)
	var sequences []string
	if node.NodeType != nil {
		sequences = []string{*node.NodeType}
	}
	for _, seq := range node.Eq {
		sequences = append(sequences, seq)
	}

	// Set of all (declared) inputs to a node
	var declaredArgs = map[string]bool{}
	for _, nodeArg := range node.Args {
		if nodeArg.Expected != nil {
			declaredArgs[*nodeArg.Expected] = true
		}
	}
	for _, each := range node.Each {
		p := strings.Split(each, ",")
		if len(p) != 2 {
			continue
		}
		declaredArgs[p[1]] = true
	}

	// Check that all required args are present
	for _, sequence := range sequences {
		seq, ok := check.specs.Sequences[sequence]
		if !ok {
			continue
		}
		for _, reqArg := range seq.Args.Required {
			if reqArg.Name == nil {
				continue
			}
			if _, ok = declaredArgs[*reqArg.Name]; !ok {
				return MissingValueError{sequenceName, &node.Name, "args", fmt.Sprintf("required arg %s not given", *reqArg.Name)}
			}

		}
	}

	return nil
}
