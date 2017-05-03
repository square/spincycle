// Copyright 2017, Square, Inc.

package chain

import (
	"strconv"

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

func (m *memoryRepo) Get(id uint) (*chain, error) {
	val, err := m.Store.Get(uintToStr(id))
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
	err := m.Store.Add(uintToStr(chain.RequestId()), chain)
	if err != nil {
		return err
	}
	return nil
}

func (m *memoryRepo) Set(chain *chain) error {
	m.Store.Set(uintToStr(chain.RequestId()), chain)
	return nil
}

func (m *memoryRepo) Remove(id uint) error {
	m.Store.Delete(uintToStr(id))
	return nil
}

// ------------------------------------------------------------------------- //

func uintToStr(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
