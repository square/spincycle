// Copyright 2020, Square, Inc.

package mock

import (
	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/graph"
)

type ResolverFactory struct {
	MakeFunc func(proto.Request) graph.Resolver
}

func (f *ResolverFactory) Make(req proto.Request) graph.Resolver {
	if f.MakeFunc != nil {
		return f.MakeFunc(req)
	}
	return nil
}

type Resolver struct {
	RequestArgsFunc       func(jobArgs map[string]interface{}) ([]proto.RequestArg, error)
	BuildRequestGraphFunc func(jobArgs map[string]interface{}) (*graph.Graph, error)
}

func (o *Resolver) RequestArgs(jobArgs map[string]interface{}) ([]proto.RequestArg, error) {
	if o.RequestArgsFunc != nil {
		return o.RequestArgsFunc(jobArgs)
	}
	return []proto.RequestArg{}, nil
}
func (o *Resolver) BuildRequestGraph(jobArgs map[string]interface{}) (*graph.Graph, error) {
	if o.BuildRequestGraphFunc != nil {
		return o.BuildRequestGraphFunc(jobArgs)
	}
	return nil, nil
}
