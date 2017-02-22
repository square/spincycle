// Copyright 2017, Square, Inc.

package proto

const (
	STATE_UNKNOWN    byte = iota
	STATE_PENDING         // hasn't started yet
	STATE_RUNNING         // is running
	STATE_COMPLETE        // has completed
	STATE_INCOMPLETE      // did not complete and isn't running
	STATE_FAIL            // failed or was stoppoed
	STATE_TIMEOUT         // stopped due to timeout
)

var StateName = map[byte]string{
	STATE_UNKNOWN:    "UNKNOWN",
	STATE_PENDING:    "PENDING",
	STATE_RUNNING:    "RUNNING",
	STATE_COMPLETE:   "COMPLETE",
	STATE_INCOMPLETE: "INCOMPLETE",
	STATE_FAIL:       "FAIL",
	STATE_TIMEOUT:    "TIMEOUT",
}

var StateValue = map[string]byte{
	"UNKNOWN":    STATE_UNKNOWN,
	"PENDING":    STATE_PENDING,
	"RUNNING":    STATE_RUNNING,
	"COMPLETE":   STATE_COMPLETE,
	"INCOMPLETE": STATE_INCOMPLETE,
	"FAIL":       STATE_FAIL,
	"TIMEOUT":    STATE_TIMEOUT,
}
