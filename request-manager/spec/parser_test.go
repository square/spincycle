// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"strings"
	"testing"
)

func TestParseSpec(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"
	_, err := ParseSpec(sequencesFile, printf)
	if err != nil {
		t.Errorf("failed to read decomm.yaml, expected success")
	}
}

func TestFailParseSpec(t *testing.T) {
	sequencesFile := specsDir + "fail-parse-spec.yaml" // mistmatched type
	_, err := ParseSpec(sequencesFile, printf)
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

	var warning string
	logFunc := func(s string, args ...interface{}) { warning = fmt.Sprintf(s, args...) }

	ParseSpec(sequencesFile, logFunc)
	if warning == "" {
		t.Errorf("failed to give warning for duplicated field")
	} else if strings.Contains(strings.ToLower(warning), "warning") {
		fmt.Println(warning)
	} else {
		t.Errorf("expected warning containing 'warning' as substring, got: %s", warning)
	}
}

// Make sure checks run correctly
func TestRunChecks(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"

	var warning string
	logFunc := func(s string, args ...interface{}) { warning = fmt.Sprintf(s, args...) }

	allSpecs, _ := ParseSpec(sequencesFile, logFunc)
	err := RunChecks(allSpecs, logFunc)
	if err != nil {
		t.Errorf("decomm.yaml failed check, expected success: %v", err)
	}
	if strings.Contains(strings.ToLower(warning), "warning") {
		t.Errorf("decomm.yaml produced warnings, expected none: %v", warning)
	} else if warning != "" {
		t.Errorf("unexpected output by RunChecks via `logfunc`: %s", warning)
	}
}
