// Copyright 2020, Square, Inc.

package spec

import (
	"testing"

	"github.com/go-test/deep"

	rmtest "github.com/square/spincycle/v2/request-manager/test"
)

var (
	specsDir = rmtest.SpecPath + "/"
	seqA     = "seq-a"
	nodeA    = "node-a"
	value    = "value"
)

func compareError(t *testing.T, err, expectedErr error, errMsg string) {
	if err == nil {
		t.Errorf(errMsg)
	} else {
		switch err.(type) {
		case InvalidValueError:
			compareInvalidValueError(t, err, expectedErr.(InvalidValueError))
		case MissingValueError:
			compareMissingValueError(t, err, expectedErr.(MissingValueError))
		case DuplicateValueError:
			compareDuplicateValueError(t, err, expectedErr.(DuplicateValueError))
		default:
			t.Errorf("expected error should be of type InvalidValueError, MissingValueError, or DuplicateValueError; got type %T: %s", expectedErr, expectedErr)
		}
	}
}

func compareInvalidValueError(t *testing.T, err error, expectedErr InvalidValueError) {
	switch err.(type) {
	case InvalidValueError:
		invalidValueErr := err.(InvalidValueError)
		invalidValueErr.Expected = ""
		if diff := deep.Equal(&invalidValueErr, &expectedErr); diff != nil {
			t.Error(diff)
		} else {
			t.Log(err.Error())
		}
	default:
		t.Errorf("expected InvalidValueError, got %T: %s", err, err)
	}
}

func compareMissingValueError(t *testing.T, err error, expectedErr MissingValueError) {
	switch err.(type) {
	case MissingValueError:
		missingValueErr := err.(MissingValueError)
		missingValueErr.Explanation = ""
		if diff := deep.Equal(&missingValueErr, &expectedErr); diff != nil {
			t.Error(diff)
		} else {
			t.Logf(err.Error())
		}
	default:
		t.Errorf("expected MissingValueError, got %T: %s", err, err)
	}
}

func compareDuplicateValueError(t *testing.T, err error, expectedErr DuplicateValueError) {
	switch err.(type) {
	case DuplicateValueError:
		duplicateValueErr := err.(DuplicateValueError)
		duplicateValueErr.Explanation = ""
		if diff := deep.Equal(&duplicateValueErr, &expectedErr); diff != nil {
			t.Error(diff)
		} else {
			t.Log(err.Error())
		}
	default:
		t.Errorf("expected DuplicateValueError, got %T: %s", err, err)
	}
}
