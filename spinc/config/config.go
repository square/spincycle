// Copyright 2016-2019, Square, Inc.

// Package config handles config files, -config, and env vars at startup.
package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
	"gopkg.in/yaml.v2"
)

const (
	DEFAULT_CONFIG_FILES = "/etc/spinc/spinc.yaml,~/.spinc.yaml"
	DEFAULT_ADDR         = "http://127.0.0.1:32308"
	DEFAULT_TIMEOUT      = 5000 // 5s
)

// An Options record for pulling the originally set user arguments
type UserOptions struct {
	Addr    *string
	Config  *string
	Debug   *bool
	Env     *string
	Help    *bool
	Timeout *uint
	Version *bool
}

type UserCommandLine struct {
	UserOptions
	Command
}

// Options represents typical command line options: --addr, --config, etc.
type Options struct {
	Addr    string `arg:"env:SPINC_ADDR" yaml:"addr"`
	Config  string `arg:"env:SPINC_CONFIG"`
	Debug   bool   `arg:"env:SPINC_DEBUG" yaml:"debug"`
	Env     string `arg:"env:SPINC_ENV" yaml:"env"`
	Help    bool
	Timeout uint `arg:"env:SPINC_TIMEOUT" yaml:"timeout"`
	Version bool
}

// Command represents a command (start, stop, etc.) and its values.
type Command struct {
	Cmd  string   `arg:"positional"`
	Args []string `arg:"positional"`
}

// CommandLine represents options (--addr, etc.) and commands (start, etc.).
// The caller is expected to copy and use the embedded structs separately, like:
//
//   var o config.Options = cmdLine.Options
//   var c config.Command = cmdLine.Command
//
// Some commands and options are mutually exclusive, like --ping and --version.
// Others can be used together, like --addr and --timeout with any command.
type CommandLine struct {
	Options
	Command
}

// Parses the Options explictly set on the command line by the user.
// Used for allowing reporting on command line needed to re-run a command
func ParseUserOptions(def UserOptions) UserOptions {
	var c UserCommandLine
	c.UserOptions = def
	p, err := arg.NewParser(arg.Config{Program: "spinc"}, &c)
	if err != nil {
		fmt.Printf("arg.NewParser: %s", err)
		os.Exit(1)
	}
	if err := p.Parse(os.Args[1:]); err != nil {
		switch err {
		case arg.ErrHelp:
			*c.Help = true
		case arg.ErrVersion:
			*c.Version = true
		default:
			fmt.Printf("Error parsing command line: %s\n", err)
			os.Exit(1)
		}
	}
	return c.UserOptions
}

func (u *UserOptions) ToOptions() Options {
	var o Options

	if u.Addr != nil {
		o.Addr = *u.Addr
	}

	if u.Config != nil {
		o.Config = *u.Config
	}

	if u.Debug != nil {
		o.Debug = *u.Debug
	}

	if u.Env != nil {
		o.Env = *u.Env
	}

	if u.Help != nil {
		o.Help = *u.Help
	}

	if u.Timeout != nil {
		o.Timeout = *u.Timeout
	}

	if u.Version != nil {
		o.Version = *u.Version
	}

	return o
}

// ParseCommandLine parses the command line and env vars. Command line options
// override env vars. Default options are used unless overridden by env vars or
// command line options. Defaults are usually parsed from config files.
func ParseCommandLine(def Options) CommandLine {
	var c CommandLine
	c.Options = def
	p, err := arg.NewParser(arg.Config{Program: "spinc"}, &c)
	if err != nil {
		fmt.Printf("arg.NewParser: %s", err)
		os.Exit(1)
	}
	if err := p.Parse(os.Args[1:]); err != nil {
		switch err {
		case arg.ErrHelp:
			c.Help = true
		case arg.ErrVersion:
			c.Version = true
		default:
			fmt.Printf("Error parsing command line: %s\n", err)
			os.Exit(1)
		}
	}
	return c
}

func ParseConfigFiles(files string, debug bool) Options {
	var def Options
	for _, file := range strings.Split(files, ",") {
		// If file starts with ~/, we need to expand this to the user home dir
		// because this is a shell expansion, not something Go knows about.
		if file[:2] == "~/" {
			usr, _ := user.Current()
			file = filepath.Join(usr.HomeDir, file[2:])
		}

		absfile, err := filepath.Abs(file)
		if err != nil {
			if debug {
				log.Printf("filepath.Abs(%s) error: %s", file, err)
			}
			continue
		}

		bytes, err := ioutil.ReadFile(absfile)
		if err != nil {
			if debug {
				log.Printf("Cannot read config file %s: %s", file, err)
			}
			continue
		}

		var o Options
		if err := yaml.Unmarshal(bytes, &o); err != nil {
			if debug {
				log.Printf("Invalid YAML in config file %s: %s", file, err)
			}
			continue
		}

		// Set options from this config file only if they're set
		if debug {
			log.Printf("Applying config file %s (%s)", file, absfile)
		}
		if o.Addr != "" {
			def.Addr = o.Addr
		}
		if o.Timeout != 0 {
			def.Timeout = o.Timeout
		}
	}
	return def
}
