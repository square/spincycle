// Copyright 2017-2020, Square, Inc.

package spec

import (
	"testing"
)

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

func TestFailEachOnceNodeCheck(t *testing.T) {
	filename := "fail-each-once-node-check.yaml"
	check := EachOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "each", "alias-a", "accepted duplicated each alias, expected error")
}

func TestFailEachNotRenamedNodeCheck(t *testing.T) {
	filename := "fail-each-not-renamed-node-check.yaml"
	check := EachNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "each", "arg-a", "accepted duplicated each arg, expected error")
}

func TestFailArgsAreNamedNodeCheck(t *testing.T) {
	filename := "fail-args-are-named-node-check.yaml"
	check := ArgsAreNamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "args -> expected", "accepted node arg without `expected` field, expected error")
}

func TestFailArgsOnceNodeCheck1(t *testing.T) {
	filename := "fail-args-once-node-check-1.yaml"
	check := ArgsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args -> expected", "arg-a", "accepted duplicated args, expected error")
}

func TestFailArgsOnceNodeCheck2(t *testing.T) {
	filename := "fail-args-once-node-check-2.yaml"
	check := ArgsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args -> expected", "arg-a", "accepted duplicated args, expected error")
}

func TestFailArgsOnceNodeCheck3(t *testing.T) {
	filename := "fail-args-once-node-check-3.yaml"
	check := ArgsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args -> expected", "arg-a", "accepted duplicated args, expected error")
}

func TestFailArgsNotRenamedNodeCheck(t *testing.T) {
	filename := "fail-args-not-renamed-node-check.yaml"
	check := ArgsNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "args -> given", "arg-a", "arg renamed differently with args -> given, expected error")
}

func TestFailArgsEachOnceNodeCheck(t *testing.T) {
	filename := "fail-args-each-once-node-check.yaml"
	check := ArgsEachOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "each", "alias-a", "allowed two args to be renamed alias-a, expected error")
}

func TestFailArgsEachNotRenamedNodeCheck(t *testing.T) {
	filename := "fail-args-each-not-renamed-node-check.yaml"
	check := ArgsEachNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "each", "arg-a", "allowed arg-a to be renamed twice, expected error")
}

func TestArgsEachNotRenamedNodeCheck(t *testing.T) {
	filename := "args-each-not-renamed-node-check.yaml"
	check := ArgsEachNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestFailSetsAreNamedNodeCheck(t *testing.T) {
	filename := "fail-sets-are-named-node-check.yaml"
	check := SetsAreNamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "sets -> arg", "accepted node sets without `arg` field, expected error")
}

func TestFailSetsOnceNodeCheck1(t *testing.T) {
	filename := "fail-sets-once-node-check-1.yaml"
	check := SetsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets -> as", "arg-a", "accepted duplicated sets, expected error")
}

func TestFailSetsOnceNodeCheck2(t *testing.T) {
	filename := "fail-sets-once-node-check-2.yaml"
	check := SetsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets -> as", "arg-x", "accepted duplicated sets, expected error")
}

func TestFailSetsOnceNodeCheck3(t *testing.T) {
	filename := "fail-sets-once-node-check-3.yaml"
	check := SetsOnceNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets -> as", "arg-x", "accepted duplicated sets, expected error")
}

func TestFailSetsNotRenamedNodeCheck(t *testing.T) {
	filename := "fail-sets-not-renamed-node-check.yaml"
	check := SetsNotRenamedNodeCheck{}
	err := runNodeCheckOn(check, filename)
	testFailDuplicateValueError(t, err, true, "sets -> arg", "arg-a", "arg renamed differently with sets -> as, expected error")
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
	allSpecs, _, _ := ParseSpec(filepath)

	check := RequiredArgsProvidedNodeCheck{allSpecs}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "args", "not all required args to seq-b listed in `args`, expected error")
}

func TestFailRequiredArgsProvidedNodeCheck2(t *testing.T) {
	filename := "fail-required-args-provided-node-check-2.yaml" // tests conditional node
	filepath := specsDir + filename
	allSpecs, _, _ := ParseSpec(filepath)

	check := RequiredArgsProvidedNodeCheck{allSpecs}
	err := runNodeCheckOn(check, filename)
	testFailMissingValueError(t, err, true, "args", "not all required args to seq-b listed in `args`, expected error")
}

func TestFailRequiredArgsProvidedNodeCheck3(t *testing.T) {
	filename := "fail-required-args-provided-node-check-3.yaml" // tests expanded sequence node
	filepath := specsDir + filename
	allSpecs, _, _ := ParseSpec(filepath)

	check := RequiredArgsProvidedNodeCheck{allSpecs}
	err := runNodeCheckOn(check, filename)

	testFailMissingValueError(t, err, true, "args", "not all required args to seq-b listed in `args`, expected error")
}
