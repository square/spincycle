// Copyright 2017, Square, Inc.

package chain

import (
	"errors"

	"github.com/square/spincycle/job-runner/kv"
	"github.com/square/spincycle/job-runner/runner"
)

var (
	ErrInvalidRunner = errors.New("invalid runner in the repo")
)

type RunnerRepo interface {
	Add(string, runner.Runner) error
	Get(string) (runner.Runner, error)
	GetAll() (map[string]runner.Runner, error)
	Remove(string)
}

type runnerRepo struct {
	kv.Store
}

func NewRunnerRepo() *runnerRepo {
	return &runnerRepo{
		kv.NewStore(),
	}
}

func (r *runnerRepo) Add(jobName string, runner runner.Runner) error {
	return r.Store.Add(jobName, runner)
}

func (r *runnerRepo) Get(jobName string) (runner.Runner, error) {
	val, err := r.Store.Get(jobName)
	if err != nil {
		return nil, err
	}

	runner, ok := val.(runner.Runner)
	if !ok {
		return nil, err
	}

	return runner, nil
}

func (r *runnerRepo) GetAll() (map[string]runner.Runner, error) {
	vals := r.Store.GetAll()

	allRunners := make(map[string]runner.Runner)
	for jobName, val := range vals {
		runner, ok := val.(runner.Runner) // make sure we got a Runner from the repo
		if ok {
			allRunners[jobName] = runner
		} else {
			return allRunners, ErrInvalidRunner
		}
	}

	return allRunners, nil
}

func (r *runnerRepo) Remove(jobName string) {
	r.Store.Delete(jobName)
}
