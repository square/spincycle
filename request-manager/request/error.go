package request

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidParams = errors.New("invalid request type and/or arguments")
)

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
