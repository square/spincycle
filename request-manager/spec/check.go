// Copyright 2017-2020, Square, Inc.

package spec

import (
	"fmt"
)

/* Runs checks on allSpecs. */
func RunChecks(allSpecs Specs) error {
	sequenceErrors := makeSequenceErrors()
	nodeErrors := makeNodeErrors(allSpecs)
	nodeWarnings := makeNodeWarnings()

	for _, sequence := range allSpecs.Sequences {
		for _, sequenceCheck := range sequenceErrors {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				return err
			}
		}

		for _, node := range sequence.Nodes {
			for _, nodeCheck := range nodeErrors {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					return err
				}
			}
			for _, nodeCheck := range nodeWarnings {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					fmt.Printf("Warning: %s\n", err.Error())
				}
			}
		}
	}

	return nil
}

/* ========================================================================== */
/* Add new checks here. Order shouldn't matter. */

func makeSequenceErrors() []SequenceCheck {
	return []SequenceCheck{
		RequiredArgsNamedSequenceCheck{},
		OptionalArgsNamedSequenceCheck{},
		StaticArgsNamedSequenceCheck{},

		OptionalArgsHaveDefaultsSequenceCheck{},
		StaticArgsHaveDefaultsSequenceCheck{},

		HasNodesSequenceCheck{},

		AdminXorOpsSequenceCheck{},
		AclsHaveRolesSequenceCheck{},
		NoDuplicateAclRolesSequenceCheck{},
	}
}

func makeNodeErrors(allSpecs Specs) []NodeCheck {
	return []NodeCheck{
		HasCategoryNodeCheck{},
		ValidCategoryNodeCheck{},

		ValidEachNodeCheck{},

		ArgsAreNamedNodeCheck{},
		ArgsOnceNodeCheck{},
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

func makeNodeWarnings() []NodeCheck {
	return []NodeCheck{
		ArgsNotRenamedNodeCheck{},
		SetsNotRenamedNodeCheck{},
	}
}
