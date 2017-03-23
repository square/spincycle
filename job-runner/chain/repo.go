// Copyright 2017, Square, Inc.

package chain

import (
	"errors"
)

var (
	ErrNotFound = errors.New("chain not found in repo")
	ErrConflict = errors.New("chain already exists in repo")
)

// Repo stores and provides thread-safe access to job chains.
type Repo interface {
	Get(uint) (*chain, error)
	Add(*chain) error
	Set(*chain) error
	Remove(uint) error
}
