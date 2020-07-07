// Copyright 2019, Square, Inc.

package proto_test

import (
	"testing"
	"time"

	"github.com/square/spincycle/v2/proto"
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

func TestRequestFilterString(t *testing.T) {
	f := proto.RequestFilter{}
	expect := ""
	got := f.String()
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}

	f = proto.RequestFilter{
		Type: "request-type",
		States: []byte{
			proto.STATE_PENDING,
			proto.STATE_RUNNING,
			proto.STATE_SUSPENDED,
		},
		User:   "felixp",
		Since:  time.Date(2020, 01, 01, 12, 34, 56, 789123000, time.UTC),
		Until:  time.Date(2020, 01, 02, 12, 34, 56, 789000000, time.UTC),
		Limit:  5,
		Offset: 10,
	}
	expect = "limit=5&offset=10&since=2020-01-01T12%3A34%3A56.789123Z&state=PENDING&state=RUNNING&state=SUSPENDED&type=request-type&until=2020-01-02T12%3A34%3A56.789Z&user=felixp"
	got = f.String()
	if got != expect {
		t.Errorf("got '%s', expected '%s'", got, expect)
	}
}
