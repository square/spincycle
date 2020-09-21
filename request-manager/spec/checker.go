// Copyright 2020, Square, Inc.

package spec

// Runs checks on sequence and node specs.
type Checker struct {
	// Checks to run. ErrorChecks are fatal on failure. Warnings are not.
	sequenceErrorChecks   []SequenceCheck
	sequenceWarningChecks []SequenceCheck
	nodeErrorChecks       []NodeCheck
	nodeWarningChecks     []NodeCheck
}

// Create a new Checker with the checks specified by check factories in list.
func NewChecker(checkFactories []CheckFactory) (*Checker, error) {
	checker := &Checker{
		sequenceErrorChecks:   []SequenceCheck{},
		sequenceWarningChecks: []SequenceCheck{},
		nodeErrorChecks:       []NodeCheck{},
		nodeWarningChecks:     []NodeCheck{},
	}

	for _, factory := range checkFactories {
		sec, err := factory.MakeSequenceErrorChecks()
		if err != nil {
			return nil, err
		}
		checker.sequenceErrorChecks = append(checker.sequenceErrorChecks, sec...)

		swc, err := factory.MakeSequenceWarningChecks()
		if err != nil {
			return nil, err
		}
		checker.sequenceWarningChecks = append(checker.sequenceWarningChecks, swc...)

		nec, err := factory.MakeNodeErrorChecks()
		if err != nil {
			return nil, err
		}
		checker.nodeErrorChecks = append(checker.nodeErrorChecks, nec...)

		nwc, err := factory.MakeNodeWarningChecks()
		if err != nil {
			return nil, err
		}
		checker.nodeWarningChecks = append(checker.nodeWarningChecks, nwc...)
	}

	return checker, nil
}

// Runs checks on allSpecs.
func (checker *Checker) RunChecks(allSpecs Specs) *CheckResults {
	results := NewCheckResults()

	for name, sequence := range allSpecs.Sequences {
		for _, sequenceCheck := range checker.sequenceErrorChecks {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				results.AddError(name, err)
			}
		}
		for _, sequenceCheck := range checker.sequenceWarningChecks {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				results.AddWarning(name, err)
			}
		}

		for _, node := range sequence.Nodes {
			for _, nodeCheck := range checker.nodeErrorChecks {
				if err := nodeCheck.CheckNode(*node); err != nil {
					results.AddError(name, err)
				}
			}
			for _, nodeCheck := range checker.nodeWarningChecks {
				if err := nodeCheck.CheckNode(*node); err != nil {
					results.AddWarning(name, err)
				}
			}
		}
	}

	return results
}
