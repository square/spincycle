// Copyright 2017, Square, Inc.

// Package mock provides mocks for testing.
package mock

import (
	"errors"
)

var (
	ErrKVStore = errors.New("forced error in datastore")
)

type KVStore struct {
	AddErr     error
	GetResp    interface{}
	GetErr     error
	GetAllResp map[string]interface{}
}

func (d *KVStore) Add(key string, val interface{}) error {
	return d.AddErr
}

func (d *KVStore) Set(key string, val interface{}) {
	return
}

func (d *KVStore) Get(key string) (interface{}, error) {
	return d.GetResp, d.GetErr
}

func (d *KVStore) GetAll() map[string]interface{} {
	return d.GetAllResp
}

func (d *KVStore) Delete(key string) {
	return
}
