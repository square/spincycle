// Copyright 2017, Square, Inc.

package chain

import (
	"errors"
	"strconv"

	"github.com/square/spincycle/job-runner/db"

	log "github.com/Sirupsen/logrus"
)

const REPO_DB = "CHAINREPO"

var (
	ErrNotFound = errors.New("chain not found in repo")
	ErrConflict = errors.New("chain already exists in repo")
)

type Repo interface {
	Get(uint) (*chain, error)
	Add(*chain) error
	Set(*chain) error
	Remove(uint) error
}

type repo struct {
	db db.Driver
}

func NewMemoryRepo() Repo {
	return &repo{
		db: db.NewMemory(),
	}
}

// Todo: implement this
//func NewRedisRepo() Repo {}
//
// Questions/TODOs:
//   - How do I pass in config info if this doesn't take any args?
//   - I need to understand interfaces/pointers to interfaces

func (r *repo) Get(id uint) (*chain, error) {
	val, err := r.db.Get(REPO_DB, uintToStr(id))
	if err != nil {
		log.Errorf("Couldn't get chain from the repo (error: %s).", err)
		return nil, ErrNotFound
	}

	chain, ok := val.(*chain)
	if !ok {
		log.Errorf("Couldn't get chain from the repo (error: %s).", err)
		return nil, ErrNotFound
	}

	return chain, nil
}

func (r *repo) Add(chain *chain) error {
	err := r.db.Add(REPO_DB, uintToStr(chain.RequestId()), chain)
	if err != nil {
		log.Errorf("Chain already exists in the repo (error: %s).", err)
		return ErrConflict
	}
	return nil
}

func (r *repo) Set(chain *chain) error {
	return r.db.Set(REPO_DB, uintToStr(chain.RequestId()), chain)
}

func (r *repo) Remove(id uint) error {
	return r.db.Delete(REPO_DB, uintToStr(id))
}

// ------------------------------------------------------------------------- //

func uintToStr(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
