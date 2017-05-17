// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/square/spincycle/request-manager/grapher"
)

var (
	ErrGrapher = errors.New("forced error in grapher")
)

type Grapher struct {
	CreateGraphFunc func(string, map[string]interface{}) (*grapher.Graph, error)
}

func (g *Grapher) CreateGraph(requestType string, args map[string]interface{}) (*grapher.Graph, error) {
	if g.CreateGraphFunc != nil {
		return g.CreateGraphFunc(requestType, args)
	}
	return &grapher.Graph{}, nil
}

type Payload struct {
	CreateErr     error
	SerializeResp []byte
	SerializeErr  error
	TypeResp      string
	NameResp      string
}

func (p *Payload) Create(map[string]interface{}) error {
	return p.CreateErr
}

func (p *Payload) Serialize() ([]byte, error) {
	return p.SerializeResp, p.SerializeErr
}

func (p *Payload) Type() string {
	return p.TypeResp
}

func (p *Payload) Name() string {
	return p.NameResp
}
