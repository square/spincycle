// Copyright 2017-2018, Square, Inc.

package grapher

import (
	"fmt"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

// NodeSpec defines the structure expected from the yaml file to define each nodes.
type NodeSpec struct {
	Name         string            `yaml:"name"`      // unique name assigned to this node
	Category     string            `yaml:"category"`  // "job", "sequence", or "conditional"
	NodeType     string            `yaml:"type"`      // the type of job or sequence to create
	Each         []string          `yaml:"each"`      // arguments to repeat over
	Args         []*NodeArg        `yaml:"args"`      // expected arguments
	Parallel     *uint             `yaml:"parallel"`  // max number of sequences to run in parallel
	Sets         []NodeSet         `yaml:"sets"`      // expected job args to be set
	Dependencies []string          `yaml:"deps"`      // nodes with out-edges leading to this node
	Retry        uint              `yaml:"retry"`     // the number of times to retry a "job" that fails
	RetryWait    string            `yaml:"retryWait"` // the time to sleep between "job" retries
	If           string            `yaml:"if"`        // the name of the jobArg to check for a conditional value
	Eq           map[string]string `yaml:"eq"`        // conditional values mapping to appropriate sequence names
}

// NodeArg defines the structure expected from the yaml file to define a job's args.
type NodeArg struct {
	Expected string `yaml:"expected"` // the name of the argument that this job expects
	Given    string `yaml:"given"`    // the name of the argument that will be given to this job
}

// NodeSet defines the structure expected from the yaml file to define the args a job sets.
type NodeSet struct {
	Arg string  `yaml:"arg"` // the name of the argument this job outputs by default
	As  *string `yaml:"as"`  // the name of the argument this job should output
}

// SequenceSpec defines the structure expected from the config yaml file to
// define each sequence
// If a field is in the yaml, it appears here, but the reverse is not true; some
// fields here are only for information-passing purposes, and not read in from
// the yaml
type SequenceSpec struct {
	/* Read in from yaml. */
	Name    string               `yaml:"name"`    // name of the sequence
	Args    SequenceArgs         `yaml:"args"`    // arguments to the sequence
	Nodes   map[string]*NodeSpec `yaml:"nodes"`   // list of nodes that are a part of the sequence
	Request bool                 `yaml:"request"` // whether or not the sequence spec is a user request
	ACL     []ACL                `yaml:"acl"`     // allowed caller roles (optional)
	/* Information-passing fields. */
	Retry     uint   `yaml:"-"` // the number of times to retry the sequence if it fails
	RetryWait string `yaml:"-"` // the time to sleep between sequence retries
}

// SequenceArgs defines the structure expected from the config file to define
// a sequence's arguments. A sequence can have required arguments; any arguments
// on this list that are missing will result in an error from Grapher.
// A sequence can also have optional arguemnts; arguments on this list that are
// missing will not result in an error. Additionally optional arguments can
// have default values that will be used if not explicitly given.
type SequenceArgs struct {
	Required []*ArgSpec `yaml:"required"`
	Optional []*ArgSpec `yaml:"optional"`
	Static   []*ArgSpec `yaml:"static"`
}

// ArgSpec defines the structure expected from the config to define sequence args.
type ArgSpec struct {
	Name    string  `yaml:"name"`
	Desc    string  `yaml:"desc"`
	Default *string `yaml:"default"`
}

// ACL represents one role-based ACL entry. Every auth.Caller (from the
// user-provided auth plugin Authenticate method) is authorized with a matching
// ACL, else the request is denied with HTTP 401 unauthorized. Roles are
// user-defined. If Admin is true, Ops cannot be set.
type ACL struct {
	Role  string   `yaml:"role"`  // user-defined role
	Admin bool     `yaml:"admin"` // all ops allowed if true
	Ops   []string `yaml:"ops"`   // proto.REQUEST_OP_*
}

// All Sequences in the yaml. Also contains the user defined no-op job.
type Config struct {
	Sequences map[string]*SequenceSpec `yaml:"sequences"`
}

// ReadConfig will read from configFile and return a Config that the user
// can then use for NewGrapher(). configFile is expected to be in the yaml
// format specified.
func ReadConfig(configFile string) (Config, error) {
	var cfg Config
	sequenceData, err := ioutil.ReadFile(configFile)
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(sequenceData, &cfg)
	if err != nil {
		return cfg, err
	}

	for sequenceName, sequence := range cfg.Sequences {
		sequence.Name = sequenceName
		for _, arg := range sequence.Args.Optional {
			if arg.Default == nil {
				return cfg, fmt.Errorf("optional arg %s in sequence %s was not given a default", arg.Name, sequence.Name)
			}
		}
		for _, arg := range sequence.Args.Static {
			if arg.Default == nil {
				return cfg, fmt.Errorf("static arg %s in sequence %s was not given a default", arg.Name, sequence.Name)
			}
		}

		for nodeName, node := range sequence.Nodes {
			node.Name = nodeName
			if node.Parallel != nil && *node.Parallel == 0 {
				return cfg, fmt.Errorf("parallel: 0 in sequence %s node %s, expected parallel > 0", sequence.Name, node.Name)
			}
			if node.Retry > 0 {
				// retry is set, so parse retryWait if set, else default to 0s
				if node.RetryWait != "" {
					if _, err := time.ParseDuration(node.RetryWait); err != nil {
						return cfg, fmt.Errorf("error in '%s %s' node: retryWait: %s is not a valid duration:  %s", node.NodeType, node.Name, node.RetryWait, err)
					}
				} else {
					node.RetryWait = "0s"
				}
			} else if node.RetryWait != "" {
				// If no retry, then retryWait shouldn't be set
				return cfg, fmt.Errorf("error in '%s %s' node: retryWait: %s is set but retry is not set", node.NodeType, node.Name, node.RetryWait)
			}

			for i, nodeSet := range node.Sets {
				if nodeSet.As == nil {
					node.Sets[i].As = &nodeSet.Arg
				}
			}
		}

		// Validate ACLs, if any
		seen := map[string]bool{}
		for _, acl := range sequence.ACL {
			if acl.Admin && len(acl.Ops) != 0 {
				return cfg, fmt.Errorf("invalid user ACL for %s in %s: admin=true and ops are mutually exclusive; set admin=false or remove ops", sequenceName, configFile)
			}
			if acl.Role == "" {
				return cfg, fmt.Errorf("invalid user ACL for %s in %s: role is not set (empty string); it must be set", sequenceName, configFile)
			}
			if seen[acl.Role] {
				return cfg, fmt.Errorf("duplicate user ACL for %s in %s: role=%s", sequenceName, configFile, acl.Role)
			}
			seen[acl.Role] = true
		}
	}

	return cfg, nil
}

// isSequence will return true if j is a Sequence, and false otherwise.
func (j *NodeSpec) isSequence() bool {
	return j.Category == "sequence"
}

// isSequence will return true if j is a Sequence, and false otherwise.
func (j *NodeSpec) isConditional() bool {
	return j.Category == "conditional"
}
