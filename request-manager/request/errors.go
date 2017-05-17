package request

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidParams   = errors.New("invalid request type and/or arguments")
	ErrNotUpdated      = errors.New("cannot find request to update")
	ErrMultipleUpdated = errors.New("multiple requests updated")
)

type ErrNotFound struct {
	resource string
}

func NewErrNotFound(resource string) ErrNotFound {
	return ErrNotFound{resource}
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.resource)
}

type ErrInvalidState struct {
	expectedState string
	actualState   string
}

func (e ErrInvalidState) Error() string {
	return fmt.Sprintf("request not in %s state (state = %s)", e.expectedState, e.actualState)
}

func NewErrInvalidState(expectedState, actualState string) ErrInvalidState {
	return ErrInvalidState{
		expectedState: expectedState,
		actualState:   actualState,
	}
}
