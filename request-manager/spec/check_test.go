// Copyright 2017-2020, Square, Inc.

package spec

import (
	"testing"
)

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
