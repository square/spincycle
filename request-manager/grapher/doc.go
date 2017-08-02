/*
Package Grapher provides capabilities to construct directed acyclic graphs
from configuration yaml files. Specifically, the graphs that Grapher can
create are structure with the intent to organize workflows, or 'job chains.'

The implementation will read in a yaml file (structure defined below) and
will return a graph struct. Traversing the graph struct can be done via
two methods: a linked list structure and an adjacency list (also described
below).

Basics

* Graph - The graph is defined as a set of Vertices and a set of
directed edges between vertices. The graph has some restrictions on it
in addition to those of a traditional DAG. The graph must:
	* be acyclic.
	* ave only single edges between vertices.
	* contain exacly one source node and one sink node.
	* each node in the graph must be reachable from the source node.
	* the sink node must be reachable from every other node in the graph.
+ The user need not explicitly define a single start/end node in the config
file. Grapher will insert "no-op" nodes at the start/end of sequences to
enforce the single source/sink rule.

* Node - A single vertex in the graph.

* Sequence - A set of Nodes that form a subset of the main Graph. Each
sequence in a graph will also follow the same properties that a graph
has (described above).

Config Structure

The yaml file to encode the graph types are defined with the outermost
object being the Sequence. Sequences is a map of "sequence name" ->
Sequence Spec.
Sequence Specs have two fields: "args" and "nodes."
The "args" field is made up of required and optional arguments.
The "nodes" field is a map of node structures.

The nodes list is a map of node name -> node spec. Each node spec is
defined as:
	* category: "job"|"sequence" - this indicates whether the specified node
	represents a single vertex, or actually refers to another sequence defined.

	* type: this refers to the actual type of data to assign to the node, and will
	be stored in the Payload of the node.

	* args: refers to the arguments that must be provided to the node during creation.
	There is another level of mapping arguments here. Each argument consists of a
	"expected" field, which is the name of the variable that is expected by the node
	creator, and the "given" field, which indicates the variable that is actually given by
	grapher to the job creator.

	* sets: this defines the variables that will be set on creation of a Node. This, used
	in conjunction with the "args" of later nodes can be used to pass data
	from one node to the other on creation of a graph.

	* deps: This lists the nodes that have out edges leading into the node defined by
	this spec.

	* each: This signifies to grapher that the node (or sequence) needs to be
	repeated in parallel a variable number of times. The format of this struct is
	given as `each: foos:foo`, and can be read as "repeat for each foo in foos" The
	variable `foos` must necessarily be an argument given to the node.

	* retries: the number of times that a node will be retried if it fails. This field
	only applies to nodes with category="job" (i.e., does not apply to sequences).

	* retryDelay: the delay, in seconds, between retries of a node. This field only
	applies to nodes with category="job" (i.e., does not apply to sequences).

Also included in the config file, is

Example Config

The full example file can be foind at grapher/test/example-requests.yaml
	sequences:
		decommission-cluster:
		  args:
		    required:
		      - name: cluster
		      - name: env
		    optional:
		      - name: something
		        default: 100
		  nodes:
		    get-instances:
		      category: job
		      type: get-cluster-instances
		      args:
		        - expects: cluster
		          given: cluster
		      sets: [instances]
		      deps: []
		      retries: 3
		      retryDelay: 10
		    delete-job:
		      category: job
		      type: delete-job-1
		      args:
		        - expects: cluster
		          given: cluster
		        - expects: env
		          given: env
		        - expects: instances
		          given: instances
		      sets: []
		      deps: [pre-flight-checks]
		    pre-flight-checks:
		      category: sequence
		      type: check-instance-is-ok
		      each: instances:instance # repeat for each instance in instances
		      args:
		        - expects: instances
		          given: instances
		      deps: [get-instances]
		check-instance-is-ok:
		  args:
		    required:
		      - name: instance
		    optional:
		  nodes:
		    check-ok:
		      category: job
		      type: check-ok-1
		      args:
		        - expects: container
		          given: instance
		      sets: [physicalhost]
		      deps: []
	noop-node:
	  category: job
	  type: no-op    # this is left up to the user to define in their jobs repo

*/
package grapher
