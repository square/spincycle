// Copyright 2019, Square, Inc.

package proto_test

import (
	"testing"

	"github.com/square/spincycle/proto"
)

func TestStatusFilterString(t *testing.T) {
	f := proto.StatusFilter{}
	expect := ""
	got := f.String()
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}

	f = proto.StatusFilter{RequestId: "abc"}
	expect = "?requestId=abc"
	got = f.String()
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}

	f = proto.StatusFilter{RequestId: "abc", OrderBy: "startTime"}
	expect = "?requestId=abc&orderBy=starttime"
	got = f.String()
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}
}
