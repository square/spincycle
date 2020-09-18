// Copyright 2020, Square, Inc.

package spec_test

import (
	"fmt"
	"testing"

	. "github.com/square/spincycle/v2/request-manager/spec"
)

func TestFailHasCategoryNodeCheck(t *testing.T) {
	check := HasCategoryNodeCheck{}
	node := Node{
		Name:     nodeA,
		Category: nil,
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "category",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted node with no category, expected error")
}

func TestFailValidCategoryNodeCheck(t *testing.T) {
	check := ValidCategoryNodeCheck{}
	node := Node{
		Name:     nodeA,
		Category: &testVal,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "category",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted category of value other than 'job', 'sequence', or 'conditional', expected error")
}

func TestFailValidEachNodeCheck(t *testing.T) {
	check := ValidEachNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{testVal},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "each",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted each not in format 'list:element', expected error")
}

func TestFailEachElementUniqueNodeCheck(t *testing.T) {
	check := EachElementUniqueNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{"a:element", "b:element"},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "each",
		Values: []string{"element"},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated each element, expected error")
}

func TestFailEachNotRenamedTwiceNodeCheck(t *testing.T) {
	check := EachNotRenamedTwiceNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{"list:a", "list:b"},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "each",
		Values: []string{"list"},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated each list, expected error")
}

func TestFailArgsNotNilNodeCheck(t *testing.T) {
	check := ArgsNotNilNodeCheck{}
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{nil},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "args",
		Values: []string{"nil"},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted nil node args, expected error")
}

func TestFailArgsAreNamedNodeCheck(t *testing.T) {
	check := ArgsAreNamedNodeCheck{}
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Given: &testVal,
			},
		},
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "args.expected",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted node arg without 'expected' field, expected error")
}

func TestFailArgsExpectedUniqueNodeCheck1(t *testing.T) {
	check := ArgsExpectedUniqueNodeCheck{}
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Expected: &testVal,
			},
			&NodeArg{
				Expected: &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "args.expected",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated args, expected error")
}

func TestFailArgsExpectedUniqueNodeCheck2(t *testing.T) {
	check := ArgsExpectedUniqueNodeCheck{}
	given1 := "given1"
	given2 := "given2"
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Expected: &testVal,
				Given:    &given1,
			},
			&NodeArg{
				Expected: &testVal,
				Given:    &given2,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "args.expected",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated args, expected error")
}

func TestFailArgsExpectedUniqueNodeCheck3(t *testing.T) {
	check := ArgsExpectedUniqueNodeCheck{}
	given1 := "given1"
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Expected: &testVal,
				Given:    &given1,
			},
			&NodeArg{
				Expected: &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "args.expected",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated args, expected error")
}

func TestFailArgsNotRenamedTwiceNodeCheck(t *testing.T) {
	check := ArgsNotRenamedTwiceNodeCheck{}
	expected1 := "expected1"
	expected2 := "expected2"
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Expected: &expected1,
				Given:    &testVal,
			},
			&NodeArg{
				Expected: &expected2,
				Given:    &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "args.given",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "arg renamed differently with args.given, expected error")
}

func TestFailEachElementDoesNotDuplicateArgsExpectedNodeCheck(t *testing.T) {
	check := EachElementDoesNotDuplicateArgsExpectedNodeCheck{}
	given := "given"
	node := Node{
		Name: nodeA,
		Each: []string{"list:" + testVal},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &testVal,
				Given:    &given,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "each",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, fmt.Sprintf("allowed two lists to be renamed %s, expected error", testVal))
}

func TestFailEachListDoesNotDuplicateArgsGivenNodeCheck1(t *testing.T) {
	check := EachListDoesNotDuplicateArgsGivenNodeCheck{}
	expected := "expected"
	node := Node{
		Name: nodeA,
		Each: []string{testVal + ":element"},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &expected,
				Given:    &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "each",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, fmt.Sprintf("allowed %s to be renamed twice, expected error", testVal))
}

func TestFailEachListDoesNotDuplicateArgsGivenNodeCheck2(t *testing.T) {
	check := EachListDoesNotDuplicateArgsGivenNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{testVal + ":element"},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &testVal,
				Given:    &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "each",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, fmt.Sprintf("allowed %s to be renamed twice, expected error", testVal))
}

func TestEachListDoesNotDuplicateArgsGivenNodeCheck(t *testing.T) {
	check := EachListDoesNotDuplicateArgsGivenNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{testVal + ":element"},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &testVal,
			},
		},
	}

	err := check.CheckNode(node)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestFailSetsNotNilNodeCheck(t *testing.T) {
	check := SetsNotNilNodeCheck{}
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{nil},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "sets",
		Values: []string{"nil"},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted nil node sets, expected error")
}

func TestFailSetsAreNamedNodeCheck(t *testing.T) {
	check := SetsAreNamedNodeCheck{}
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				As: &testVal,
			},
		},
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "sets.arg",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted node sets without 'arg' field, expected error")
}

func TestFailSetsAsUniqueNodeCheck1(t *testing.T) {
	check := SetsAsUniqueNodeCheck{}
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				Arg: &testVal,
				As:  &testVal,
			},
			&NodeSet{
				Arg: &testVal,
				As:  &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "sets.as",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated sets, expected error")
}

func TestFailSetsAsUniqueNodeCheck2(t *testing.T) {
	check := SetsAsUniqueNodeCheck{}
	arg1 := "arg1"
	arg2 := "arg2"
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				Arg: &arg1,
				As:  &testVal,
			},
			&NodeSet{
				Arg: &arg2,
				As:  &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "sets.as",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated sets, expected error")
}

func TestFailSetsAsUniqueNodeCheck3(t *testing.T) {
	check := SetsAsUniqueNodeCheck{}
	arg1 := "arg1"
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				Arg: &arg1,
				As:  &testVal,
			},
			&NodeSet{
				Arg: &testVal,
				As:  &testVal,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "sets.as",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted duplicated sets, expected error")
}

func TestFailSetsNotRenamedTwiceNodeCheck(t *testing.T) {
	check := SetsNotRenamedTwiceNodeCheck{}
	as1 := "as1"
	as2 := "as2"
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				Arg: &testVal,
				As:  &as1,
			},
			&NodeSet{
				Arg: &testVal,
				As:  &as2,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Node:   &nodeA,
		Field:  "sets.arg",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "arg renamed differently with sets.as, expected error")
}

func TestFailEachIfParallelNodeCheck(t *testing.T) {
	check := EachIfParallelNodeCheck{}
	var parallel uint = 5
	node := Node{
		Name:     nodeA,
		Parallel: &parallel,
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "each",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted node with 'parallel' field with empty 'each' field, expected error")
}

func TestFailValidParallelNodeCheck(t *testing.T) {
	check := ValidParallelNodeCheck{}
	var parallel uint = 0
	node := Node{
		Name:     nodeA,
		Parallel: &parallel,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "parallel",
		Values: []string{"0"},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted parallel = 0, expected error")
}

func TestFailConditionalNoTypeNodeCheck(t *testing.T) {
	check := ConditionalNoTypeNodeCheck{}
	conditional := "conditional"
	node := Node{
		Name:     nodeA,
		Category: &conditional,
		NodeType: &testVal,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "type",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted conditional node specifying a type, expected error")
}

func TestFailConditionalHasIfNodeCheck(t *testing.T) {
	check := ConditionalHasIfNodeCheck{}
	conditional := "conditional"
	node := Node{
		Name:     nodeA,
		Category: &conditional,
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "if",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted conditional sequence without 'if' field, expected error")
}

func TestFailConditionalHasEqNodeCheck(t *testing.T) {
	check := ConditionalHasEqNodeCheck{}
	conditional := "conditional"
	node := Node{
		Name:     nodeA,
		Category: &conditional,
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "eq",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted conditional sequence without 'eq' field, expected error")
}

func TestFailNonconditionalHasTypeNodeCheck(t *testing.T) {
	check := NonconditionalHasTypeNodeCheck{}
	node := Node{
		Name: nodeA,
	}

	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "type",
	}
	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted node with no type, expected error")
}

func TestFailNonconditionalNoIfNodeCheck(t *testing.T) {
	check := NonconditionalNoIfNodeCheck{}
	node := Node{
		Name: nodeA,
		If:   &testVal,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "if",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted nonconditional sequence with 'if' field, expected error")
}

func TestFailNonconditionalNoEqNodeCheck(t *testing.T) {
	check := NonconditionalNoEqNodeCheck{}
	eq := map[string]string{"yes": "noop", "default": "noop"}
	node := Node{
		Name: nodeA,
		Eq:   eq,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "eq",
		Values: []string{fmt.Sprint(eq)},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted nonconditional sequence with 'eq' field, expected error")
}

func TestFailRetryIfRetryWaitNodeCheck(t *testing.T) {
	check := RetryIfRetryWaitNodeCheck{}
	node := Node{
		Name:      nodeA,
		RetryWait: testVal,
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "retry",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted node with 'retryWait' field with retry: 0, expected error")
}

func TestFailValidRetryWaitNodeCheck(t *testing.T) {
	check := ValidRetryWaitNodeCheck{}
	node := Node{
		Name:      nodeA,
		RetryWait: testVal,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "retryWait",
		Values: []string{testVal},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "accepted bad retryWait: duration, expected error")
}

func TestFailRequiredArgsProvidedNodeCheck1(t *testing.T) {
	seqa := "seq-a"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqa: &Sequence{
				Name: seqa,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &testVal},
					},
				},
			},
		},
	}
	check := RequiredArgsProvidedNodeCheck{specs}
	sequence := "sequence" // Test sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqa,
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "args",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "not all required args to sequence node listed in 'args', expected error")
}

func TestFailRequiredArgsProvidedNodeCheck2(t *testing.T) {
	seqa := "seq-a"
	seqb := "seq-b"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqa: &Sequence{
				Name: seqa,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &testVal},
					},
				},
			},
			seqb: &Sequence{
				Name: seqb,
			},
		},
	}
	check := RequiredArgsProvidedNodeCheck{specs}
	conditional := "conditional" // Test conditional node
	node := Node{
		Name:     nodeA,
		Category: &conditional,
		Eq: map[string]string{
			seqa: seqa,
			seqb: seqb,
		},
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "args",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "not all required args to conditional sequence node listed in 'args', expected error")
}

func TestFailRequiredArgsProvidedNodeCheck3(t *testing.T) {
	seqa := "seq-a"
	reqa := "req-a"
	reqb := "req-b"
	reqc := "req-c"
	reqd := "req-d"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqa: &Sequence{
				Name: seqa,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &reqa},
						&Arg{Name: &reqb},
						&Arg{Name: &reqc},
						&Arg{Name: &reqd}, // This is missing
					},
				},
			},
		},
	}
	check := RequiredArgsProvidedNodeCheck{specs}
	sequence := "sequence" // Test expanded sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqa,
		Each: []string{
			fmt.Sprintf("%ss:%s", reqa, reqa), // Specify in 'each' and 'args'
			fmt.Sprintf("%ss:%s", reqc, reqc), // Specify only here
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &reqa, Given: &reqa}, // Specified above in 'each'
			&NodeArg{Expected: &reqb, Given: &reqb}, // Specify only here
		},
	}
	expectedErr := MissingValueError{
		Node:  &nodeA,
		Field: "args",
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "not all required args to expanded sequence node listed in 'args', expected error")
}

func TestNoExtraSequenceArgsProvidedNodeCheck1(t *testing.T) {
	argA := "arg-a"
	argB := "arg-b"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &argA},
					},
					Optional: []*Arg{
						&Arg{Name: &argB},
					},
				},
			},
		},
	}
	check := NoExtraSequenceArgsProvidedNodeCheck{specs}
	sequence := "sequence" // Test sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqA,
		Each: []string{
			fmt.Sprintf("%ss:%s", argA, argA),
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &argB, Given: &argB},
		},
	}

	err := check.CheckNode(node)
	if err != nil {
		t.Errorf("NoExtraSequenceArgsProvidedNodeCheck failed, expected pass")
	}
}

func TestNoExtraSequenceArgsProvidedNodeCheck2(t *testing.T) {
	seqB := "seq-b"
	argA := "arg-a"
	argB := "arg-b"
	argC := "arg-c"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
				Args: SequenceArgs{
					Optional: []*Arg{
						&Arg{Name: &argB},
					},
				},
			},
			seqB: &Sequence{
				Name: seqB,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &argA},
						&Arg{Name: &argC},
					},
				},
			},
		},
	}
	check := NoExtraSequenceArgsProvidedNodeCheck{specs}
	conditional := "conditional" // Test conditional node
	node := Node{
		Name:     nodeA,
		Category: &conditional,
		Eq: map[string]string{
			seqA: seqA,
			seqB: seqB,
		},
		Each: []string{
			fmt.Sprintf("%ss:%s", argA, argA),
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &argB, Given: &argB},
			&NodeArg{Expected: &argC, Given: &argC},
		},
	}

	err := check.CheckNode(node)
	if err != nil {
		t.Errorf("NoExtraSequenceArgsProvidedNodeCheck failed, expected pass")
	}
}

func TestNoExtraSequenceArgsProvidedNodeCheck3(t *testing.T) {
	argA := "arg-a"
	argB := "arg-b"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &argA},
					},
					Optional: []*Arg{
						&Arg{Name: &argB},
					},
				},
			},
		},
	}
	check := NoExtraSequenceArgsProvidedNodeCheck{specs}
	sequence := "sequence" // Test expanded sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqA,
		Each: []string{
			fmt.Sprintf("%ss:%s", argA, argA),
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &argB, Given: &argB},
		},
	}

	err := check.CheckNode(node)
	if err != nil {
		t.Errorf("NoExtraSequenceArgsProvidedNodeCheck failed, expected pass")
	}
}

func TestFailNoExtraSequenceArgsProvidedNodeCheck1(t *testing.T) {
	argA := "arg-a"
	argB := "arg-b"
	argC := "arg-c"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &argA},
					},
					Optional: []*Arg{
						&Arg{Name: &argB},
					},
				},
			},
		},
	}
	check := NoExtraSequenceArgsProvidedNodeCheck{specs}
	sequence := "sequence" // Test sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqA,
		Each: []string{
			fmt.Sprintf("%ss:%s", argA, argA),
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &argB, Given: &argB},
			&NodeArg{Expected: &argC, Given: &argC}, // Unnecessary arg
		},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "args', 'each.element",
		Values: []string{argC},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "sequence node provides subsequence with unnecessary, expected error")
}

func TestFailNoExtraSequenceArgsProvidedNodeCheck2(t *testing.T) {
	seqB := "seq-b"
	argA := "arg-a"
	argB := "arg-b"
	argC := "arg-c"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
				Args: SequenceArgs{
					Optional: []*Arg{
						&Arg{Name: &argB},
					},
				},
			},
			seqB: &Sequence{
				Name: seqB,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &argA},
					},
				},
			},
		},
	}
	check := NoExtraSequenceArgsProvidedNodeCheck{specs}
	conditional := "conditional" // Test conditional node
	node := Node{
		Name:     nodeA,
		Category: &conditional,
		Eq: map[string]string{
			seqA: seqA,
			seqB: seqB,
		},
		Each: []string{
			fmt.Sprintf("%ss:%s", argA, argA),
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &argB, Given: &argB},
			&NodeArg{Expected: &argC, Given: &argC}, // Unnecessary arg
		},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "args', 'each.element",
		Values: []string{argC},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "sequence node provides subsequence with unnecessary, expected error")
}

func TestFailNoExtraSequenceArgsProvidedNodeCheck3(t *testing.T) {
	argA := "arg-a"
	argB := "arg-b"
	argC := "arg-c"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
				Args: SequenceArgs{
					Required: []*Arg{
						&Arg{Name: &argA},
					},
					Optional: []*Arg{
						&Arg{Name: &argB},
					},
				},
			},
		},
	}
	check := NoExtraSequenceArgsProvidedNodeCheck{specs}
	sequence := "sequence" // Test expanded sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqA,
		Each: []string{
			fmt.Sprintf("%ss:%s", argA, argA),
			fmt.Sprintf("%ss:%s", argC, argC), // Unnecessary arg
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &argB, Given: &argB},
		},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "args', 'each.element",
		Values: []string{argC},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "sequence node provides subsequence with unnecessary, expected error")
}

func TestFailSubsequencesExistNodeCheck1(t *testing.T) {
	seqB := "seq-b"
	specs := Specs{
		Sequences: map[string]*Sequence{
			seqA: &Sequence{
				Name: seqA,
			},
		},
	}
	check := SubsequencesExistNodeCheck{specs}
	conditional := "conditional" // Test conditional node
	node := Node{
		Name:     nodeA,
		Category: &conditional,
		Eq: map[string]string{
			seqA:      seqA,
			"default": seqB,
		},
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "eq",
		Values: []string{seqB},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "node calls seq that does not exist in specs, expected error")
}

func TestFailSubsequencesExistNodeCheck2(t *testing.T) {
	specs := Specs{
		Sequences: map[string]*Sequence{},
	}
	check := SubsequencesExistNodeCheck{specs}
	sequence := "sequence" // Test sequence node
	node := Node{
		Name:     nodeA,
		Category: &sequence,
		NodeType: &seqA,
	}
	expectedErr := InvalidValueError{
		Node:   &nodeA,
		Field:  "type",
		Values: []string{seqA},
	}

	err := check.CheckNode(node)
	compareError(t, err, expectedErr, "node calls seq that does not exist in specs, expected error")
}
