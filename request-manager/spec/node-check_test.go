// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"testing"
)

func TestFailHasCategoryNodeCheck(t *testing.T) {
	check := HasCategoryNodeCheck{}
	node := Node{
		Name:     nodeA,
		Category: nil,
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "category",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted node with no category, expected error")
}

func TestFailValidCategoryNodeCheck(t *testing.T) {
	check := ValidCategoryNodeCheck{}
	node := Node{
		Name:     nodeA,
		Category: &value,
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "category",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted category of value other than 'job', 'sequence', or 'conditional', expected error")
}

func TestFailValidEachNodeCheck(t *testing.T) {
	check := ValidEachNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{value},
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted each not in format `arg:alias`, expected error")
}

func TestFailEachAliasUniqueNodeCheck(t *testing.T) {
	check := EachAliasUniqueNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{"a:alias", "b:alias"},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
		Values:   []string{"alias"},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted duplicated each alias, expected error")
}

func TestFailEachNotRenamedTwiceNodeCheck(t *testing.T) {
	check := EachNotRenamedTwiceNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{"arg:a", "arg:b"},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
		Values:   []string{"arg"},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted duplicated each arg, expected error")
}

func TestFailArgsAreNamedNodeCheck(t *testing.T) {
	check := ArgsAreNamedNodeCheck{}
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Given: &value,
			},
		},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args.expected",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted node arg without `expected` field, expected error")
}

func TestFailArgsExpectedUniqueNodeCheck1(t *testing.T) {
	check := ArgsExpectedUniqueNodeCheck{}
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Expected: &value,
			},
			&NodeArg{
				Expected: &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args.expected",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
				Expected: &value,
				Given:    &given1,
			},
			&NodeArg{
				Expected: &value,
				Given:    &given2,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args.expected",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted duplicated args, expected error")
}

func TestFailArgsExpectedUniqueNodeCheck3(t *testing.T) {
	check := ArgsExpectedUniqueNodeCheck{}
	given1 := "given1"
	node := Node{
		Name: nodeA,
		Args: []*NodeArg{
			&NodeArg{
				Expected: &value,
				Given:    &given1,
			},
			&NodeArg{
				Expected: &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args.expected",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
				Given:    &value,
			},
			&NodeArg{
				Expected: &expected2,
				Given:    &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args.given",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "arg renamed differently with args.given, expected error")
}

func TestFailEachAliasDoesNotDuplicateArgsExpectedNodeCheck(t *testing.T) {
	check := EachAliasDoesNotDuplicateArgsExpectedNodeCheck{}
	given := "given"
	node := Node{
		Name: nodeA,
		Each: []string{"arg:" + value},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &value,
				Given:    &given,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, fmt.Sprintf("allowed two args to be renamed %s, expected error", value))
}

func TestFailEachArgDoesNotDuplicateArgsGivenNodeCheck1(t *testing.T) {
	check := EachArgDoesNotDuplicateArgsGivenNodeCheck{}
	expected := "expected"
	node := Node{
		Name: nodeA,
		Each: []string{value + ":alias"},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &expected,
				Given:    &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, fmt.Sprintf("allowed %s to be renamed twice, expected error", value))
}

func TestFailEachArgDoesNotDuplicateArgsGivenNodeCheck2(t *testing.T) {
	check := EachArgDoesNotDuplicateArgsGivenNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{value + ":alias"},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &value,
				Given:    &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, fmt.Sprintf("allowed %s to be renamed twice, expected error", value))
}

func TestEachArgDoesNotDuplicateArgsGivenNodeCheck(t *testing.T) {
	check := EachArgDoesNotDuplicateArgsGivenNodeCheck{}
	node := Node{
		Name: nodeA,
		Each: []string{value + ":alias"},
		Args: []*NodeArg{
			&NodeArg{
				Expected: &value,
			},
		},
	}

	err := check.CheckNode(seqA, node)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestFailSetsAreNamedNodeCheck(t *testing.T) {
	check := SetsAreNamedNodeCheck{}
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				As: &value,
			},
		},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "sets.arg",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted node sets without `arg` field, expected error")
}

func TestFailSetsAsUniqueNodeCheck1(t *testing.T) {
	check := SetsAsUniqueNodeCheck{}
	node := Node{
		Name: nodeA,
		Sets: []*NodeSet{
			&NodeSet{
				Arg: &value,
				As:  &value,
			},
			&NodeSet{
				Arg: &value,
				As:  &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "sets.as",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
				As:  &value,
			},
			&NodeSet{
				Arg: &arg2,
				As:  &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "sets.as",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
				As:  &value,
			},
			&NodeSet{
				Arg: &value,
				As:  &value,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "sets.as",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
				Arg: &value,
				As:  &as1,
			},
			&NodeSet{
				Arg: &value,
				As:  &as2,
			},
		},
	}
	expectedErr := DuplicateValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "sets.arg",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "each",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted node with `parallel` field with empty `each` field, expected error")
}

func TestFailValidParallelNodeCheck(t *testing.T) {
	check := ValidParallelNodeCheck{}
	var parallel uint = 0
	node := Node{
		Name:     nodeA,
		Parallel: &parallel,
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "parallel",
		Values:   []string{"0"},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted parallel = 0, expected error")
}

func TestFailConditionalNoTypeNodeCheck(t *testing.T) {
	check := ConditionalNoTypeNodeCheck{}
	conditional := "conditional"
	node := Node{
		Name:     nodeA,
		Category: &conditional,
		NodeType: &value,
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "type",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "if",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted conditional sequence without `if` field, expected error")
}

func TestFailConditionalHasEqNodeCheck(t *testing.T) {
	check := ConditionalHasEqNodeCheck{}
	conditional := "conditional"
	node := Node{
		Name:     nodeA,
		Category: &conditional,
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "eq",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted conditional sequence without `eq` field, expected error")
}

func TestFailNonconditionalHasTypeNodeCheck(t *testing.T) {
	check := NonconditionalHasTypeNodeCheck{}
	node := Node{
		Name: nodeA,
	}

	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "type",
	}
	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted node with no type, expected error")
}

func TestFailNonconditionalNoIfNodeCheck(t *testing.T) {
	check := NonconditionalNoIfNodeCheck{}
	node := Node{
		Name: nodeA,
		If:   &value,
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "if",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted nonconditional sequence with `if` field, expected error")
}

func TestFailNonconditionalNoEqNodeCheck(t *testing.T) {
	check := NonconditionalNoEqNodeCheck{}
	eq := map[string]string{"yes": "noop", "default": "noop"}
	node := Node{
		Name: nodeA,
		Eq:   eq,
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "eq",
		Values:   []string{fmt.Sprint(eq)},
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted nonconditional sequence with `eq` field, expected error")
}

func TestFailRetryIfRetryWaitNodeCheck(t *testing.T) {
	check := RetryIfRetryWaitNodeCheck{}
	node := Node{
		Name:      nodeA,
		RetryWait: value,
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "retry",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "accepted node with `retryWait` field with retry: 0, expected error")
}

func TestFailValidRetryWaitNodeCheck(t *testing.T) {
	check := ValidRetryWaitNodeCheck{}
	node := Node{
		Name:      nodeA,
		RetryWait: value,
	}
	expectedErr := InvalidValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "retryWait",
		Values:   []string{value},
	}

	err := check.CheckNode(seqA, node)
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
						&Arg{Name: &value},
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
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "not all required args to sequence node listed in `args`, expected error")
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
						&Arg{Name: &value},
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
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "not all required args to conditional sequence node listed in `args`, expected error")
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
			fmt.Sprintf("%ss:%s", reqa, reqa), // Specify in `each` and `args`
			fmt.Sprintf("%ss:%s", reqc, reqc), // Specify only here
		},
		Args: []*NodeArg{
			&NodeArg{Expected: &reqa, Given: &reqa}, // Specified above in `each`
			&NodeArg{Expected: &reqb, Given: &reqb}, // Specify only here
		},
	}
	expectedErr := MissingValueError{
		Sequence: seqA,
		Node:     &nodeA,
		Field:    "args",
	}

	err := check.CheckNode(seqA, node)
	compareError(t, err, expectedErr, "not all required args to expanded sequence node listed in `args`, expected error")
}
