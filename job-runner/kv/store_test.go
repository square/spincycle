// Copyright 2017, Square, Inc.

package kv

import (
	"testing"
)

func TestAdd(t *testing.T) {
	store := NewStore()
	// Add something.
	err := store.Add("key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Add duplicate key.
	err = store.Add("key1", "newitem")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestSet(t *testing.T) {
	store := NewStore()
	// Set something.
	store.Set("key1", "item")

	// Set duplicate key.
	store.Set("key1", "newitem")

	// Get the key and make sure the value matches the second Set.
	item, err := store.Get("key1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	str, ok := item.(string)
	if !ok {
		t.Errorf("expected to get a string out from the store, but did not")
	}
	if str != "newitem" {
		t.Errorf("str = %s, expected newitem", str)
	}
}

func TestGet(t *testing.T) {
	store := NewStore()
	// Add something.
	err := store.Add("key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Get a key that doesn't exist.
	item, err := store.Get("key2")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}

	// Get something that does exist.
	item, err = store.Get("key1")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	str, ok := item.(string)
	if !ok {
		t.Errorf("expected to get a string out from the store, but did not")
	}
	if str != "item" {
		t.Errorf("str = %s, expected item", str)
	}
}

func TestGetAll(t *testing.T) {
	store := NewStore()
	// Add something.
	err := store.Add("key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	items := store.GetAll()
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
	store := NewStore()
	// Add something.
	err := store.Add("key1", "item")
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}

	// Delete it.
	store.Delete("key1")

	// Make sure we can no longer get it.
	_, err = store.Get("key1")
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}
