// Copyright 2017, Square, Inc.

package runner

import (
	"errors"

	"github.com/square/spincycle/job-runner/kv"
)

var (
	ErrInvalidRunner = errors.New("invalid runner in the repo")
)

type Repo interface {
	Add(string, Runner) error
	Get(string) (Runner, error)
	GetAll() (map[string]Runner, error)
	Remove(string)
}

type rep struct {
	kv.Store
}

func NewRepo() *rep {
	return &rep{
		kv.NewStore(),
	}
}

func (r *rep) Add(jobName string, runner Runner) error {
	return r.Store.Add(jobName, runner)
}

func (r *rep) Get(jobName string) (Runner, error) {
	val, err := r.Store.Get(jobName)
	if err != nil {
		return nil, err
	}

	runner, ok := val.(Runner)
	if !ok {
		return nil, err
	}

	return runner, nil
}

func (r *rep) GetAll() (map[string]Runner, error) {
	vals := r.Store.GetAll()

	allRunners := make(map[string]Runner)
	for jobName, val := range vals {
		runner, ok := val.(Runner) // make sure we got a Runner from the repo
		if ok {
			allRunners[jobName] = runner
		} else {
			return allRunners, ErrInvalidRunner
		}
	}

	return allRunners, nil
}

func (r *rep) Remove(jobName string) {
	r.Store.Delete(jobName)
}
