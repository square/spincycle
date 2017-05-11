// Copyright 2017, Square, Inc.

package chain

import (
	"github.com/square/spincycle/job-runner/kv"
)

type memoryRepo struct {
	kv.Store
}

// NewMemoryRepo returns a repo that is backed by a memory kv store.
func NewMemoryRepo() *memoryRepo {
	return &memoryRepo{
		kv.NewStore(),
	}
}

func (m *memoryRepo) Get(id string) (*chain, error) {
	val, err := m.Store.Get(id)
	if err != nil {
		return nil, err
	}

	chain, ok := val.(*chain)
	if !ok {
		return nil, ErrNotFound
	}

	return chain, nil
}

func (m *memoryRepo) Add(chain *chain) error {
	err := m.Store.Add(chain.RequestId(), chain)
	if err != nil {
		return err
	}
	return nil
}

func (m *memoryRepo) Set(chain *chain) error {
	m.Store.Set(chain.RequestId(), chain)
	return nil
}

func (m *memoryRepo) Remove(id string) error {
	m.Store.Delete(id)
	return nil
}
