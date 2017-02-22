// Copyright 2017, Square, Inc.

// Package db implements interactions with key/value datastores.
package db

import (
	"errors"
)

var (
	ErrDbNotFound  = errors.New("database not found")
	ErrKeyNotFound = errors.New("key not found")
	ErrConflict    = errors.New("key already exists in database")
)

// A Driver enables interactions with a datastore. It functions under the
// assumption that items in a datastore are stored with the following keyspace -
// database::key::item. Databases and keys must be strings, but the items
// themselves can be any interface. Clients can use databases to logically
// separate the items that they store in the cache.
type Driver interface {
	// Add adds an interface to the datastore. It takes two string arguments,
	// the first of which is the database to add the interface to, and the
	// second of which is the key to add the interface to. If the key
	// already exists, it will return an error.
	Add(string, string, interface{}) error

	// Set adds or updates an interface in the datastore. It takes two string
	// arguments, the first of which is the database to add the interface to,
	// and the second of which is the key to add the interface to.
	Set(string, string, interface{}) error

	// Get looks up an interface from the datastore, using the supplied database
	// and key. It returns an error if the database+key does not exist.
	Get(string, string) (interface{}, error)

	// GetAll returns a map of all keys and interfaces in a database. It
	// returns an empty map if the database does not exist.
	GetAll(string) (map[string]interface{}, error)

	// Delete removes an interface from the datastore, using the supplied
	// database and key. It does nothing if the database+key does not exist.
	Delete(string, string) error
}
