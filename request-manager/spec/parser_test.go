// Copyright 2017-2020, Square, Inc.

package spec

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"testing"
)

/* ========================================================================= */
// Parse tests

func TestParseSpec(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"
	_, err, _ := ParseSpec(sequencesFile)
	if err != nil {
		t.Errorf("failed to read decomm.yaml, expected success")
	}
}

func TestFailParseSpec(t *testing.T) {
	sequencesFile := specsDir + "fail-parse-spec.yaml" // mistmatched type
	_, err, _ := ParseSpec(sequencesFile)
	if err == nil {
		t.Errorf("unmarshaled string into uint")
	} else {
		switch err.(type) {
		case *yaml.TypeError:
			fmt.Println(err.Error())
		default:
			t.Errorf("expected yaml.TypeError, got %T: %s", err, err)
		}
	}
}

func TestWarnParseSpec(t *testing.T) {
	sequencesFile := specsDir + "warn-parse-spec.yaml" // duplicated field
	_, _, warn := ParseSpec(sequencesFile)
	if warn == nil {
		t.Errorf("failed to give warning for duplicated field")
	} else {
		switch warn.(type) {
		case *yaml.TypeError:
			fmt.Println(warn.Error())
		default:
			t.Errorf("expected yaml.TypeError, got %T: %s", warn, warn.Error())
		}
	}
}

/* ========================================================================= */
// Make sure checks run correctly

func TestRunChecks(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"
	allSpecs, _, _ := ParseSpec(sequencesFile)
	err, warn := RunChecks(allSpecs)
	if len(err) > 0 {
		t.Errorf("decomm.yaml failed check, expected success: %v", err)
	}
	if len(warn) > 0 {
		t.Errorf("decomm.yaml produced warnings, expected none: %v", warn)
	}
}
