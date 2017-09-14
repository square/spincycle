package db

import (
	"errors"
	"fmt"
)

var (
	ErrNotUpdated      = errors.New("cannot find record to update")
	ErrMultipleUpdated = errors.New("multiple records updated")
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
