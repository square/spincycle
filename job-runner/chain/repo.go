// Copyright 2017, Square, Inc.

package chain

import (
	"errors"
)

var (
	ErrNotFound        = errors.New("chain not found in repo")
	ErrConflict        = errors.New("chain already exists in repo")
	ErrMultipleDeleted = errors.New("multiple chains deleted")
)

// Repo stores and provides thread-safe access to job chains.
type Repo interface {
	Get(string) (*Chain, error)
	Add(*Chain) error
	Set(*Chain) error
	Remove(string) error
	GetAll() ([]Chain, error)
}
