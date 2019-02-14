// Copyright 2019, Square, Inc.

// Package errors provides errors reported to the user. These are mapped to a
// proto.Error by the API and sent to the user. All errors must implement the
// error interface and return a helpful error message. The message can be terse
// because it will be reported in context. For example, the RequestNotFound
// error message makes sense in response to "spinc status abc123" when "abc123"
// does not exist.
package errors

import (
	"fmt"
)

var _ error = RequestNotFound{}

type RequestNotFound struct {
	RequestId string
}

func (e RequestNotFound) Error() string {
	return fmt.Sprintf("request %s not found", e.RequestId)
}

// --------------------------------------------------------------------------

var _ error = JobNotFound{}

type JobNotFound struct {
	RequestId string
	JobId     string
}

func (e JobNotFound) Error() string {
	return fmt.Sprintf("job %s not found in request %s", e.JobId, e.RequestId)
}

// --------------------------------------------------------------------------

var _ error = DbError{}

// Error represents a generic database error. This struct is not superfluous,
// it allows the API to distinguish the error type and return an appropriate
// proto.Error.
type DbError struct {
	err   error
	query string
}

func NewDbError(err error, query string) DbError {
	return DbError{err: err, query: query}
}

func (e DbError) Error() string {
	return fmt.Sprintf("database error: %s (%s)", e.err, e.query)
}

// --------------------------------------------------------------------------

var _ error = ErrInvalidCreateRequest{}

type ErrInvalidCreateRequest struct {
	Message string
}

func (e ErrInvalidCreateRequest) Error() string {
	return e.Message
}

// --------------------------------------------------------------------------

var _ error = ErrInvalidState{}

type ErrInvalidState struct {
	expectedState string
	actualState   string
}

func NewErrInvalidState(expectedState, actualState string) ErrInvalidState {
	return ErrInvalidState{
		expectedState: expectedState,
		actualState:   actualState,
	}
}

func (e ErrInvalidState) Error() string {
	return fmt.Sprintf("request in state %s, expected state %s", e.actualState, e.expectedState)
}
