// Copyright 2017, Square, Inc.

package db

import (
	"testing"
)

func TestAdd(t *testing.T) {
	db := NewMemory()
	// Add something.
	err := db.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Add same key/item, but to a different db.
	err = db.Add("db2", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Add duplicate db/key.
	err = db.Add("db1", "key1", "newitem")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestSet(t *testing.T) {
	db := NewMemory()
	// Set something.
	err := db.Set("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Set duplicate db/key.
	err = db.Set("db1", "key1", "newitem")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get the key and make sure the value matches the secon Set.
	item, err := db.Get("db1", "key1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	str, ok := item.(string)
	if !ok {
		t.Errorf("expected to get a string out from the db, but did not")
	}
	if str != "newitem" {
		t.Errorf("str = %s, expected newitem", str)
	}
}

func TestGet(t *testing.T) {
	db := NewMemory()
	// Add something.
	err := db.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get a key that doesn't exist.
	item, err := db.Get("db1", "key2")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}

	// Get a db that doesn't exist.
	item, err = db.Get("db2", "key1")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}

	// Get something that does exist.
	item, err = db.Get("db1", "key1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	str, ok := item.(string)
	if !ok {
		t.Errorf("expected to get a string out from the db, but did not")
	}
	if str != "item" {
		t.Errorf("str = %s, expected item", str)
	}
}

func TestGetAll(t *testing.T) {
	db := NewMemory()
	// Add something.
	err := db.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get a db that doesn't exist.
	items, err := db.GetAll("db2")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if len(items) != 0 {
		t.Errorf("expected there to be 0 items, but there are %d", len(items))
	}

	// Get a db that does exist.
	items, err = db.GetAll("db1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if len(items) != 1 {
		t.Errorf("expected there to be 1 item, but there are %d", len(items))
	}

	str, ok := items["key1"].(string)
	if !ok {
		t.Errorf("expected the first item to be a string, but it is not")
	}
	if str != "item" {
		t.Errorf("str = %s, expected item", str)
	}
}

func TestDelete(t *testing.T) {
	db := NewMemory()
	// Add something.
	err := db.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Delete it.
	err = db.Delete("db1", "key1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Make sure we can no longer get it.
	_, err = db.Get("db1", "key1")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}
