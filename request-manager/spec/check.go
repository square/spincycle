// Copyright 2017-2020, Square, Inc.

package spec

/* Runs checks on allSpecs. */
func RunChecks(allSpecs Specs) (errors, warnings []error) {
	sequenceErrors := makeSequenceErrorChecks()
	nodeErrors := makeNodeErrorChecks(allSpecs)
	nodeWarnings := makeNodeWarningChecks()

	for _, sequence := range allSpecs.Sequences {
		for _, sequenceCheck := range sequenceErrors {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				errors = append(errors, err)
			}
		}

		for _, node := range sequence.Nodes {
			for _, nodeCheck := range nodeErrors {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					errors = append(errors, err)
				}
			}
			for _, nodeCheck := range nodeWarnings {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					warnings = append(warnings, err)
				}
			}
		}
	}

	return
}

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