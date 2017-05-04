// Copyright 2017, Square, Inc.

package mock

import (
	"errors"

	"github.com/garyburd/redigo/redis"
)

var (
	ErrRedis = errors.New("forced error in redis")
)

// Conn satisfies the Conn interface in redigo/redis. It is used to mock out a
// redis connection.
type Conn struct {
	CloseFunc   func() error
	ErrFunc     func() error
	DoFunc      func(commandName string, args ...interface{}) (reply interface{}, err error)
	SendFunc    func(commandName string, args ...interface{}) error
	FlushFunc   func() error
	ReceiveFunc func() (reply interface{}, err error)
}

func (c *Conn) Close() error {
	if c.CloseFunc != nil {
		return c.CloseFunc()
	}
	return nil
}

func (c *Conn) Err() error {
	if c.ErrFunc != nil {
		return c.ErrFunc()
	}
	return nil
}

func (c *Conn) Do(commandName string, args ...interface{}) (reply interface{}, err error) {
	if c.DoFunc != nil {
		return c.DoFunc(commandName, args...)
	}
	return nil, nil
}

func (c *Conn) Send(commandName string, args ...interface{}) error {
	if c.SendFunc != nil {
		return c.SendFunc(commandName, args)
	}
	return nil
}

func (c *Conn) Flush() error {
	if c.FlushFunc != nil {
		return c.FlushFunc()
	}
	return nil
}

func (c *Conn) Receive() (reply interface{}, err error) {
	if c.ReceiveFunc != nil {
		return c.ReceiveFunc()
	}
	return nil, nil
}

// Pool implements the same methods as the Pool struct in redigo/redis, and can
// therefore be used to satisfy the same interfaces.
type Pool struct {
	ActiveCountFunc func() int
	CloseFunc       func() error
	GetFunc         func() redis.Conn
}

func (p *Pool) ActiveCount() int {
	if p.ActiveCountFunc != nil {
		return p.ActiveCountFunc()
	}
	return 0
}

func (p *Pool) Close() error {
	if p.CloseFunc != nil {
		return p.CloseFunc()
	}
	return nil
}

func (p *Pool) Get() redis.Conn {
	if p.GetFunc != nil {
		return p.GetFunc()
	}
	return &Conn{}
}
