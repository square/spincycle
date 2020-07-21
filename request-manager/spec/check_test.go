// Copyright 2017-2020, Square, Inc.

package spec

import (
	"testing"
)

/* ========================================================================= */
// Make sure checks run correctly

func TestRunChecks(t *testing.T) {
	sequencesFile := specsDir + "decomm.yaml"
	allSpecs, _ := ParseSpec(sequencesFile)
	err := RunChecks(allSpecs)
	if err != nil {
		t.Errorf("decomm.yaml failed check, expected success: %s", err.Error())
	}
}
