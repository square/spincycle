// Copyright 2017-2019, Square, Inc.

// Package app provides app-wide data structs and functions.
package app

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/square/spincycle/request-manager"
	"github.com/square/spincycle/spinc/config"
)

var (
	ErrHelp           = errors.New("print help")
	ErrUnknownRequest = errors.New("request does not exist")
)

// Context represents how to run spinc. A context is passed to spinc.Run().
// A default context is created in main.go. Wrapper code can integrate with
// spinc by passing a custom context to spinc.Run(). Integration is done
// primarily with hooks and factories.
type Context struct {
	// Set in main.go or by wrapper
	In        io.Reader // where to read user input (default: stdin)
	Out       io.Writer // where to print output (default: stdout)
	Hooks     Hooks     // for integration with other code
	Factories Factories // for integration with other code

	// Set automatically in spinc.Run()
	Options  config.Options // command line options (--addr, etc.)
	Command  config.Command // command and args, if any ("start <request>", etc.)
	RMClient rm.Client      // Request Manager client
	Nargs    int            // number of positional args including command
}

type Command interface {
	Prepare() error
	Run() error
	Cmd() string
	Help() string
}

type CommandFactory interface {
	Make(string, Context) (Command, error)
}

type HTTPClientFactory interface {
	Make(Context) (*http.Client, error)
}

type Factories struct {
	HTTPClient HTTPClientFactory
	Command    CommandFactory
}

type Hooks struct {
	AfterParseOptions func(*config.Options)
	CommandRunResult  func(interface{}, error)
}

func init() {
	log.SetPrefix("DEBUG ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func Debug(fmt string, v ...interface{}) {
	log.Printf(fmt, v...)
}
