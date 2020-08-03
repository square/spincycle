// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"strings"
)

type SequenceCheck interface {
	CheckSequence(Sequence) error
}

/* ========================================================================== */
type RequiredArgsNamedSequenceCheck struct{}

/* Sequence args must be named, i.e. include a `name` field. */
func (check RequiredArgsNamedSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, arg := range sequence.Args.Required {
		if arg.Name == nil {
			return MissingValueError{
				Sequence:    sequence.Name,
				Node:        nil,
				Field:       "args.required.name",
				Explanation: "",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type OptionalArgsNamedSequenceCheck struct{}

/* Sequence args must be named, i.e. include a `name` field. */
func (check OptionalArgsNamedSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, arg := range sequence.Args.Optional {
		if arg.Name == nil {
			return MissingValueError{
				Sequence:    sequence.Name,
				Node:        nil,
				Field:       "args.optional.name",
				Explanation: "",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type StaticArgsNamedSequenceCheck struct{}

/* Sequence args must be named, i.e. include a `name` field. */
func (check StaticArgsNamedSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, arg := range sequence.Args.Static {
		if arg.Name == nil {
			return MissingValueError{
				Sequence:    sequence.Name,
				Node:        nil,
				Field:       "args.static.name",
				Explanation: "",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type RequiredArgsHaveNoDefaultsSequenceCheck struct{}

/* Required args must not have defaults. */
func (check RequiredArgsHaveNoDefaultsSequenceCheck) CheckSequence(sequence Sequence) error {
	values := []string{} // names of required args that specify defaults
	for _, arg := range sequence.Args.Required {
		if arg.Default != nil {
			var name string
			if arg.Name == nil {
				name = "arg name not specified"
			} else {
				name = *arg.Name
			}
			values = append(values, fmt.Sprintf("%s (%s)", *arg.Default, name))
		}
	}

	if len(values) > 0 {
		return InvalidValueError{
			Sequence: sequence.Name,
			Node:     nil,
			Field:    "args.required.default",
			Values:   values,
			Expected: "no value",
		}
	}

	return nil
}

/* ========================================================================== */
type OptionalArgsHaveDefaultsSequenceCheck struct{}

/* Optional args must have defaults. */
func (check OptionalArgsHaveDefaultsSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, arg := range sequence.Args.Optional {
		if arg.Default == nil {
			return MissingValueError{
				Sequence:    sequence.Name,
				Node:        nil,
				Field:       "args.optional.default",
				Explanation: "required for optional args",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type StaticArgsHaveDefaultsSequenceCheck struct{}

/* Static args must have defaults. */
func (check StaticArgsHaveDefaultsSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, arg := range sequence.Args.Static {
		if arg.Default == nil {
			return MissingValueError{
				Sequence:    sequence.Name,
				Node:        nil,
				Field:       "args.static.default",
				Explanation: "required for static args",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type NoDuplicateArgsSequenceCheck struct{}

/* Args should appear once per sequence. */
func (check NoDuplicateArgsSequenceCheck) CheckSequence(sequence Sequence) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, args := range [][]*Arg{sequence.Args.Required, sequence.Args.Optional, sequence.Args.Static} {
		for _, arg := range args {
			if arg.Name != nil && seen[*arg.Name] {
				values[*arg.Name] = true
			}
			seen[*arg.Name] = true
		}
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequence.Name,
			Node:        nil,
			Field:       "args.*.name",
			Values:      stringSetToArray(values),
			Explanation: "sequence args must have unique names within the sequence",
		}
	}

	return nil
}

/* ========================================================================== */
type HasNodesSequenceCheck struct{}

/* Sequences must contain nodes i.e. are not empty. */
func (check HasNodesSequenceCheck) CheckSequence(sequence Sequence) error {
	if len(sequence.Nodes) == 0 {
		return MissingValueError{
			Sequence:    sequence.Name,
			Node:        nil,
			Field:       "nodes",
			Explanation: "at least one node required",
		}
	}

	return nil
}

/* ========================================================================== */
type NodesSetsUniqueSequenceCheck struct{}

/* Nodes can't set the same args. */
func (check NodesSetsUniqueSequenceCheck) CheckSequence(sequence Sequence) error {
	set := map[string]string{}          // arg --> first node that sets it
	duplicated := map[string][]string{} // duplicated arg --> nodes that set it
	for _, node := range sequence.Nodes {
		for _, nodeSet := range node.Sets {
			if setBy, ok := duplicated[*nodeSet.As]; ok {
				duplicated[*nodeSet.As] = append(setBy, node.Name)
			} else if setBy, ok := set[*nodeSet.As]; ok {
				duplicated[*nodeSet.As] = []string{setBy, node.Name}
			} else {
				set[*nodeSet.As] = node.Name
			}
		}
	}

	if len(duplicated) > 0 {
		values := []string{}
		for arg, setBy := range duplicated {
			values = append(values, fmt.Sprintf("%s (set by %s)", arg, strings.Join(setBy, ", ")))
		}
		return DuplicateValueError{
			Sequence:    sequence.Name,
			Node:        nil,
			Field:       "nodes.sets.as",
			Values:      values,
			Explanation: "note that if `as` is not explicitly specified, then its value is the same as `args`",
		}
	}

	return nil
}

/* ========================================================================== */
type ACLAdminXorOpsSequenceCheck struct{}

/* `admin` and `ops` are mutually exclusive. */
func (check ACLAdminXorOpsSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, acl := range sequence.ACL {
		if acl.Admin && len(acl.Ops) != 0 {
			return InvalidValueError{
				Sequence: sequence.Name,
				Node:     nil,
				Field:    "acl.admin",
				Values:   []string{"true"},
				Expected: "admin=false; alternatively, remove ops",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type ACLsHaveRolesSequenceCheck struct{}

/* ACLs must specify a role. */
func (check ACLsHaveRolesSequenceCheck) CheckSequence(sequence Sequence) error {
	for _, acl := range sequence.ACL {
		if acl.Role == "" {
			return MissingValueError{
				Sequence:    sequence.Name,
				Node:        nil,
				Field:       "acl.role",
				Explanation: "roles may not be the empty string",
			}
		}
	}

	return nil
}

/* ========================================================================== */
type NoDuplicateACLRolesSequenceCheck struct{}

/* ACL roles must not be duplicated. */
func (check NoDuplicateACLRolesSequenceCheck) CheckSequence(sequence Sequence) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, acl := range sequence.ACL {
		if seen[acl.Role] {
			values[acl.Role] = true
		}
		seen[acl.Role] = true
	}

	if len(values) > 0 {
		return DuplicateValueError{
			Sequence:    sequence.Name,
			Node:        nil,
			Field:       "acl.role",
			Values:      stringSetToArray(values),
			Explanation: "",
		}
	}

	return nil
}
