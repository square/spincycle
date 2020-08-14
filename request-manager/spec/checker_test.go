// Copyright 2020, Square, Inc.

package spec_test

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/square/spincycle/v2/request-manager/spec"
)

// Dummy check objects that always fail or always pass.
type PassSequenceCheck struct{}

func (c PassSequenceCheck) CheckSequence(s Sequence) error {
	return nil
}

type PassNodeCheck struct{}

func (c PassNodeCheck) CheckNode(s string, n Node) error {
	return nil
}

var failSequenceCheckText = "FailSequenceCheck failed"
var failNodeCheckText = "FailNodeCheck failed"

type FailSequenceCheck struct{}

func (c FailSequenceCheck) CheckSequence(s Sequence) error {
	return fmt.Errorf(failSequenceCheckText)
}

type FailNodeCheck struct{}

func (c FailNodeCheck) CheckNode(s string, n Node) error {
	return fmt.Errorf(failNodeCheckText)
}

// Dummy sequence specs. Contains the absolute minimum of one sequence and one node
// to give our dummy check objects something to run on.
var specs = Specs{
	Sequences: map[string]*Sequence{
		"seq": &Sequence{
			Name: "seq",
			Nodes: map[string]*Node{
				"node": &Node{Name: "node"},
			},
		},
	},
}

/* ========================================================================== */
// The actual tests, which each (necessarily) have their own mock check factory.

/* ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++ */
// Pass with no warnings.
type PassFact struct{}

func (f PassFact) MakeSequenceErrorChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{PassSequenceCheck{}}, nil
}
func (f PassFact) MakeSequenceWarningChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{PassSequenceCheck{}}, nil
}
func (f PassFact) MakeNodeErrorChecks() ([]NodeCheck, error) {
	return []NodeCheck{PassNodeCheck{}}, nil
}
func (f PassFact) MakeNodeWarningChecks() ([]NodeCheck, error) {
	return []NodeCheck{PassNodeCheck{}}, nil
}
func TestPassRunChecks(t *testing.T) {
	var log []string
	logf := func(s string, args ...interface{}) { log = append(log, fmt.Sprintf(s, args...)) }

	checker, err := NewChecker([]CheckFactory{PassFact{}}, logf)
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	ok := checker.RunChecks(specs)
	if !ok {
		t.Error("RunChecks reports some error, expected success")
	}

	// Check that nothing was printed (if anything was printed, it's an error or warning)
	if len(log) > 0 {
		t.Errorf("log printed something (presumed to be warning or error): %s", log)
	}
}

/* ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++ */
// Pass, with warnings.
type PassWithWarning struct{}

func (f PassWithWarning) MakeSequenceErrorChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{PassSequenceCheck{}}, nil
}
func (f PassWithWarning) MakeSequenceWarningChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{FailSequenceCheck{}}, nil
}
func (f PassWithWarning) MakeNodeErrorChecks() ([]NodeCheck, error) {
	return []NodeCheck{PassNodeCheck{}}, nil
}
func (f PassWithWarning) MakeNodeWarningChecks() ([]NodeCheck, error) {
	return []NodeCheck{FailNodeCheck{}}, nil
}
func TestPassWithWarningRunChecks(t *testing.T) {
	var log []string
	logf := func(s string, args ...interface{}) { log = append(log, fmt.Sprintf(s, args...)) }

	checker, err := NewChecker([]CheckFactory{PassWithWarning{}}, logf)
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	ok := checker.RunChecks(specs)
	if !ok {
		t.Error("RunChecks reports some error, expected success")
	}

	// Check that all warnings were printed properly
	seqWarningFound := false
	nodeWarningFound := false
	for _, output := range log {
		if strings.Contains(strings.ToLower(output), "warning") {
			if strings.Contains(output, failSequenceCheckText) {
				seqWarningFound = true
			} else if strings.Contains(output, failNodeCheckText) {
				nodeWarningFound = true
			}
		}
	}
	if !seqWarningFound {
		t.Errorf("Expected but could not find sequence check warning in log: %s", log)
	}
	if !nodeWarningFound {
		t.Errorf("Expected but could not find node check warning in log: %s", log)
	}
}

/* ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++ */
// Fail wildly.
type FailFact struct{}

func (f FailFact) MakeSequenceErrorChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{FailSequenceCheck{}}, nil
}
func (f FailFact) MakeSequenceWarningChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{FailSequenceCheck{}}, nil
}
func (f FailFact) MakeNodeErrorChecks() ([]NodeCheck, error) {
	return []NodeCheck{FailNodeCheck{}}, nil
}
func (f FailFact) MakeNodeWarningChecks() ([]NodeCheck, error) {
	return []NodeCheck{FailNodeCheck{}}, nil
}
func TestFailRunChecks(t *testing.T) {
	var log []string
	logf := func(s string, args ...interface{}) { log = append(log, fmt.Sprintf(s, args...)) }

	checker, err := NewChecker([]CheckFactory{FailFact{}}, logf)
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	ok := checker.RunChecks(specs)
	if ok {
		t.Error("RunChecks reports no error, expected failure")
	}

	// Check that all errors were printed properly
	seqErrorFound := false
	nodeErrorFound := false
	for _, output := range log {
		if strings.Contains(strings.ToLower(output), "error") {
			if strings.Contains(output, failSequenceCheckText) {
				seqErrorFound = true
			} else if strings.Contains(output, failNodeCheckText) {
				nodeErrorFound = true
			}
		}
	}
	if !seqErrorFound {
		t.Errorf("Expected but could not find sequence check error in log: %s", log)
	}
	if !nodeErrorFound {
		t.Errorf("Expected but could not find node check error in log: %s", log)
	}

	// Check that all warnings were printed properly
	seqWarningFound := false
	nodeWarningFound := false
	for _, output := range log {
		if strings.Contains(strings.ToLower(output), "warning") {
			if strings.Contains(output, failSequenceCheckText) {
				seqWarningFound = true
			} else if strings.Contains(output, failNodeCheckText) {
				nodeWarningFound = true
			}
		}
	}
	if !seqWarningFound {
		t.Errorf("Expected but could not find sequence check warning in log: %s", log)
	}
	if !nodeWarningFound {
		t.Errorf("Expected but could not find node check warning in log: %s", log)
	}
}

/* ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++ */
// Fail, but with no warnings.
type FailNoWarningFact struct{}

func (f FailNoWarningFact) MakeSequenceErrorChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{FailSequenceCheck{}}, nil
}
func (f FailNoWarningFact) MakeSequenceWarningChecks() ([]SequenceCheck, error) {
	return []SequenceCheck{PassSequenceCheck{}}, nil
}
func (f FailNoWarningFact) MakeNodeErrorChecks() ([]NodeCheck, error) {
	return []NodeCheck{FailNodeCheck{}}, nil
}
func (f FailNoWarningFact) MakeNodeWarningChecks() ([]NodeCheck, error) {
	return []NodeCheck{PassNodeCheck{}}, nil
}
func TestFailNoWarningRunChecks(t *testing.T) {
	var log []string
	logf := func(s string, args ...interface{}) { log = append(log, fmt.Sprintf(s, args...)) }

	checker, err := NewChecker([]CheckFactory{FailNoWarningFact{}}, logf)
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	ok := checker.RunChecks(specs)
	if ok {
		t.Error("RunChecks reports no error, expected failure")
	}

	// Check that no warnings were printed
	warningFound := false
	for _, output := range log {
		if strings.Contains(strings.ToLower(output), "warning") {
			warningFound = true
		}
	}
	if warningFound {
		t.Errorf("Unexpected warning found in log: %s", log)
	}
}
