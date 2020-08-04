// Copyright 2020, Square, Inc.

package spec

type Checker struct {
	sequenceErrorChecks   []SequenceCheck
	sequenceWarningChecks []SequenceCheck
	nodeErrorChecks       []NodeCheck
	nodeWarningChecks     []NodeCheck
	logFunc               func(string, ...interface{})
}

func NewChecker(checkFactories []CheckFactory, logFunc func(string, ...interface{})) (*Checker, error) {
	checker := &Checker{
		sequenceErrorChecks:   []SequenceCheck{},
		sequenceWarningChecks: []SequenceCheck{},
		nodeErrorChecks:       []NodeCheck{},
		nodeWarningChecks:     []NodeCheck{},
		logFunc:               logFunc,
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
// 'logFunc' is a Printf-like function used to log warnings and errors should they occur.
// If any error is logged, this function returns an error.
func (checker *Checker) RunChecks(allSpecs Specs) bool {
	errOccurred := false

	for _, sequence := range allSpecs.Sequences {
		for _, sequenceCheck := range checker.sequenceErrorChecks {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				checker.logFunc("Error: %s", err)
				errOccurred = true
			}
		}
		for _, sequenceCheck := range checker.sequenceWarningChecks {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				checker.logFunc("Warning: %s", err)
			}
		}

		for _, node := range sequence.Nodes {
			for _, nodeCheck := range checker.nodeErrorChecks {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					checker.logFunc("Error: %s", err)
					errOccurred = true
				}
			}
			for _, nodeCheck := range checker.nodeWarningChecks {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					checker.logFunc("Warning: %s", err)
				}
			}
		}
	}

	return !errOccurred
}
