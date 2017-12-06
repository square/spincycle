// Copyright 2017, Square, Inc.

// Package id provides an interface for generating ids.
package id

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

func init() { rand.Seed(time.Now().UTC().UnixNano()) }

// The characters used in ids. This should not be modified.
var CHARS = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

var (
	ErrGenerateUnique = errors.New("unable to generate a UID")
)

// A GeneratorFactory makes Generators.
type GeneratorFactory interface {
	// Make makes a Generator.
	Make() Generator
}

// generatorFactory implements the GeneratorFactory interface.
type generatorFactory struct {
	idLen int // number of characters in an id
	tries int // number of times to attempt generating an id before erroring
}

// NewGeneratorFactory creates a GeneratorFactory. The first argument it takes is
// the length of ids (number of characters) that are created by a Generator. The
// second argument is the number of times a Generator tries to create an id
// before returning an error.
func NewGeneratorFactory(idLen, tries int) GeneratorFactory {
	return &generatorFactory{
		idLen: idLen,
		tries: tries,
	}
}

func (f *generatorFactory) Make() Generator {
	return NewGenerator(f.idLen, f.tries)
}

// generator implements the Generator interface.
type generator struct {
	idLen   int // number of characters in an id
	tries   int // number of times to attempt generating an id before erroring
	usedIds map[string]struct{}
	*sync.Mutex
}

// A Generator generates ids. It is safe for use in concurrent threads.
type Generator interface {
	// ID generates an alphanumeric id.
	ID() string

	// UID generates an alphanumeric unique id (UID). The id is guaranteed to be
	// unique to the Generator. It returns ErrGenerateUnique if it can't generate
	// an id that is unique.
	UID() (string, error)
}

// NewGenerator creates a Generator. The first argument it takes is the length of
// ids (number of characters) that the Generator creates. The second argument is
// the number of times the Generator tries to create an id before returning an error.
func NewGenerator(idLen, tries int) Generator {
	return &generator{
		idLen:   idLen,
		tries:   tries,
		usedIds: map[string]struct{}{},
		Mutex:   &sync.Mutex{},
	}
}

func (g *generator) ID() string {
	return randSeq(g.idLen)
}

func (g *generator) UID() (string, error) {
	for i := 0; i < g.tries; i++ {
		id := randSeq(g.idLen)
		g.Lock()
		if _, ok := g.usedIds[id]; !ok {
			g.usedIds[id] = struct{}{}
			g.Unlock()
			return id, nil
		}
		g.Unlock()
	}
	return "", ErrGenerateUnique
}

// ------------------------------------------------------------------------- //

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = CHARS[rand.Intn(len(CHARS))]
	}
	return string(b)
}
