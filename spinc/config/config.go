// Copyright 2017, Square, Inc.

// Package config handles config files, -config, and env vars at startup.
package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager"
	"gopkg.in/yaml.v2"
)

const (
	DEFAULT_CONFIG_FILES = "/etc/spinc/spinc.yaml,~/.spinc.yaml"
	DEFAULT_TIMEOUT      = 10000 // 10s
)

var (
	ErrUnknownRequest = errors.New("request does not exist")
)

// Options represents typical command line options: --addr, --config, etc.
type Options struct {
	Addr    string `arg:"env" yaml:"addr"`
	Config  string `arg:"env"`
	Debug   bool
	Env     string
	Help    bool
	Ping    bool
	Timeout uint `arg:"env" yaml:"timeout"`
	Version bool
	Verbose bool `arg:"-v"`
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

// Help prints full help or minimal request help if full is false. An rm.Client
// is used to get the list of available requests from the Request Manager API.
func Help(full bool, rmc rm.Client) {
	if full {
		fmt.Printf("Usage: spinc [flags] <command> [request|id] [args|file]\n\n"+
			"Flags:\n"+
			"  --addr     Address of Request Managers (https://local.domain:8080)\n"+
			"  --config   Config files (default: %s)\n"+
			"  --debug    Print debug to stderr\n"+
			"  --env      Environment (dev, staging, production)\n"+
			"  --help     Print help\n"+
			"  --ping     Ping addr\n"+
			"  --timeout  API timeout, milliseconds (default: %d)\n"+
			"  --version  Print version\n"+
			"  -v         Verbose ps or status\n\n"+
			"Commands:\n"+
			"  help    <request>  Print request help\n"+
			"  start   <request>  Start new request\n"+
			"  stop    <ID>       Stop request\n"+
			"  status  <ID>       Print status of request\n"+
			"  running <ID>       Exit 0 if request is running, else exit 1\n"+
			"  ps                 Show all running requests and jobs\n\n",
			DEFAULT_CONFIG_FILES, DEFAULT_TIMEOUT)
		printRequestList(rmc)
		fmt.Println("\n'spinc help <request>' for request help")
	} else {
		fmt.Printf("spinc help  <request>\nspinc start <request>\n\n")
		printRequestList(rmc)
		fmt.Println("\n'spinc help' for more")
	}
}

func printRequestList(rmc rm.Client) {
	if rmc == nil {
		fmt.Println("Specify --addr or ADDR environment variable to list all requests")
		return
	}

	fmt.Println("Requests:")
	req, err := rmc.RequestList()
	if err != nil {
		fmt.Println(err)
	} else {
		for _, r := range req {
			fmt.Println("  " + r.Name)
		}
	}
}

func RequestHelp(reqName string, rmc rm.Client) error {
	reqList, err := rmc.RequestList()
	if err != nil {
		return err
	}

	var req *proto.RequestSpec
	for _, r := range reqList {
		if r.Name != reqName {
			continue
		}
		req = &r
		break
	}

	if req == nil {
		return ErrUnknownRequest
	}

	fmt.Printf("%s args (* required)\n\n", req.Name)
	l := 0
	for _, a := range req.Args {
		if len(a.Name) > l {
			l = len(a.Name)
		}
	}
	line := fmt.Sprintf("  %%s %%-%ds  %%s\n", l)
	for _, a := range req.Args {
		help := a.Desc
		star := " "
		if a.Required {
			star = "*"
		} else {
			help += " (default: " + a.Default + ")"
		}
		fmt.Printf(line, star, a.Name, help)
	}
	fmt.Println("\nspinc start shutdown-host # prompt for args\n" +
		"spinc start shutdown-host <args>\n" +
		"spinc start shutdown-host <args-file.yaml>")
	return nil
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
