// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"strings"
	"time"
)

type NodeCheck interface {
	CheckNode(string, Node) error
}

/* ========================================================================== */
type HasCategoryNodeCheck struct{}

/* Nodes must specify a category. */
func (check HasCategoryNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.Category == nil {
		return MissingValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "category",
			Explanation: "",
		}
	}

	return nil
}

/* ========================================================================== */
type ValidCategoryNodeCheck struct{}

/* `category: (job | sequence | conditional)` */
func (check ValidCategoryNodeCheck) CheckNode(sequenceName string, node Node) error {
	if !node.IsJob() && !node.IsSequence() && !node.IsConditional() {
		return InvalidValueError{
			Sequence: sequenceName,
			Node:     &node.Name,
			Field:    "category",
			Values:   []string{*node.Category},
			Expected: "(job | sequence | conditional)",
		}
	}

	return nil
}

/* ========================================================================== */
type ValidEachNodeCheck struct{}

/* `each` values must be in the format `arg:alias`. */
func (check ValidEachNodeCheck) CheckNode(sequenceName string, node Node) error {
	var values []string
	for _, each := range node.Each {
		split := len(strings.Split(each, ":"))
		if split != 2 {
			values = append(values, each)
		}
	}

	if len(values) > 0 {
		return InvalidValueError{
			Sequence: sequenceName,
			Node:     &node.Name,
			Field:    "each",
			Values:   values,
			Expected: "value(s) of form `arg:alias`",
		}
	}

	return nil
}

/* ========================================================================== */
type EachAliasUniqueNodeCheck struct{}

/* Each cannot repeat aliases. */
func (check EachAliasUniqueNodeCheck) CheckNode(sequenceName string, node Node) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, each := range node.Each {
		split := strings.Split(each, ":")
		if len(split) != 2 {
			continue
		}
		alias := split[1]
		if seen[alias] {
			values[alias] = true
		}
		seen[alias] = true
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "each",
			Values:      stringSetToArray(values),
			Explanation: "in `each: arg:alias`, a given `alias` should appear at most once per node",
		}
	}

	return nil
}

/* ========================================================================== */
type EachNotRenamedTwiceNodeCheck struct{}

/* Each cannot rename its args */
func (check EachNotRenamedTwiceNodeCheck) CheckNode(sequenceName string, node Node) error {
	aliases := map[string]string{}
	values := map[string]bool{}
	for _, each := range node.Each {
		split := strings.Split(each, ":")
		if len(split) != 2 {
			continue
		}
		arg := split[0]
		alias := split[1]
		if existingAlias, ok := aliases[arg]; ok && alias != existingAlias {
			values[arg] = true
		}
		aliases[arg] = alias
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "each",
			Values:      stringSetToArray(values),
			Explanation: "in `each: arg:alias`, a given `arg` value should appear at most once per node",
		}
	}

	return nil
}

/* ========================================================================== */
type ArgsAreNamedNodeCheck struct{}

/* Node `args` must be named, i.e. include an `expected` field. */
func (check ArgsAreNamedNodeCheck) CheckNode(sequenceName string, node Node) error {
	for _, nodeArg := range node.Args {
		if nodeArg.Expected == nil {
			return MissingValueError{
				Sequence:    sequenceName,
				Node:        &node.Name,
				Field:       "args.expected",
				Explanation: "must be specified for all `args`",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type ArgsExpectedUniqueNodeCheck struct{}

/* Args cannot be expected multiple times by one job. */
func (check ArgsExpectedUniqueNodeCheck) CheckNode(sequenceName string, node Node) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, nodeSet := range node.Args {
		if nodeSet.Expected == nil {
			continue
		}
		if seen[*nodeSet.Expected] {
			values[*nodeSet.Expected] = true
		}
		seen[*nodeSet.Expected] = true
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "args.expected",
			Values:      stringSetToArray(values),
			Explanation: "",
		}
	}

	return nil
}

/* ========================================================================== */
type ArgsNotRenamedTwiceNodeCheck struct{}

/* A single arg cannot be renamed and used as multiple inputs to a node. */
// Example of a rename:
// - expected: x
//   given: foo
// - expected: y
//   given: foo
func (check ArgsNotRenamedTwiceNodeCheck) CheckNode(sequenceName string, node Node) error {
	aliases := map[string]string{}
	values := map[string]bool{}
	for _, nodeSet := range node.Args {
		if nodeSet.Given == nil || nodeSet.Expected == nil {
			continue
		}
		if alias, ok := aliases[*nodeSet.Given]; ok && alias != *nodeSet.Expected {
			values[*nodeSet.Given] = true
		}
		aliases[*nodeSet.Given] = *nodeSet.Expected
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "args.given",
			Values:      stringSetToArray(values),
			Explanation: "note that if `given` is not explicitly specified, then its value is the same as `expected`",
		}
	}

	return nil
}

/* ========================================================================== */
type EachAliasDoesNotDuplicateArgsExpectedNodeCheck struct{}

/* `alias` in `each` cannot share a name as an `expected` job arg. */
func (check EachAliasDoesNotDuplicateArgsExpectedNodeCheck) CheckNode(sequenceName string, node Node) error {
	expected := map[string]bool{} // All expected args
	for _, nodeSet := range node.Args {
		if nodeSet.Expected == nil {
			continue
		}
		expected[*nodeSet.Expected] = true
	}

	values := map[string]bool{}
	for _, each := range node.Each {
		split := strings.Split(each, ":")
		if len(split) != 2 {
			continue
		}
		alias := split[1]
		if expected[alias] {
			values[alias] = true
		}
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "each",
			Values:      stringSetToArray(values),
			Explanation: "in `each: arg:alias`, a given `alias` should not be repeated in `args.expected`",
		}
	}

	return nil
}

/* ========================================================================== */
type EachArgDoesNotDuplicateArgsGivenNodeCheck struct{}

/* `each` cannot take a job arg as an arg if it a `given` job arg. */
func (check EachArgDoesNotDuplicateArgsGivenNodeCheck) CheckNode(sequenceName string, node Node) error {
	given := map[string]bool{} // All given args
	for _, nodeSet := range node.Args {
		if nodeSet.Given == nil || nodeSet.Expected == nil {
			continue
		}
		given[*nodeSet.Given] = true
	}

	values := map[string]bool{}
	for _, each := range node.Each {
		split := strings.Split(each, ":")
		if len(split) != 2 {
			continue
		}
		arg := split[0]
		if given[arg] {
			values[arg] = true
		}
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "each",
			Values:      stringSetToArray(values),
			Explanation: "in `each: arg:alias`, a given `arg` value should not be repeated in `args.given`; note that if `given` is not explicitly specified, then its value is the same as `expected`",
		}
	}

	return nil
}

/* ========================================================================== */
type SetsAreNamedNodeCheck struct{}

/* Node `sets` must be named, i.e. include an `arg`field. */
func (check SetsAreNamedNodeCheck) CheckNode(sequenceName string, node Node) error {
	for _, nodeSet := range node.Sets {
		if nodeSet.Arg == nil {
			return MissingValueError{
				Sequence:    sequenceName,
				Node:        &node.Name,
				Field:       "sets.arg",
				Explanation: "must be specified for all `sets`",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type SetsAsUniqueNodeCheck struct{}

/* Args cannot be set multiple times by one job. */
func (check SetsAsUniqueNodeCheck) CheckNode(sequenceName string, node Node) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, nodeSet := range node.Sets {
		if nodeSet.As == nil {
			continue
		}
		if seen[*nodeSet.As] {
			values[*nodeSet.As] = true
		}
		seen[*nodeSet.As] = true
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "sets.as",
			Values:      stringSetToArray(values),
			Explanation: "note that if `as` is not explicitly specified, then its value is the same as `args`",
		}
	}

	return nil
}

/* ========================================================================== */
type SetsNotRenamedTwiceNodeCheck struct{}

/* A single `sets` arg cannot be renamed. */
func (check SetsNotRenamedTwiceNodeCheck) CheckNode(sequenceName string, node Node) error {
	aliases := map[string]string{}
	values := map[string]bool{}
	for _, nodeSet := range node.Sets {
		if nodeSet.Arg == nil || nodeSet.As == nil {
			continue
		}
		if alias, ok := aliases[*nodeSet.Arg]; ok && alias != *nodeSet.As {
			values[*nodeSet.Arg] = true
		}
		aliases[*nodeSet.Arg] = *nodeSet.As
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "sets.arg",
			Values:      stringSetToArray(values),
			Explanation: "value(s) renamed multiple times using `as`",
		}
	}

	return nil
}

/* ========================================================================== */
type EachIfParallelNodeCheck struct{}

/* If `parallel` is set, `each` must be set. */
func (check EachIfParallelNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.Parallel != nil {
		if node.Each == nil {
			return MissingValueError{
				Sequence:    sequenceName,
				Node:        &node.Name,
				Field:       "each",
				Explanation: "required when `parallel` field set",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type ValidParallelNodeCheck struct{}

/* `parallel` > 0. */
func (check ValidParallelNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.Parallel != nil {
		if *node.Parallel == 0 {
			return InvalidValueError{
				Sequence: sequenceName,
				Node:     &node.Name,
				Field:    "parallel",
				Values:   []string{"0"},
				Expected: "> 0",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type ConditionalNoTypeNodeCheck struct{}

/* Conditional nodes may not specify a type. */
func (check ConditionalNoTypeNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.IsConditional() {
		if node.NodeType != nil {
			return InvalidValueError{
				Sequence: sequenceName,
				Node:     &node.Name,
				Field:    "type",
				Values:   []string{*node.NodeType},
				Expected: "no value; conditional nodes may not specify a type",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type ConditionalHasIfNodeCheck struct{}

/* `Conditional nodes must specify `if`. */
func (check ConditionalHasIfNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.IsConditional() {
		if node.If == nil {
			return MissingValueError{
				Sequence:    sequenceName,
				Node:        &node.Name,
				Field:       "if",
				Explanation: "required for conditional nodes",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type ConditionalHasEqNodeCheck struct{}

/* Conditional nodes must specify `eq`. */
func (check ConditionalHasEqNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.IsConditional() {
		if len(node.Eq) == 0 {
			return MissingValueError{
				Sequence:    sequenceName,
				Node:        &node.Name,
				Field:       "eq",
				Explanation: "required for conditional nodes",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type NonconditionalHasTypeNodeCheck struct{}

/* Nonconditional nodes must specify a type. */
func (check NonconditionalHasTypeNodeCheck) CheckNode(sequenceName string, node Node) error {
	if !node.IsConditional() {
		if node.NodeType == nil {
			return MissingValueError{
				Sequence:    sequenceName,
				Node:        &node.Name,
				Field:       "type",
				Explanation: "required for nonconditional nodes",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type NonconditionalNoIfNodeCheck struct{}

/* Nononditional nodes may not specify `if`. */
func (check NonconditionalNoIfNodeCheck) CheckNode(sequenceName string, node Node) error {
	if !node.IsConditional() {
		if node.If != nil {
			return InvalidValueError{
				Sequence: sequenceName,
				Node:     &node.Name,
				Field:    "if",
				Values:   []string{*node.If},
				Expected: "no value; noncoditional nodes may not specify if",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type NonconditionalNoEqNodeCheck struct{}

/* Nononditional nodes may not specify `eq`. */
func (check NonconditionalNoEqNodeCheck) CheckNode(sequenceName string, node Node) error {
	if !node.IsConditional() {
		if len(node.Eq) != 0 {
			eq := fmt.Sprintf("%v", node.Eq)
			return InvalidValueError{
				Sequence: sequenceName,
				Node:     &node.Name,
				Field:    "eq",
				Values:   []string{eq},
				Expected: "no value; noncoditional nodes may not specify eq",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type RetryIfRetryWaitNodeCheck struct{}

/* If `retryWait` is set, `retry` must be set (nonzero). */
func (check RetryIfRetryWaitNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.RetryWait != "" && node.Retry == 0 {
		return MissingValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "retry",
			Explanation: "required when `retryWait` field set",
		}
	}

	return nil
}

/* ========================================================================== */
type ValidRetryWaitNodeCheck struct{}

/* `retryWait` should be a valid duration. */
func (check ValidRetryWaitNodeCheck) CheckNode(sequenceName string, node Node) error {
	if node.RetryWait != "" {
		if _, err := time.ParseDuration(node.RetryWait); err != nil {
			return InvalidValueError{
				Sequence: sequenceName,
				Node:     &node.Name,
				Field:    "retryWait",
				Values:   []string{node.RetryWait},
				Expected: "valid duration string",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type RequiredArgsProvidedNodeCheck struct {
	specs Specs
}

/* Sequence and conditional nodes must be provided with their required args. */
func (check RequiredArgsProvidedNodeCheck) CheckNode(sequenceName string, node Node) error {
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
		split := strings.Split(each, ":")
		if len(split) != 2 {
			continue
		}
		declaredArgs[split[1]] = true
	}

	// Check that all required args are present
	missing := map[string][]string{} // missing arg -> list of sequences that require it
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
				missing[*reqArg.Name] = append(missing[*reqArg.Name], seq.Name)
			}
		}
	}

	if len(missing) > 0 {
		missingFmt := []string{}
		for missingArg, sequences := range missing {
			multipleValues := ""
			if len(sequences) > 1 {
				multipleValues = "s"
			}
			missingFmt = append(missingFmt, fmt.Sprintf("arg: %s, sequence%s: %s", missingArg, multipleValues, strings.Join(sequences, ",")))
		}
		multipleValues := ""
		if len(missing) > 1 {
			multipleValues = "s"
		}
		explanation := fmt.Sprintf("required arg%s to sequence(s) called by node not provided: %s", multipleValues, strings.Join(missingFmt, "; "))
		return MissingValueError{
			Sequence:    sequenceName,
			Node:        &node.Name,
			Field:       "args",
			Explanation: explanation,
		}
	}

	return nil
}

/* ========================================================================== */
type NoExtraSequenceArgsProvidedNodeCheck struct {
	specs Specs
}

/* Sequence and conditional nodes shouldn't provide more than the specified sequence args. */
func (check NoExtraSequenceArgsProvidedNodeCheck) CheckNode(sequenceName string, node Node) error {
	// List of sequences to check
	// (This will also include jobs, but we'll ignore them later)
	var sequences []string
	if node.NodeType != nil {
		sequences = []string{*node.NodeType}
	}
	for _, seq := range node.Eq {
		sequences = append(sequences, seq)
	}

	// Set of all (excess) inputs to a node
	// Starts out as all the input args
	var excessArgs = map[string]bool{}
	for _, nodeArg := range node.Args {
		if nodeArg.Expected != nil {
			excessArgs[*nodeArg.Expected] = true
		}
	}
	for _, each := range node.Each {
		split := strings.Split(each, ":")
		if len(split) != 2 {
			continue
		}
		excessArgs[split[1]] = true
	}

	// Delete from `excessArgs` the ones that actually show up as sequence args to some subsequence
	for _, sequence := range sequences {
		seq, ok := check.specs.Sequences[sequence]
		if !ok {
			continue
		}
		for _, argList := range [][]*Arg{seq.Args.Required, seq.Args.Optional} {
			for _, arg := range argList {
				if arg.Name != nil && excessArgs[*arg.Name] {
					delete(excessArgs, *arg.Name)
				}
			}
		}
	}

	if len(excessArgs) > 0 {
		multiple1 := ""
		multiple2 := "s"
		if len(excessArgs) > 1 {
			multiple1 = "s"
			multiple2 = ""
		}
		return InvalidValueError{
			Sequence: sequenceName,
			Node:     &node.Name,
			Field:    "args`, `each.alias",
			Values:   stringSetToArray(excessArgs),
			Expected: fmt.Sprintf("only args that the subsequence%s require%s", multiple1, multiple2),
		}
	}

	return nil
}

/* ========================================================================== */
type SubsequencesExistNodeCheck struct {
	specs Specs
}

/* All sequences called by the node exist in the specs. */
func (check SubsequencesExistNodeCheck) CheckNode(sequenceName string, node Node) error {
	// List of sequences to check
	var sequences []string
	if node.IsSequence() {
		sequences = []string{*node.NodeType}
	} else if node.IsConditional() {
		for _, seq := range node.Eq {
			sequences = append(sequences, seq)
		}
	}

	// Check that subsequences exist
	values := map[string]bool{}
	for _, sequence := range sequences {
		_, ok := check.specs.Sequences[sequence]
		if !ok {
			values[sequence] = true
		}
	}

	if len(values) > 0 {
		multiple1 := ""
		multiple2 := "s"
		if len(values) > 1 {
			multiple1 = "s"
			multiple2 = ""
		}
		var field string
		if node.IsSequence() {
			field = "type"
		} else if node.IsConditional() {
			field = "eq"
		}
		return InvalidValueError{
			Sequence: sequenceName,
			Node:     &node.Name,
			Field:    field,
			Values:   stringSetToArray(values),
			Expected: fmt.Sprintf("subsequence%s that exist%s in specs", multiple1, multiple2),
		}
	}

	return nil
}
