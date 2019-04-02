// Copyright 2017-2019, Square, Inc.

package chain

import (
	"errors"

	"github.com/orcaman/concurrent-map"
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
	GetAll() ([]*Chain, error)
}

type memoryRepo struct {
	cmap.ConcurrentMap
}

// NewMemoryRepo returns a repo that is backed by a thread-safe map in memory.
func NewMemoryRepo() *memoryRepo {
	return &memoryRepo{
		cmap.New(),
	}
}

func (m *memoryRepo) Get(id string) (*Chain, error) {
	val, exists := m.ConcurrentMap.Get(id)
	if !exists {
		return nil, ErrNotFound
	}

	chain, ok := val.(*Chain)
	if !ok {
		return nil, ErrNotFound
	}

	return chain, nil
}

func (m *memoryRepo) GetAll() ([]*Chain, error) {
	chains := []*Chain{}
	for _, v := range m.ConcurrentMap.Items() {
		chain := v.(*Chain)
		chains = append(chains, chain)
	}
	return chains, nil
}

func (m *memoryRepo) Add(chain *Chain) error {
	wasAbsent := m.ConcurrentMap.SetIfAbsent(chain.RequestId(), chain)
	if !wasAbsent {
		return ErrConflict
	}
	return nil
}

func (m *memoryRepo) Set(chain *Chain) error {
	// Memory repo doesn't have to do anything on Set because we have a *Chain
	// so any changes to that pointer are automatically "saved".
	return nil
}

func (m *memoryRepo) Remove(id string) error {
	m.ConcurrentMap.Remove(id)
	return nil
}
