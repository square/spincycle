// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/request-manager/id"
)

var (
	ErrIdGenerator = errors.New("forced error in id generator")
)

type IDGenerator struct {
	UIDFunc func() (string, error)
	IDFunc  func() string
}

func (g IDGenerator) UID() (string, error) {
	if g.UIDFunc != nil {
		return g.UIDFunc()
	}
	return "", nil
}

func (g IDGenerator) ID() string {
	if g.IDFunc != nil {
		return g.IDFunc()
	}
	return ""
}

type IDGeneratorFactory struct {
	MakeFunc func() id.Generator
}

func (f IDGeneratorFactory) Make() id.Generator {
	return f.MakeFunc()
}
