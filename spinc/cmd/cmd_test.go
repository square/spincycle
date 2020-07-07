// Copyright 2019, Square, Inc.

package cmd_test

import (
	"testing"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/spinc/cmd"
)

// args is used in ps_test and status_test (and maybe others)
var args []proto.RequestArg = []proto.RequestArg{
	{
		Name:  "key",
		Type:  proto.ARG_TYPE_REQUIRED,
		Value: "value",
	},
	{
		Name:  "key2",
		Type:  proto.ARG_TYPE_REQUIRED,
		Value: "val2",
	},
	{
		Name:  "opt",
		Type:  proto.ARG_TYPE_OPTIONAL,
		Value: "not-shown",
	},
}

func TestSqueezeString(t *testing.T) {
	// The documented (code comment) example
	got := cmd.SqueezeString("hello, world!", 5, "..")
	expect := "he..!"
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}

	// Squeezed len 2 = len("..") so we get only ".."
	got = cmd.SqueezeString("hello, world!", 2, "..")
	expect = ".."
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}

	// Squeeze too hard and the string disappears
	got = cmd.SqueezeString("hello, world!", 1, "..")
	expect = ""
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}
	got = cmd.SqueezeString("hello, world!", 0, "..")
	expect = ""
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}

	// A more realistic example: long job name
	got = cmd.SqueezeString("this-is-a-very-long-job-name", 20, "..")
	expect = "this-is-a..-job-name"
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}
}

func TestQuoteArgValue(t *testing.T) {
	inputs := [][]string{
		{"foo", "foo"},           // no spaces or quotes = no change
		{"foo bar", `"foo bar"`}, // space
		{"foo	bar", `"foo	bar"`}, // tab
		{`the "inner" quote`, `"the \"inner\" quote"`}, // escape inner quotes
	}
	for _, v := range inputs {
		got := cmd.QuoteArgValue(v[0])
		if got != v[1] {
			t.Errorf("got '%s', expected '%s'", got, v[1])
		}
	}
}
