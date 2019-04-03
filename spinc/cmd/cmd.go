// Copyright 2017-2019, Square, Inc.

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
	case "info":
		return NewInfo(ctx), nil
	default:
		return nil, ErrNotExist
	}
}

// SqueezeString makes string s fit into n characters by truncating and replacing
// middle characters with x. Example: "hello, world!" squeezed into 5 characters
// with ".." replacement: "he..!". Preference is given to left-side characters
// when the truncation is not even. If s is squeezed too hard, it returns an empty
// string (n - len(x) < 0 = "").
func SqueezeString(s string, n int, x string) string {
	ls := len(s)
	if ls <= n {
		return s
	}
	r := n - len(x)
	if r < 0 {
		return ""
	}
	if r == 0 {
		return x
	}
	right := r / 2
	left := r - right
	return s[0:left] + x + s[ls-right:ls]
}

func QuoteArgValue(val string) string {
	// If no whitespace or quotes, no change
	if !strings.ContainsAny(val, " \t") && !strings.ContainsAny(val, `"`) {
		return val
	}
	// Return in "" and escape inner ", if any
	return `"` + strings.Replace(val, `"`, `\"`, -1) + `"`
}
