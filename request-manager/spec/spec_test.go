// Copyright 2017-2020, Square, Inc.

package spec

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/go-test/deep"
)

const (
	specsDir = "../test/specs/"
)

/* ========================================================================= */
// Parse tests

func TestParseSpec(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"
	_, err := ParseSpec(sequencesFile)
	if err != nil {
		t.Errorf("failed to read decomm.yaml, expected success")
	}
}

func TestFailWrongType(t *testing.T) {
	sequencesFile := specsDir + "wrong-type.yaml"
	_, err := ParseSpec(sequencesFile)
	if err == nil {
		t.Errorf("unmarshaled string into uint")
	} else {
		switch err.(type) {
		case *yaml.TypeError:
			fmt.Println(err.Error())
		default:
			t.Errorf("expected yaml.TypeError, got %T: %s", err, err)
		}
	}
}

/* ========================================================================= */
// SequenceCheck tests

func TestFailRequiredArgsNamedSequenceCheck(t *testing.T) {
	filename := "fail-required-args-named-sequence-check.yaml"
	check := RequiredArgsNamedSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "name", "accepted sequence arg with no name, expected error")
}

func TestFailOptionalArgsHaveDefaultsSequenceCheck(t *testing.T) {
	filename := "fail-optional-args-have-defaults-sequence-check.yaml"
	check := OptionalArgsHaveDefaultsSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "default", "accepted optional arg with no default, expected error")
}

func TestFailStaticArgsHaveDefaultsSequenceCheck(t *testing.T) {
	filename := "fail-static-args-have-defaults-sequence-check.yaml"
	check := StaticArgsHaveDefaultsSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "default", "accepted static arg with no default, expected error")
}

func TestFailHasNodesSequenceCheck(t *testing.T) {
	filename := "fail-has-nodes-sequence-check.yaml"
	check := HasNodesSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "nodes", "accepted sequence with no nodes, expected error")
}

func TestFailAdminXorOpsSequenceCheck(t *testing.T) {
	filename := "fail-admin-xor-ops-sequence-check.yaml"
	check := AdminXorOpsSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailInvalidValueError(t, err, false, "admin", "true", "accepted sequence specifying ops with admin=true, expected error")
}

func TestFailAclsHaveRolesSequenceCheck(t *testing.T) {
	filename := "fail-acls-have-roles-sequence-check.yaml"
	check := AclsHaveRolesSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "role", "accepted ACL with missing role, expected error")
}

func TestFailNoDuplicateAclRolesSequenceCheck(t *testing.T) {
	filename := "fail-no-duplicate-acl-roles-sequence-check.yaml"
	check := NoDuplicateAclRolesSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailDuplicateValueError(t, err, false, "role", "eng", "accepted duplicated acl roles, expected error")
}

/* ========================================================================== */
// NodeCheck tests

func TestFailHasCategoryNodeCheck(t *testing.T) {
	filename := "fail-has-category-node-check.yaml"
	check := HasCategoryNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "category", "accepted node with no category, expected error")
}

func TestFailValidCategoryNodeCheck(t *testing.T) {
	filename := "fail-valid-category-node-check.yaml"
	check := ValidCategoryNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "category", "square", "accepted category of value other than 'job', 'sequence', or 'conditional', expected error")
}

func TestFailValidEachNodeCheck(t *testing.T) {
	filename := "fail-valid-each-node-check.yaml"
	check := ValidEachNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "each", "required-a", "accepted each not in format `arg:alias`, expected error")
}

func TestFailArgsAreNamedNodeCheck(t *testing.T) {
	filename := "fail-args-are-named-node-check.yaml"
	check := ArgsAreNamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "expected", "accepted node arg without `expected` field, expected error")
}

func TestFailArgsOnceNodeCheck1(t *testing.T) {
	filename := "fail-args-once-node-check-1.yaml"
	check := ArgsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args", "arg-a", "accepted duplicated args, expected error")
}

func TestFailArgsOnceNodeCheck2(t *testing.T) {
	filename := "fail-args-once-node-check-2.yaml"
	check := ArgsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args", "arg-a", "accepted duplicated args, expected error")
}

func TestFailArgsOnceNodeCheck3(t *testing.T) {
	filename := "fail-args-once-node-check-3.yaml"
	check := ArgsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args", "arg-a", "accepted duplicated args, expected error")
}

func TestFailArgsNotRenamedNodeCheck(t *testing.T) {
	filename := "fail-args-not-renamed-node-check.yaml"
	check := ArgsNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args/given", "arg-a", "arg renamed differently with args/given, expected error")
}

func TestFailSetsAreNamedNodeCheck(t *testing.T) {
	filename := "fail-sets-are-named-node-check.yaml"
	check := SetsAreNamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "arg", "accepted node sets without `arg` field, expected error")
}

func TestFailSetsOnceNodeCheck1(t *testing.T) {
	filename := "fail-sets-once-node-check-1.yaml"
	check := SetsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets", "arg-a", "accepted duplicated sets, expected error")
}

func TestFailSetsOnceNodeCheck2(t *testing.T) {
	filename := "fail-sets-once-node-check-2.yaml"
	check := SetsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets", "arg-x", "accepted duplicated sets, expected error")
}

func TestFailSetsOnceNodeCheck3(t *testing.T) {
	filename := "fail-sets-once-node-check-3.yaml"
	check := SetsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets", "arg-x", "accepted duplicated sets, expected error")
}

func TestFailSetsNotRenamedNodeCheck(t *testing.T) {
	filename := "fail-sets-not-renamed-node-check.yaml"
	check := SetsNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets/arg", "arg-a", "arg renamed differently with sets/as, expected error")
}

func TestFailEachIfParallelNodeCheck(t *testing.T) {
	filename := "fail-each-if-parallel-node-check.yaml"
	check := EachIfParallelNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "each", "accepted node with `parallel` field with empty `each` field, expected error")
}

func TestFailValidParallelNodeCheck(t *testing.T) {
	filename := "fail-valid-parallel-node-check.yaml"
	check := ValidParallelNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "parallel", "0", "accepted conditional node specifying a type, expected error")
}

func TestFailConditionalNoTypeNodeCheck(t *testing.T) {
	filename := "fail-conditional-no-type-node-check.yaml"
	check := ConditionalNoTypeNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "type", "job-type-a", "accepted conditional node specifying a type, expected error")
}

func TestFailConditionalHasIfNodeCheck(t *testing.T) {
	filename := "fail-conditional-has-if-node-check.yaml"
	check := ConditionalHasIfNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "if", "accepted conditional sequence without `if` field, expected error")
}

func TestFailConditionalHasEqNodeCheck(t *testing.T) {
	filename := "fail-conditional-has-eq-node-check.yaml"
	check := ConditionalHasEqNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "eq", "accepted conditional sequence without `eq` field, expected error")
}

func TestFailNonconditionalHasTypeNodeCheck(t *testing.T) {
	filename := "fail-nonconditional-has-type-node-check.yaml"
	check := NonconditionalHasTypeNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "type", "accepted node with no type, expected error")
}

func TestFailNonconditionalNoIfNodeCheck(t *testing.T) {
	filename := "fail-nonconditional-no-if-node-check.yaml"
	check := NonconditionalNoIfNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "if", "nonexistent-arg", "accepted nonconditional sequence with `if` field, expected error")
}

func TestFailNonconditionalNoEqNodeCheck(t *testing.T) {
	filename := "fail-nonconditional-no-eq-node-check.yaml"
	check := NonconditionalNoEqNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "eq", "map[default:noop yes:noop]", "accepted nonconditional sequence with `eq` field, expected error")
}

func TestFailRetryIfRetryWaitNodeCheck(t *testing.T) {
	filename := "fail-retry-if-retry-wait-node-check.yaml"
	check := RetryIfRetryWaitNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "retry", "accepted node with `retryWait` field with retry: 0, expected error")
}

func TestFailValidRetryWaitNodeCheck(t *testing.T) {
	filename := "fail-valid-retry-wait-node-check.yaml"
	check := ValidRetryWaitNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailInvalidValueError(t, err, true, "retryWait", "500", "accepted bad retryWait: duration, expected error")
}

func TestFailRequiredArgsProvidedNodeCheck1(t *testing.T) {
	filename := "fail-required-args-provided-node-check-1.yaml" // tests normal sequence node
	filepath := specsDir + filename
	allSpecs, _ := ParseSpec(filepath)

	check := RequiredArgsProvidedNodeCheck{allSpecs}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "args", "not all required args to seq-b listed in `args`, expected error")
}

func TestFailRequiredArgsProvidedNodeCheck2(t *testing.T) {
	filename := "fail-required-args-provided-node-check-2.yaml" // tests conditional node
	filepath := specsDir + filename
	allSpecs, _ := ParseSpec(filepath)

	check := RequiredArgsProvidedNodeCheck{allSpecs}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "args", "not all required args to seq-b listed in `args`, expected error")
}

func TestFailRequiredArgsProvidedNodeCheck3(t *testing.T) {
	filename := "fail-required-args-provided-node-check-3.yaml" // tests expanded sequence node
	filepath := specsDir + filename
	allSpecs, _ := ParseSpec(filepath)

	check := RequiredArgsProvidedNodeCheck{allSpecs}
	err := runNodeCheckOn(check, filename)

	testFailMissingValueError(t, err, true, "args", "not all required args to seq-b listed in `args`, expected error")
}

/* ========================================================================= */
// Helper functions

func runSequenceCheckOn(check SequenceCheck, filename string) error {
	filepath := specsDir + filename
	allSpecs, err := ParseSpec(filepath)
	if err != nil {
		return err
	}
	for _, sequence := range allSpecs.Sequences {
		if err := check.CheckSequence(*sequence); err != nil {
			return err
		}
	}
	return nil
}

func runNodeCheckOn(check NodeCheck, filename string) error {
	filepath := specsDir + filename
	allSpecs, err := ParseSpec(filepath)
	if err != nil {
		return err
	}
	for _, sequence := range allSpecs.Sequences {
		for _, node := range sequence.Nodes {
			if err := check.CheckNode(sequence.Name, *node); err != nil {
				return err
			}
		}
	}
	return nil
}

func testFailInvalidValueError(t *testing.T, err error, nodeLevel bool, field, expectedVal, errMsg string) {
	var node *string = nil
	if nodeLevel {
		n := "node-a"
		node = &n
	}
	expectedErr := InvalidValueError{"seq-a", node, field, expectedVal, ""}

	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case InvalidValueError:
			invalidValueErr := err.(InvalidValueError)
			invalidValueErr.Expected = ""
			if diff := deep.Equal(&invalidValueErr, &expectedErr); diff != nil {
				t.Error(diff)
			} else {
				fmt.Println(err.Error())
			}
		default:
			t.Errorf("expected InvalidValueError, got %T: %s", err, err)
		}
	}
}

func testFailMissingValueError(t *testing.T, err error, nodeLevel bool, field, errMsg string) {
	var node *string = nil
	if nodeLevel {
		n := "node-a"
		node = &n
	}
	expectedErr := MissingValueError{"seq-a", node, field, ""}

	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case MissingValueError:
			missingValueErr := err.(MissingValueError)
			missingValueErr.Explanation = ""
			if diff := deep.Equal(&missingValueErr, &expectedErr); diff != nil {
				t.Error(diff)
			} else {
				fmt.Println(err.Error())
			}
		default:
			t.Errorf("expected MissingValueError, got %T: %s", err, err)
		}
	}
}

func testFailDuplicateValueError(t *testing.T, err error, nodeLevel bool, field, expectedVal, errMsg string) {
	var node *string = nil
	if nodeLevel {
		n := "node-a"
		node = &n
	}
	expectedErr := DuplicateValueError{"seq-a", node, field, expectedVal}

	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case DuplicateValueError:
			duplicateValueErr := err.(DuplicateValueError)
			if diff := deep.Equal(&duplicateValueErr, &expectedErr); diff != nil {
				t.Error(diff)
			} else {
				fmt.Println(err.Error())
			}
		default:
			t.Errorf("expected DuplicateValueError, got %T: %s", err, err)
		}
	}
}
