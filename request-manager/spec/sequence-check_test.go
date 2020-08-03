// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"testing"
)

func TestFailRequiredArgsNamedSequenceCheck(t *testing.T) {
	check := RequiredArgsNamedSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		Args: SequenceArgs{
			Required: []*Arg{
				&Arg{Name: nil},
			},
		},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Field:    "args.required.name",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted sequence arg with no name, expected error")
}

func TestFailRequiredArgsHaveNoDefaultsSequenceCheck(t *testing.T) {
	check := RequiredArgsHaveNoDefaultsSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		Args: SequenceArgs{
			Required: []*Arg{
				&Arg{Name: &value, Default: &value},
			},
		},
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Field:    "args.required.default",
		Values:   []string{fmt.Sprintf("%s (%s)", value, value)},
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted optional arg with no default, expected error")
}

func TestFailOptionalArgsHaveDefaultsSequenceCheck(t *testing.T) {
	check := OptionalArgsHaveDefaultsSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		Args: SequenceArgs{
			Optional: []*Arg{
				&Arg{Default: nil},
			},
		},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Field:    "args.optional.default",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted optional arg with no default, expected error")
}

func TestFailStaticArgsHaveDefaultsSequenceCheck(t *testing.T) {
	check := StaticArgsHaveDefaultsSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		Args: SequenceArgs{
			Static: []*Arg{
				&Arg{Default: nil},
			},
		},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Field:    "args.static.default",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted static arg with no default, expected error")
}

func TestFailNoDuplicateArgsSequenceCheck(t *testing.T) {
	check := NoDuplicateArgsSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		Args: SequenceArgs{
			Required: []*Arg{
				&Arg{Name: &value},
			},
			Static: []*Arg{
				&Arg{Name: &value},
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Field:    "args.*.name",
		Values:   []string{value},
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, fmt.Sprintf("accepted two instances of %s in sequence args, expected error", value))
}

func TestFailHasNodesSequenceCheck(t *testing.T) {
	check := HasNodesSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Field:    "nodes",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted sequence with no nodes, expected error")
}

func TestFailNodesSetsUniqueSequenceCheck(t *testing.T) {
	check := NodesSetsUniqueSequenceCheck{}
	nodeB := "node-b"
	sequence := Sequence{
		Name: seqA,
		Nodes: map[string]*Node{
			nodeA: &Node{
				Name: nodeA,
				Sets: []*NodeSet{&NodeSet{Arg: &value, As: &value}},
			},
			nodeB: &Node{
				Name: nodeA, // Cheat to make the `Values` field of the returned err predictable
				Sets: []*NodeSet{&NodeSet{Arg: &value, As: &value}},
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Field:    "nodes.sets.as",
		Values:   []string{fmt.Sprintf("%s (set by %s, %s)", value, nodeA, nodeA)},
	}
	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted sequence with multiple nodes setting the same arg, expected error")
}

func TestFailACLAdminXorOpsSequenceCheck(t *testing.T) {
	check := ACLAdminXorOpsSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		ACL: []ACL{
			ACL{
				Admin: true,
				Ops:   []string{"op"},
			},
		},
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Field:    "acl.admin",
		Values:   []string{"true"},
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted sequence specifying ops with admin=true, expected error")
}

func TestFailACLsHaveRolesSequenceCheck(t *testing.T) {
	check := ACLsHaveRolesSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		ACL:  []ACL{ACL{}},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Field:    "acl.role",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted ACL with missing role, expected error")
}

func TestFailNoDuplicateACLRolesSequenceCheck(t *testing.T) {
	check := NoDuplicateACLRolesSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		ACL: []ACL{
			ACL{Role: value},
			ACL{Role: value},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Field:    "acl.role",
		Values:   []string{value},
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted duplicated acl roles, expected error")
}
