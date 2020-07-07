// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/v2/proto"
)

var (
	ErrJCStore = errors.New("forced error in jobchain store")
)

type JCStore struct {
	GetFunc func(string) (proto.JobChain, error)
}

func (j *JCStore) Get(reqId string) (proto.JobChain, error) {
	if j.GetFunc != nil {
		return j.GetFunc(reqId)
	}
	return proto.JobChain{}, nil
}
