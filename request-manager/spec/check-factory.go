// Copyright 2020, Square, Inc.

package spec

// Generates static checks to be performed on sequence and node specs.
//
// Errors are mistakes in the specs that prevent the RM from functioning
// properly. For example, when a node specifies an invalid value.
// Errors can also be used to enforce reasonable conventions. For example,
// sequences should have nodes. Presumably, if a sequence has no nodes, this
// was an oversight on the writer's part.
// If any error occurs, the RM will fail to boot.
//
// Warnings identify probable mistakes in the specs. For example, an arg
// that is renamed twice using sets/arg/as syntax.
// Warnings will be logged, but will not cause the RM to fail to boot.
type CheckFactory interface {
	MakeSequenceErrorChecks() ([]SequenceCheck, error)
	MakeSequenceWarningChecks() ([]SequenceCheck, error)
	MakeNodeErrorChecks() ([]NodeCheck, error)
	MakeNodeWarningChecks() ([]NodeCheck, error)
}

// The absolute minimum of checks for the RM to work properly.
type BaseCheckFactory struct {
	AllSpecs Specs // All specs in specs dir
}

func (c BaseCheckFactory) MakeSequenceErrorChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{
		RequiredArgsNamedSequenceCheck{},
		OptionalArgsNamedSequenceCheck{},
		StaticArgsNamedSequenceCheck{},

		OptionalArgsHaveDefaultsSequenceCheck{},
		StaticArgsHaveDefaultsSequenceCheck{},

		ACLAdminXorOpsSequenceCheck{},
		ACLsHaveRolesSequenceCheck{},
		NoDuplicateACLRolesSequenceCheck{},
	}, nil
}

func (c BaseCheckFactory) MakeSequenceWarningChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{}, nil
}

func (c BaseCheckFactory) MakeNodeErrorChecks() ([]NodeCheck, error) {
	return []NodeCheck{
		HasCategoryNodeCheck{},
		ValidCategoryNodeCheck{},

		ValidEachNodeCheck{},
		ArgsAreNamedNodeCheck{},
		SetsAreNamedNodeCheck{},

		ValidParallelNodeCheck{},

		ConditionalHasIfNodeCheck{},
		ConditionalHasEqNodeCheck{},
		NonconditionalHasTypeNodeCheck{},

		ValidRetryWaitNodeCheck{},

		RequiredArgsProvidedNodeCheck{c.AllSpecs},
	}, nil
}

func (c BaseCheckFactory) MakeNodeWarningChecks() ([]NodeCheck, error) {
	return []NodeCheck{}, nil
}

// Some default checks. Not strictly necessary for proper operation of RM, but generally reasonable.
type DefaultCheckFactory struct{}

func (c DefaultCheckFactory) MakeSequenceErrorChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{
		NoDuplicateArgsSequenceCheck{},
		HasNodesSequenceCheck{},
	}, nil
}

func (c DefaultCheckFactory) MakeSequenceWarningChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{}, nil
}

func (c DefaultCheckFactory) MakeNodeErrorChecks() ([]NodeCheck, error) {
	return []NodeCheck{
		ArgsExpectedUniqueNodeCheck{},
		EachAliasDoesNotDuplicateArgsExpectedNodeCheck{},
		SetsAsUniqueNodeCheck{},

		EachIfParallelNodeCheck{},

		ConditionalNoTypeNodeCheck{},
		NonconditionalNoIfNodeCheck{},
		NonconditionalNoEqNodeCheck{},

		RetryIfRetryWaitNodeCheck{},
	}, nil
}

func (c DefaultCheckFactory) MakeNodeWarningChecks() ([]NodeCheck, error) {
	return []NodeCheck{
		EachNotRenamedTwiceNodeCheck{},
		ArgsNotRenamedTwiceNodeCheck{},
		EachAliasDoesNotDuplicateArgsExpectedNodeCheck{},
		SetsNotRenamedTwiceNodeCheck{},
	}, nil
}
