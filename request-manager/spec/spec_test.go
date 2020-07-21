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
	_, err := ParseSpec(sequencesFile)
	if err != nil {
		t.Errorf("failed to read decomm.yaml, expected success")
	}
}

func TestFailWrongType(t *testing.T) {
	sequencesFile := specsDir + "wrong-type.yaml"
	_, err := ParseSpec(sequencesFile)
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
