// Copyright 2020, Square, Inc.

package spec_test

import (
	"fmt"
	"testing"

	. "github.com/square/spincycle/v2/request-manager/spec"
)

// Dummy check objects that always fail or always pass.
type PassSequenceCheck struct{}

func (c PassSequenceCheck) CheckSequence(s Sequence) error {
	return nil
}

type PassNodeCheck struct{}

func (c PassNodeCheck) CheckNode(n Node) error {
	return nil
}

var failSequenceCheckError = fmt.Errorf("FailSequenceCheck failed")
var failNodeCheckError = fmt.Errorf("FailNodeCheck failed")

type FailSequenceCheck struct{}

func (c FailSequenceCheck) CheckSequence(s Sequence) error {
	return failSequenceCheckError
}

type FailNodeCheck struct{}

func (c FailNodeCheck) CheckNode(n Node) error {
	return failNodeCheckError
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
	checker, err := NewChecker([]CheckFactory{PassFact{}})
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	seqErrors, seqWarnings := checker.RunChecks(specs)
	if len(seqErrors) > 0 {
		t.Errorf("RunChecks reports some error, expected success: %v", seqErrors)
	}
	if len(seqWarnings) > 0 {
		t.Errorf("RunChecks reports some warning, expected success: %v", seqWarnings)
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
	checker, err := NewChecker([]CheckFactory{PassWithWarning{}})
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	seqErrors, seqWarnings := checker.RunChecks(specs)
	if len(seqErrors) > 0 {
		t.Errorf("RunChecks reports some error, expected success: %v", seqErrors)
	}
	if len(seqWarnings) == 0 {
		t.Errorf("RunChecks reports no warnings, expected some warning")
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
	checker, err := NewChecker([]CheckFactory{FailFact{}})
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	seqErrors, seqWarnings := checker.RunChecks(specs)
	if len(seqErrors) == 0 {
		t.Errorf("RunChecks reports no error, expected failure")
	}
	if len(seqWarnings) == 0 {
		t.Errorf("RunChecks reports no warnings, expected some warning")
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
	checker, err := NewChecker([]CheckFactory{FailNoWarningFact{}})
	if err != nil {
		t.Fatalf("Error creating checker: %s", err)
	}
	seqErrors, seqWarnings := checker.RunChecks(specs)
	if len(seqErrors) == 0 {
		t.Errorf("RunChecks reports no error, expected failure")
	}

	if len(seqWarnings) > 0 {
		t.Errorf("RunChecks reports some warning, expected none: %v", seqWarnings)
	}
}
