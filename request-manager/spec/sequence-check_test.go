// Copyright 2020, Square, Inc.

package spec_test

import (
	"fmt"
	"testing"

	. "github.com/square/spincycle/v2/request-manager/spec"
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
		Field: "args.required.name",
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
				&Arg{Name: &testVal, Default: testVal},
			},
		},
	}
	expectedErr := InvalidValueError{
		Field:  "args.required.default",
		Values: []string{fmt.Sprintf("%s (%s)", testVal, testVal)},
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
		Field: "args.optional.default",
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
		Field: "args.static.default",
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
				&Arg{Name: &testVal},
			},
			Static: []*Arg{
				&Arg{Name: &testVal},
			},
		},
	}
	expectedErr := DuplicateValueError{
		Field:  "args.*.name",
		Values: []string{testVal},
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, fmt.Sprintf("accepted two instances of %s in sequence args, expected error", testVal))
}

func TestFailHasNodesSequenceCheck(t *testing.T) {
	check := HasNodesSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
	}
	expectedErr := MissingValueError{
		Field: "nodes",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted sequence with no nodes, expected error")
}

func TestNodesSetsUniqueSequenceCheck(t *testing.T) {
	check := NodesSetsUniqueSequenceCheck{}
	sequence := Sequence{ // Check that repeats within a node _aren't_ caught
		Name: seqA,
		Nodes: map[string]*Node{
			nodeA: &Node{
				Name: nodeA,
				Sets: []*NodeSet{
					&NodeSet{Arg: &testVal, As: &testVal},
					&NodeSet{Arg: &testVal, As: &testVal},
				},
			},
		},
	}
	err := check.CheckSequence(sequence)
	if err != nil {
		t.Fatalf("check failed, expected pass")
	}
}

func TestFailNodesSetsUniqueSequenceCheck1(t *testing.T) {
	check := NodesSetsUniqueSequenceCheck{}
	nodeB := "node-b"
	sequence := Sequence{ // Check that repeats between nodes are caught
		Name: seqA,
		Nodes: map[string]*Node{
			nodeA: &Node{
				Name: nodeA,
				Sets: []*NodeSet{&NodeSet{Arg: &testVal, As: &testVal}},
			},
			nodeB: &Node{
				Name: nodeA, // Cheat to make the 'Values' field of the returned err predictable
				Sets: []*NodeSet{&NodeSet{Arg: &testVal, As: &testVal}},
			},
		},
	}
	expectedErr := DuplicateValueError{
		Field:  "nodes.sets.as",
		Values: []string{fmt.Sprintf("%s (set by %s, %s)", testVal, nodeA, nodeA)},
	}
	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted sequence with multiple nodes setting the same arg, expected error")
}

func TestFailNodesSetsUniqueSequenceCheck2(t *testing.T) {
	check := NodesSetsUniqueSequenceCheck{}
	sequence := Sequence{ // Check that setting a sequence arg is caught
		Name: seqA,
		Args: SequenceArgs{
			Required: []*Arg{
				&Arg{Name: &testVal},
			},
		},
		Nodes: map[string]*Node{
			nodeA: &Node{
				Name: "this sequence", // Cheat to make the 'Values' field of the returned err predictable
				Sets: []*NodeSet{&NodeSet{Arg: &testVal, As: &testVal}},
			},
		},
	}
	expectedErr := DuplicateValueError{
		Field:  "nodes.sets.as",
		Values: []string{fmt.Sprintf("%s (set by %s, %s)", testVal, "this sequence", "this sequence")},
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
		Field:  "acl.admin",
		Values: []string{"true"},
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
		Field: "acl.role",
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted ACL with missing role, expected error")
}

func TestFailNoDuplicateACLRolesSequenceCheck(t *testing.T) {
	check := NoDuplicateACLRolesSequenceCheck{}
	sequence := Sequence{
		Name: seqA,
		ACL: []ACL{
			ACL{Role: testVal},
			ACL{Role: testVal},
		},
	}
	expectedErr := DuplicateValueError{
		Field:  "acl.role",
		Values: []string{testVal},
	}

	err := check.CheckSequence(sequence)
	compareError(t, err, expectedErr, "accepted duplicated acl roles, expected error")
}
