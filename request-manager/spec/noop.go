// Copyright 2020, Square, Inc.

package spec

var noopCategory = "job"
var noopNodeType = "noop"

var NoopNode = Node{
	Name:     "noop",
	Category: &noopCategory,
	NodeType: &noopNodeType,
}
