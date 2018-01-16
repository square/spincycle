// Copyright 2017, Square, Inc.

// Package db provides database helper functions.
package db

// ["a","b"] -> "'a','b'"
func IN(vals []string) string {
	in := ""
	n := len(vals) - 1
	for i, val := range vals {
		in += "'" + val + "'"
		if i < n {
			in += ","
		}
	}
	return in
}
