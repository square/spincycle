// Copyright 2020, Square, Inc.

package spec

type SequenceCheck interface {
	CheckSequence(SequenceSpec) error
}

/* ========================================================================== */
type RequiredArgsNamedSequenceCheck struct{}

/* Sequence args must be named, i.e. include a `name` field. */
func (check RequiredArgsNamedSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, arg := range sequence.Args.Required {
		if arg.Name == nil {
			return MissingValueError{sequence.Name, nil, "required -> name", "required for required args"}
		}
	}

	return nil
}

/* ========================================================================== */
type OptionalArgsNamedSequenceCheck struct{}

/* Sequence args must be named, i.e. include a `name` field. */
func (check OptionalArgsNamedSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, arg := range sequence.Args.Optional {
		if arg.Name == nil {
			return MissingValueError{sequence.Name, nil, "optional -> name", "required for optional args"}
		}
	}

	return nil
}

/* ========================================================================== */
type StaticArgsNamedSequenceCheck struct{}

/* Sequence args must be named, i.e. include a `name` field. */
func (check StaticArgsNamedSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, arg := range sequence.Args.Static {
		if arg.Name == nil {
			return MissingValueError{sequence.Name, nil, "static -> name", "required for static args"}
		}
	}

	return nil
}

/* ========================================================================== */
type OptionalArgsHaveDefaultsSequenceCheck struct{}

/* Optional args must have defaults. */
func (check OptionalArgsHaveDefaultsSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, arg := range sequence.Args.Optional {
		if arg.Default == nil {
			return MissingValueError{sequence.Name, nil, "optional -> default", "required for optional args"}
		}
	}

	return nil
}

/* ========================================================================== */
type StaticArgsHaveDefaultsSequenceCheck struct{}

/* Static args must have defaults. */
func (check StaticArgsHaveDefaultsSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, arg := range sequence.Args.Static {
		if arg.Default == nil {
			return MissingValueError{sequence.Name, nil, "static -> default", "required for static args"}
		}
	}

	return nil
}

/* ========================================================================== */
type NoDuplicateArgsSequenceCheck struct{}

/* Args should appear once per sequence. */
func (check NoDuplicateArgsSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, args := range [][]*ArgSpec{sequence.Args.Required, sequence.Args.Optional, sequence.Args.Static} {
		for _, arg := range args {
			if arg.Name != nil && seen[*arg.Name] {
				values[*arg.Name] = true
			}
			seen[*arg.Name] = true
		}
	}

	if len(values) > 0 {
		return DuplicateValueError{sequence.Name, nil, "sequence args -> name", stringSetToArray(values), "sequence args must have unique names within the sequence"}
	}

	return nil
}

/* ========================================================================== */
type HasNodesSequenceCheck struct{}

/* Sequences must contain nodes i.e. are not empty. */
func (check HasNodesSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	if len(sequence.Nodes) == 0 {
		return MissingValueError{sequence.Name, nil, "nodes", "at least one node required"}
	}

	return nil
}

/* ========================================================================== */
type AdminXorOpsSequenceCheck struct{}

/* `admin` and `ops` are mutually exclusive. */
func (check AdminXorOpsSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, acl := range sequence.ACL {
		if acl.Admin && len(acl.Ops) != 0 {
			return InvalidValueError{sequence.Name, nil, "acl -> admin", []string{"true"}, "admin=false; alternatively, remove ops"}
		}
	}

	return nil
}

/* ========================================================================== */
type AclsHaveRolesSequenceCheck struct{}

/* ACLs must specify a role. */
func (check AclsHaveRolesSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	for _, acl := range sequence.ACL {
		if acl.Role == "" {
			return MissingValueError{sequence.Name, nil, "acl -> role", "field is required"}
		}
	}

	return nil
}

/* ========================================================================== */
type NoDuplicateAclRolesSequenceCheck struct{}

/* ACL roles must not be duplicated. */
func (check NoDuplicateAclRolesSequenceCheck) CheckSequence(sequence SequenceSpec) error {
	seen := map[string]bool{}
	values := map[string]bool{}
	for _, acl := range sequence.ACL {
		if seen[acl.Role] {
			values[acl.Role] = true
		}
		seen[acl.Role] = true
	}

	if len(values) > 0 {
		return DuplicateValueError{sequence.Name, nil, "acl -> role", stringSetToArray(values), ""}
	}

	return nil
}
