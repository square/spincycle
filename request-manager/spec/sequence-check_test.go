// Copyright 2017-2020, Square, Inc.

package spec

import (
	"testing"
)

/* ========================================================================= */
// SequenceCheck tests

func TestFailRequiredArgsNamedSequenceCheck(t *testing.T) {
	filename := "fail-required-args-named-sequence-check.yaml"
	check := RequiredArgsNamedSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "required -> name", "accepted sequence arg with no name, expected error")
}

func TestFailOptionalArgsHaveDefaultsSequenceCheck(t *testing.T) {
	filename := "fail-optional-args-have-defaults-sequence-check.yaml"
	check := OptionalArgsHaveDefaultsSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "optional -> default", "accepted optional arg with no default, expected error")
}

func TestFailStaticArgsHaveDefaultsSequenceCheck(t *testing.T) {
	filename := "fail-static-args-have-defaults-sequence-check.yaml"
	check := StaticArgsHaveDefaultsSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "static -> default", "accepted static arg with no default, expected error")
}

func TestFailNoDuplicateArgsSequenceCheck(t *testing.T) {
	filename := "fail-no-duplicate-args-sequence-check.yaml"
	check := NoDuplicateArgsSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailDuplicateValueError(t, err, false, "sequence args -> name", "arg-a", "accepted two instances of arg-a in sequence args, expected error")
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
	testFailInvalidValueError(t, err, false, "acl -> admin", "true", "accepted sequence specifying ops with admin=true, expected error")
}

func TestFailAclsHaveRolesSequenceCheck(t *testing.T) {
	filename := "fail-acls-have-roles-sequence-check.yaml"
	check := AclsHaveRolesSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailMissingValueError(t, err, false, "acl -> role", "accepted ACL with missing role, expected error")
}

func TestFailNoDuplicateAclRolesSequenceCheck(t *testing.T) {
	filename := "fail-no-duplicate-acl-roles-sequence-check.yaml"
	check := NoDuplicateAclRolesSequenceCheck{}
	err := runSequenceCheckOn(check, filename)
	testFailDuplicateValueError(t, err, false, "acl -> role", "eng", "accepted duplicated acl roles, expected error")
}
