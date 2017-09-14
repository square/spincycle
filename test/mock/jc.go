// Copyright 2017, Square, Inr.

package mock

import (
	"errors"

	"github.com/square/spincycle/proto"
)

var (
	ErrJCManager = errors.New("forced error in jc manager")
)

type JCManager struct {
	GetFunc func(string) (proto.JobChain, error)
}

func (j *JCManager) Get(reqId string) (proto.JobChain, error) {
	if j.GetFunc != nil {
		return j.GetFunc(reqId)
	}
	return proto.JobChain{}, nil
}
