// Copyright 2017, Square, Inc.

// Package cmd provides all the commands that spinc can run: start, status, etc.
package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/square/spincycle/spinc/app"
)

var (
	ErrNotExist = errors.New("command does not exist")
)

type ErrUnknownArgs struct {
	Request string
	Args    []string
}

func (e ErrUnknownArgs) Error() string {
	return fmt.Sprintf("Unknown request args: %s. Run 'spinc help %s' to list valid args.", strings.Join(e.Args, ", "), e.Request)
}

type DefaultFactory struct {
}

func (f *DefaultFactory) Make(name string, ctx app.Context) (app.Command, error) {
	switch name {
	case "log":
		return NewLog(ctx), nil
	case "ps":
		return NewPs(ctx), nil
	case "running":
		return NewRunning(ctx), nil
	case "start":
		return NewStart(ctx), nil
	case "status":
		return NewStatus(ctx), nil
	case "stop":
		return NewStop(ctx), nil
	case "help":
		return NewHelp(ctx), nil
	case "version":
		return NewVersion(ctx), nil
	default:
		return nil, ErrNotExist
	}
}
