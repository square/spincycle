// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"testing"

	"github.com/go-test/deep"

	rmtest "github.com/square/spincycle/v2/request-manager/test"
)

var (
	specsDir = rmtest.SpecPath + "/"
)

var (
	printf = func(s string, args ...interface{}) { fmt.Printf(s, args...) }
)

func runSequenceCheckOn(check SequenceCheck, filename string) error {
	filepath := specsDir + filename
	allSpecs, err := ParseSpec(filepath, printf)
	if err != nil {
		return err
	}
	for _, sequence := range allSpecs.Sequences {
		if err := check.CheckSequence(*sequence); err != nil {
			return err
		}
	}
	return nil
}

func runNodeCheckOn(check NodeCheck, filename string) error {
	filepath := specsDir + filename
	allSpecs, err := ParseSpec(filepath, printf)
	if err != nil {
		return err
	}
	for _, sequence := range allSpecs.Sequences {
		for _, node := range sequence.Nodes {
			if err := check.CheckNode(sequence.Name, *node); err != nil {
				return err
			}
		}
	}
	return nil
}

func testFailInvalidValueError(t *testing.T, err error, nodeLevel bool, field, expectedVal, errMsg string) {
	var node *string = nil
	if nodeLevel {
		n := "node-a"
		node = &n
	}
	expectedErr := InvalidValueError{"seq-a", node, field, []string{expectedVal}, ""}

	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case InvalidValueError:
			invalidValueErr := err.(InvalidValueError)
			invalidValueErr.Expected = ""
			if diff := deep.Equal(&invalidValueErr, &expectedErr); diff != nil {
				t.Error(diff)
			} else {
				fmt.Println(err.Error())
			}
		default:
			t.Errorf("expected InvalidValueError, got %T: %s", err, err)
		}
	}
}

func testFailMissingValueError(t *testing.T, err error, nodeLevel bool, field, errMsg string) {
	var node *string = nil
	if nodeLevel {
		n := "node-a"
		node = &n
	}
	expectedErr := MissingValueError{"seq-a", node, field, ""}

	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case MissingValueError:
			missingValueErr := err.(MissingValueError)
			missingValueErr.Explanation = ""
			if diff := deep.Equal(&missingValueErr, &expectedErr); diff != nil {
				t.Error(diff)
			} else {
				fmt.Println(err.Error())
			}
		default:
			t.Errorf("expected MissingValueError, got %T: %s", err, err)
		}
	}
}

func testFailDuplicateValueError(t *testing.T, err error, nodeLevel bool, field, expectedVal, errMsg string) {
	var node *string = nil
	if nodeLevel {
		n := "node-a"
		node = &n
	}
	expectedErr := DuplicateValueError{"seq-a", node, field, []string{expectedVal}, ""}

	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case DuplicateValueError:
			duplicateValueErr := err.(DuplicateValueError)
			duplicateValueErr.Explanation = ""
			if diff := deep.Equal(&duplicateValueErr, &expectedErr); diff != nil {
				t.Error(diff)
			} else {
				fmt.Println(err.Error())
			}
		default:
			t.Errorf("expected DuplicateValueError, got %T: %s", err, err)
		}
	}
}
