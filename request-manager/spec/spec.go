// Copyright 2020, Square, Inc.

// Structs in this file completely describe the expected structure of the yaml file.
//
// Some fields are pointers. Having a field as a pointer allows us to determine
// whether or not a field was explicitly set: if a field is a pointer, and the field was
// not explicitly set in the yaml file, the value is nil. If a field is not a pointer, and
// the field was not explicitly set in the yaml file, the value is the zero value.
//
// Some fields must always be explicitly set, even if only to the zero value. These are pointers.
//
// Other fields are optional:
//   Sometimes, the zero value is the default value. These are not pointers.
//   If the default value is not the zero value, the field is a pointer.

package spec

// Nodes in a sequence.
type Node struct {
	Name         string            `yaml:"-"`         // unique name assigned to this node
	Category     *string           `yaml:"category"`  // "job", "sequence", or "conditional"
	NodeType     *string           `yaml:"type"`      // the type of job or sequence to create
	Each         []string          `yaml:"each"`      // arguments to repeat over
	Args         []*NodeArg        `yaml:"args"`      // expected arguments
	Parallel     *uint             `yaml:"parallel"`  // max number of sequences to run in parallel
	Sets         []*NodeSet        `yaml:"sets"`      // expected job args to be set
	Dependencies []string          `yaml:"deps"`      // nodes with out-edges leading to this node
	Retry        uint              `yaml:"retry"`     // the number of times to retry a "job" that fails
	RetryWait    string            `yaml:"retryWait"` // the time to sleep between "job" retries
	If           *string           `yaml:"if"`        // the name of the jobArg to check for a conditional value
	Eq           map[string]string `yaml:"eq"`        // conditional values mapping to appropriate sequence names
}

// A node's args (i.e. the `args` field).
type NodeArg struct {
	Expected *string `yaml:"expected"` // the name of the argument that this job expects
	Given    *string `yaml:"given"`    // the name of the argument that will be given to this job
}

// Args set by a node (i.e. the `sets` field).
type NodeSet struct {
	Arg *string `yaml:"arg"` // the name of the argument this job outputs by default
	As  *string `yaml:"as"`  // the name of the argument this job should output
}

// A single sequence.
type Sequence struct {
	Name     string           `yaml:"-"`       // name of the sequence
	Args     SequenceArgs     `yaml:"args"`    // arguments to the sequence
	Nodes    map[string]*Node `yaml:"nodes"`   // list of nodes that are a part of the sequence
	Request  bool             `yaml:"request"` // whether or not the sequence spec is a user request
	ACL      []ACL            `yaml:"acl"`     // allowed caller roles (optional)
	Filename string           `yaml:"_"`       // name of file this sequence was in
}

// A sequence's arguments. A sequence can have required arguments; any arguments
// on this list that are not provided by the calling sequence will result in an
// error from template.Grapher.
// A sequence can also have optional arguemnts; arguments on this list that are
// missing will not result in an error. Additionally optional arguments can
// have default values that will be used if not explicitly given.
type SequenceArgs struct {
	Required []*Arg `yaml:"required"`
	Optional []*Arg `yaml:"optional"`
	Static   []*Arg `yaml:"static"`
}

// A sequence's args.
type Arg struct {
	Name    *string `yaml:"name"`
	Desc    string  `yaml:"desc"`
	Default *string `yaml:"default"`
}

// A single role-based ACL entry. Every auth.Caller (from the
// user-provided auth plugin Authenticate method) is authorized with a matching
// ACL, else the request is denied with HTTP 401 unauthorized. Roles are
// user-defined. If Admin is true, Ops cannot be set.
type ACL struct {
	Role  string   `yaml:"role"`  // user-defined role
	Admin bool     `yaml:"admin"` // all ops allowed if true
	Ops   []string `yaml:"ops"`   // proto.REQUEST_OP_*
}

// A collection of sequences. This can be all the sequences in a single yaml file.
// It is also used to hold all sequences in the specs directory.
// Also contains the user defined no-op job.
type Specs struct {
	Sequences map[string]*Sequence `yaml:"sequences"`
}

func (j *Node) IsJob() bool {
	return j.Category != nil && *j.Category == "job"
}

func (j *Node) IsSequence() bool {
	return j.Category != nil && *j.Category == "sequence"
}

func (j *Node) IsConditional() bool {
	return j.Category != nil && *j.Category == "conditional"
}
