// Copyright 2017, Square, Inc.

// Package mock provides mocks for testing.
package mock

import (
	"errors"
)

var (
	ErrCache = errors.New("forced error in cache")
)

type Cache struct {
	AddErr     error
	GetResp    interface{}
	GetErr     error
	GetAllResp map[string]interface{}
}

func (c *Cache) Add(db, key string, val interface{}) error {
	return c.AddErr
}

func (c *Cache) Get(db, key string) (interface{}, error) {
	return c.GetResp, c.GetErr
}

func (c *Cache) GetAll(db string) map[string]interface{} {
	return c.GetAllResp
}

func (c *Cache) Delete(db, key string) {
	return
}
