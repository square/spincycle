// Copyright 2017, Square, Inc.

package chain

import (
	"errors"
)

// ChainKey is the keyspace for serialized Chains
const CHAIN_KEY = "ChainById"

var (
	ErrNotFound        = errors.New("chain not found in repo")
	ErrConflict        = errors.New("chain already exists in repo")
	ErrMultipleDeleted = errors.New("multiple chains deleted")
)

// Repo stores and provides thread-safe access to job chains.
type Repo interface {
	Get(uint) (*chain, error)
	Add(*chain) error
	Set(*chain) error
	Remove(uint) error
}
