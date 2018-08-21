// Copyright 2017-2018, Square, Inc.

package proto

// The normal state transition for chains and jobs is:
//   1: pending
//   2: running
//   3: complete
// State 3 (complete) is the one and only indicator of success. When a chain or
// job finishes, its final state will be 3 (complete) or a specific error state.

const (
	STATE_UNKNOWN byte = iota

	// Normal states, in order
	STATE_PENDING  // not started
	STATE_RUNNING  // running
	STATE_COMPLETE // completed successfully

	// Error states, no order
	STATE_FAIL    // failed due to error or non-zero exit
	STATE_TIMEOUT // timeout
	STATE_STOPPED // stopped by user

	STATE_SUSPENDED // suspended; doesn't apply to jobs
)

var StateName = map[byte]string{
	STATE_UNKNOWN:   "UNKNOWN",
	STATE_PENDING:   "PENDING",
	STATE_RUNNING:   "RUNNING",
	STATE_COMPLETE:  "COMPLETE",
	STATE_FAIL:      "FAIL",
	STATE_TIMEOUT:   "TIMEOUT",
	STATE_STOPPED:   "STOPPED",
	STATE_SUSPENDED: "SUSPENDED",
}

var StateValue = map[string]byte{
	"UNKNOWN":   STATE_UNKNOWN,
	"PENDING":   STATE_PENDING,
	"RUNNING":   STATE_RUNNING,
	"COMPLETE":  STATE_COMPLETE,
	"FAIL":      STATE_FAIL,
	"TIMEOUT":   STATE_TIMEOUT,
	"STOPPED":   STATE_STOPPED,
	"SUSPENDED": STATE_SUSPENDED,
}
