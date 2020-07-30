// Copyright 2020, Square, Inc.

package spec

/* ========================================================================== */
/* Add new checks here. Order shouldn't matter. */

func makeSequenceErrorChecks() []SequenceCheck {
	return []SequenceCheck{
		RequiredArgsNamedSequenceCheck{},
		OptionalArgsNamedSequenceCheck{},
		StaticArgsNamedSequenceCheck{},

		OptionalArgsHaveDefaultsSequenceCheck{},
		StaticArgsHaveDefaultsSequenceCheck{},

		NoDuplicateArgsSequenceCheck{},

		HasNodesSequenceCheck{},

		AdminXorOpsSequenceCheck{},
		AclsHaveRolesSequenceCheck{},
		NoDuplicateAclRolesSequenceCheck{},
	}
}

func makeNodeErrorChecks(allSpecs Specs) []NodeCheck {
	return []NodeCheck{
		HasCategoryNodeCheck{},
		ValidCategoryNodeCheck{},

		ValidEachNodeCheck{},
		EachOnceNodeCheck{},
		ArgsAreNamedNodeCheck{},
		ArgsOnceNodeCheck{},
		ArgsEachOnceNodeCheck{},
		SetsAreNamedNodeCheck{},
		SetsOnceNodeCheck{},

		EachIfParallelNodeCheck{},
		ValidParallelNodeCheck{},

		ConditionalNoTypeNodeCheck{},
		ConditionalHasIfNodeCheck{},
		ConditionalHasEqNodeCheck{},
		NonconditionalHasTypeNodeCheck{},
		NonconditionalNoIfNodeCheck{},
		NonconditionalNoEqNodeCheck{},

		RetryIfRetryWaitNodeCheck{},
		ValidRetryWaitNodeCheck{},

		RequiredArgsProvidedNodeCheck{allSpecs},
	}
}

func makeNodeWarningChecks() []NodeCheck {
	return []NodeCheck{
		EachNotRenamedNodeCheck{},
		ArgsNotRenamedNodeCheck{},
		ArgsEachNotRenamedNodeCheck{},
		SetsNotRenamedNodeCheck{},
	}
}