// Copyright 2017, Square, Inc.

// Package mock provides mocks for testing.
package mock

import (
	"errors"
)

var (
	ErrDatastore = errors.New("forced error in datastore")
)

type Datastore struct {
	AddErr     error
	SetErr     error
	GetResp    interface{}
	GetErr     error
	GetAllResp map[string]interface{}
	GetAllErr  error
	DeleteErr  error
}

func (d *Datastore) Add(db, key string, val interface{}) error {
	return d.AddErr
}

func (d *Datastore) Set(db, key string, val interface{}) error {
	return d.SetErr
}

func (d *Datastore) Get(db, key string) (interface{}, error) {
	return d.GetResp, d.GetErr
}

func (d *Datastore) GetAll(db string) (map[string]interface{}, error) {
	return d.GetAllResp, d.GetAllErr
}

func (d *Datastore) Delete(db, key string) error {
	return d.DeleteErr
}
