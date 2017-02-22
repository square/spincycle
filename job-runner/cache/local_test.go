// Copyright 2017, Square, Inc.

package cache

import (
	"testing"
)

func TestAdd(t *testing.T) {
	cache := NewLocalCache()
	// Add something.
	err := cache.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Add same key/item, but to a different db.
	err = cache.Add("db2", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Add duplicate db/key.
	err = cache.Add("db1", "key1", "newitem")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestGet(t *testing.T) {
	cache := NewLocalCache()
	// Add something.
	err := cache.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get a key that doesn't exist.
	item, err := cache.Get("db1", "key2")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}

	// Get a db that doesn't exist.
	item, err = cache.Get("db2", "key1")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}

	// Get something that does exist.
	item, err = cache.Get("db1", "key1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	str, ok := item.(string)
	if !ok {
		t.Errorf("expected to get a string out from the cache, but did not")
	}
	if str != "item" {
		t.Errorf("str = %s, expected item", str)
	}
}

func TestGetAll(t *testing.T) {
	cache := NewLocalCache()
	// Add something.
	err := cache.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get a db that doesn't exist.
	items := cache.GetAll("db2")
	if len(items) != 0 {
		t.Errorf("expected there to be 0 items, but there are %d", len(items))
	}

	// Get a db that does exist.
	items = cache.GetAll("db1")
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
	cache := NewLocalCache()
	// Add something.
	err := cache.Add("db1", "key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Delete it.
	cache.Delete("db1", "key1")

	// Make sure we can no longer get it.
	_, err = cache.Get("db1", "key1")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}
