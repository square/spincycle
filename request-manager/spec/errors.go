// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"strings"
)

func stringSetToArray(set map[string]bool) []string {
	arr := []string{}
	for v, _ := range set {
		arr = append(arr, v)
	}
	return arr
}

/* =========================================================================== */

var _ error = InvalidValueError{}

type InvalidValueError struct {
	Node     *string
	Field    string
	Values   []string
	Expected string
}

func (e InvalidValueError) Error() string {
	var loc string
	if e.Node != nil {
		loc = fmt.Sprintf("node %s: ", *e.Node)
	}

	var values string
	values = fmt.Sprintf("\"%s\"", strings.Join(e.Values, "\", \""))
	var multipleValues string
	if len(e.Values) > 1 {
		multipleValues = "s"
	}

	return fmt.Sprintf("%sinvalid value%s %s in field '%s', expected %s",
		loc, multipleValues, values, e.Field, e.Expected)
}

/* =========================================================================== */

var _ error = MissingValueError{}

type MissingValueError struct {
	Node        *string
	Field       string
	Explanation string
}

func (e MissingValueError) Error() string {
	var loc string
	if e.Node != nil {
		loc = fmt.Sprintf("node %s: ", *e.Node)
	}

	var explanation string
	switch e.Explanation {
	case "":
		explanation = ""
	default:
		explanation = fmt.Sprintf(": %s", e.Explanation)
	}

	return fmt.Sprintf("%sfield '%s' missing%s", loc, e.Field, explanation)
}

/* =========================================================================== */

var _ error = DuplicateValueError{}

type DuplicateValueError struct {
	Node        *string
	Field       string
	Values      []string
	Explanation string
}

func (e DuplicateValueError) Error() string {
	var loc string
	if e.Node != nil {
		loc = fmt.Sprintf("node %s: ", *e.Node)
	}

	var values string
	values = fmt.Sprintf("\"%s\"", strings.Join(e.Values, "\", \""))
	var multipleValues string
	if len(e.Values) > 1 {
		multipleValues = "s"
	}

	var explanation string
	switch e.Explanation {
	case "":
		explanation = ""
	default:
		explanation = fmt.Sprintf(": %s", e.Explanation)
	}

	return fmt.Sprintf("%svalue%s %s duplicated in field '%s'%s",
		loc, multipleValues, values, e.Field, explanation)
}
