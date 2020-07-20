// Copyright 2017-2020, Square, Inc.

package spec

import (
	"fmt"
)

/* =========================================================================== */

var _ error = InvalidValueError{}

type InvalidValueError struct {
	Sequence string
	Node     *string
	Field    string
	Value    string
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

	return fmt.Sprintf("%s, field `%s` contains invalid value \"%s\", expected %s",
		loc, e.Field, e.Value, e.Expected)
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

	return fmt.Sprintf("%s, field `%s` missing%s", loc, e.Field, explanation)
}

/* =========================================================================== */

var _ error = DuplicateValueError{}

type DuplicateValueError struct {
	Sequence string
	Node     *string
	Field    string
	Value    string
}

func (e DuplicateValueError) Error() string {
	var loc string
	switch e.Node {
	case nil:
		loc = fmt.Sprintf("sequence %s", e.Sequence)
	default:
		loc = fmt.Sprintf("sequence %s, node %s", e.Sequence, *e.Node)
	}

	return fmt.Sprintf("%s, field `%s` contains duplicate value \"%s\"",
		loc, e.Field, e.Value)
}
