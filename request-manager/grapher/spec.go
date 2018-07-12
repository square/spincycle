// Copyright 2017-2018, Square, Inc.

package grapher

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// NodeSpec defines the structure expected from the yaml file to define each nodes.
type NodeSpec struct {
	Name         string     `yaml:"name"`      // unique name assigned to this node
	Category     string     `yaml:"category"`  // "job" or "sequence"
	NodeType     string     `yaml:"type"`      // the type of job or sequence to create
	Each         []string   `yaml:"each"`      // arguments to repeat over
	Args         []*NodeArg `yaml:"args"`      // expected arguments
	Sets         []string   `yaml:"sets"`      // expected job args to be set
	Dependencies []string   `yaml:"deps"`      // nodes with out-edges leading to this node
	Retry        uint       `yaml:"retry"`     // the number of times to retry a "job" that fails
	RetryWait    uint       `yaml:"retryWait"` // the time, in seconds, to sleep between "job" retries
}

// NodeArg defines the structure expected from the yaml file to define a job's args.
type NodeArg struct {
	Expected string `yaml:"expected"` // the name of the argument that this job expects
	Given    string `yaml:"given"`    // the name of the argument that will be given to this job
}

// SequenceSpec defines the structure expected from the config yaml file to
// define each sequence
type SequenceSpec struct {
	Name    string               `yaml:"name"`  // name of the sequence
	Args    SequenceArgs         `yaml:"args"`  // arguments to the sequence
	Nodes   map[string]*NodeSpec `yaml:"nodes"` // list of nodes that are a part of the sequence
	Retry   uint                 `yaml:"retry"` // the number of times to retry the sequence if it fails
	Request bool                 // whether or not the sequence spec is a user request
}

// SequenceArgs defines the structure expected from the config file to define
// a sequence's arguments. A sequence can have required arguemnts; an arguments
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
	Name    string `yaml:"name"`
	Desc    string `yaml:"desc"`
	Default string `yaml:"default"`
}

// All Sequences in the yaml. Also contains the user defined no-op job.
type Config struct {
	Sequences map[string]*SequenceSpec `yaml:"sequences"`
}

// ReadConfig will read from configFile and return a Config that the user
// can then use for NewGrapher(). configFile is expected to be in the yaml
// format specified.
func ReadConfig(configFile string) (*Config, error) {
	sequenceData, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = yaml.Unmarshal(sequenceData, cfg)
	if err != nil {
		return nil, err
	}

	for sequenceName, sequence := range cfg.Sequences {
		sequence.Name = sequenceName
		for nodeName, node := range sequence.Nodes {
			node.Name = nodeName
		}
	}

	return cfg, nil
}

// isSequence will return true if j is a Sequence, and false otherwise.
func (j *NodeSpec) isSequence() bool {
	return j.Category == "sequence"
}
