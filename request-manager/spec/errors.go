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
	Sequence string
	Node     *string
	Field    string
	Values   []string
	Expected string
}

func (e InvalidValueError) Error() string {
	var loc string
	switch e.Node {
	case nil:
		loc = fmt.Sprintf("sequence %s", e.Sequence)
	default:
		loc = fmt.Sprintf("sequence %s, node %s", e.Sequence, *e.Node)
	}

	var values string
	values = fmt.Sprintf("\"%s\"", strings.Join(e.Values, "\", \""))

	return fmt.Sprintf("%s: invalid value(s) %s in field `%s`, expected %s",
		loc, values, e.Field, e.Expected)
}

/* =========================================================================== */

var _ error = MissingValueError{}

type MissingValueError struct {
	Sequence    string
	Node        *string
	Field       string
	Explanation string
}

func (e MissingValueError) Error() string {
	var loc string
	switch e.Node {
	case nil:
		loc = fmt.Sprintf("sequence %s", e.Sequence)
	default:
		loc = fmt.Sprintf("sequence %s, node %s", e.Sequence, *e.Node)
	}

	var explanation string
	switch e.Explanation {
	case "":
		explanation = ""
	default:
		explanation = fmt.Sprintf(": %s", e.Explanation)
	}

	return fmt.Sprintf("%s: field(s) `%s` missing%s", loc, e.Field, explanation)
}

/* =========================================================================== */

var _ error = DuplicateValueError{}

type DuplicateValueError struct {
	Sequence    string
	Node        *string
	Field       string
	Values      []string
	Explanation string
}

func (e DuplicateValueError) Error() string {
	var loc string
	switch e.Node {
	case nil:
		loc = fmt.Sprintf("sequence %s", e.Sequence)
	default:
		loc = fmt.Sprintf("sequence %s, node %s", e.Sequence, *e.Node)
	}

	var values string
	values = fmt.Sprintf("\"%s\"", strings.Join(e.Values, "\", \""))

	var explanation string
	switch e.Explanation {
	case "":
		explanation = ""
	default:
		explanation = fmt.Sprintf(": %s", e.Explanation)
	}

	return fmt.Sprintf("%s: value(s) %s duplicated in field `%s`%s",
		loc, values, e.Field, explanation)
}
