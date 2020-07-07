// Copyright 2017, Square, Inr.

package id_test

import (
	"testing"

	"github.com/square/spincycle/v2/request-manager/id"
)

func TestGeneratorLongIds(t *testing.T) {
	g := id.NewGenerator(5, 3)

	// Generate one id.
	id, err := g.UID()
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
	if len(id) != 5 {
		t.Errorf("id length = %d, expected %d", len(id), 5)
	}

	// There shouldn't be an error generating the next 100 ids.
	for i := 0; i < 100; i++ {
		_, err = g.UID()
		if err != nil {
			t.Errorf("err = %s, expected nil", err)
			break
		}
	}
}

func TestGeneratorShortIds(t *testing.T) {
	g := id.NewGenerator(1, len(id.CHARS)*len(id.CHARS))
	allIds := map[string]struct{}{}

	// Given that the generator is only creating ids with 1 character, there
	// should definitely be an error in generating more ids than there are
	// characters available to use ids.
	var err error
	for i := 0; i < len(id.CHARS)+1; i++ {
		var id string
		id, err = g.UID()
		if err != nil {
			break
		}

		// Check to make sure the id is unique.
		if _, ok := allIds[id]; ok {
			t.Errorf("id %s is not unique", id)
		}
		allIds[id] = struct{}{}
	}

	if len(allIds) != len(id.CHARS) {
		t.Errorf("generated %d ids, expected %d", len(allIds), len(id.CHARS))
	}

	if err != id.ErrGenerateUnique {
		t.Errorf("error = %s, expected %s", err, id.ErrGenerateUnique)
	}
}
