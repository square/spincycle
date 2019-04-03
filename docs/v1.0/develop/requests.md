---
layout: default
title: Requests
parent: Develop
nav_order: 3
---

# Requests

Requests are specified in YAML files. On startup, the Request Manager (RM) reads all specs: all the .yaml files in [specs.dir](/spincycle/v1.0/operate/configure#rm.specs.dir). Subdirectories are not currently supported, so be sure to combine all specs in one directory.

All spec files have the same syntax. The RM combines specs from multiple files to complete a request. One sequence per file and descriptively named files help keep all the specs oranized and easy to find by humans.


## Sequence Spec

The root of every spec file is a sequence spec:

```yaml
---
sequences:
  stop-container:
    request: true
    args:
      required:
        - name: containerName
          desc: "Container name to stop"
      optional:
        - name: restart
          desc: "Restart the container if \"yes\""
          default: ""
      static:
        - name: slackChan
          default: "#dba"
    acl:
      - role: eng
        ops: admin
      - role: ba
        ops: ["start","stop"]
    nodes:
      NODE_SPECS
```

This defines one or more sequences under `sequences:`. Although multiple sequence can be defined in a single file, we suggest one sequence per file.

The example above defines one sequence called "stop-container". (We would name this file stop-container.yaml.) If `request: true`, the sequence is a request that callers can make. This also makes [spinc](/spincycle/v1.0/operate/spinc) (ran without any command line options) list the request. Set `request: true` only for top-level sequences that you want to expose to users as requests. All requests are sequences, but not all sequences are requests. To distinguish:

* request: a sequence with `request: true`
* non-request sequence (NRS): a sequence with `request: false` (usually omitted)
* sequence: any and all sequences, generally speaking

### args:

Sequences have three types of arguments (args): required, optional, and static.

* `required:` args are, unsurprisingly, required. For requests, required args are provided by the caller, so they should be kept to a minimum&mdash;require the user to provide only what is necessary and sufficient to start the request, then figure out other args in jobs. For non-request sequences (NRS), required args are provided by the parent node (nodes are discussed in the next section).
* `optional:` args are optional. If not explicitly given, the default value in the spec is used. In the example above, arg "restart" defaults to an empty string unless the user provides a value.
* `static:` args are fixed values. Static arg "slackChan" has value "#dba". Static args are useful when the value is known but differs in different sequences. For example, another request might set slackChan=#yourTeam to get Slack notifications at #yourTeam instead of #dba. This could also be solved by making slackChan a required or optional arg.

In [job args](/spincycle/v1.0/develop/jobs#job-args-and-data), there are no distinctions. `jobArgs["slackChan"]` is the same as `jobArgs["containerName"]`, and jobs can change its value.

## Node Specs

A sequence is one or more node (vertex in the graph) defined under `nodes:`. There are three types of node specs. Shared fields (e.g. `retry:`) are only described once.

### Job Node

A job node specifies a job to run. (If this was a tree data structure, these would be leaf nodes.) Every sequence eventually leads to job nodes. This is where work happens:

```yaml
      expand-cluster:
        category: job
        type: etre/expand-cluster
        args:
          - expected: cluster
            given: cluster
        sets:
          - app   # string
          - env   # string
          - nodes # []string
        retry: 2
        retryWait: 3s
        deps: []
```

All node specs begin with a node name: "expand-cluster", in this case. Node names must be unique within the sequence. (Spin Cycle makes nodes unique within a request by assigning them an internal job ID.) `category: job` makes this node a job node. `type:` specifies the job type: "etre/expand-cluster". The `jobs.Factory` in your [jobs repo](http://localhost:4000/spincycle/v1.0/learn-more/jobs-repo) must be able to make a job of this type.

`args:` lists all job args that the job requires. `expected:` is the job arg name that the job expects, and `given:` is the job arg name in the specs to use. In other words,  `jobArgs[expected] = jobArgs[given]`. This is useful because it is nearly impossible to make all job args in specs match all job args in jobs. For example, a spec might use "host" for a server's hostname, but a job uses "hostname". In this case,

```
- expected: hostname
  given: host
```

makes Spin Cycle do `jobArgs["hostname"] = jobArgs["host"]` before passing `jobArgs` to the job.

If expected == given, both must still be specified.

Only job args listed under `args:` are passed to the job. If a job needs arg "foo" but "foo" is not listed, then `jobArgs["foo"]` will be nil in the job. This requirement is strict and somewhat tedious, but it makes specs complete self-describing and easy to follow because there are no "hidden" args.

If a job has optional args, they must be listed so they are passed to the job, in case they exist. The job is responsible for using the optional args or not. (Note: "optional" here is not the same as sequence-level optional args.)

`sets:` specifies the job args that the job sets. The RM checks this. In the example above, the job sets "app", "env", and "node" in `jobArgs`. After calling the job's `Create` method, the RM checks that all three are set in `jobArgs` (with any value, including nil). Like `args:`, this is strict but makes it possible to follow every arg through different sequences. It also makes it explicit which jobs set which args.

`retry:` and `retryWait:` specify how many times the JR should retry the job if `Run` does not return `proto.STATE_COMPLETE`. The job is always ran once, so total runs is 1 + `retry`. `retryWait` is the wait time between tries. It is a [time.Duration string](https://golang.org/pkg/time/#ParseDuration) like "3s" or "500ms". If not specified, the default is no wait between tries.

`deps:` is a list of node names that this node depends on. For nodes A and B, if B depends on A, the graph is A -> B. The JR runs B only after A completes successfully. A node can depend on many nodes, creating fan-out and fan-in points:

```
      B ->
    /      \
A ->        +-> E
    \      /
      C ->
```

Node E has `deps: [B,C]`. Nodes B and C have `deps: [A]`. Node A has `deps: []`.

`deps:` determines the order of nodes, not the order of node specs in the file. Every sequence must have a node with `deps: []` (the first node in the sequence). Cycles are not allowed.

### Sequence Node

All node specs begin with a node name: "notify-app-owners", in this case. `category: sequence` makes this node a sequence node. `type:` specifies the sequence name: "notify-app-owners". A node and sequence can have the same name. Whereas a job node runs a job, a sequence node imports another sequence.

```yaml
      notify-app-owners:
        category: sequence
        type: notify-app-owners
        args:
          - expected: appName
            given: app
          - expected: env
            given: env
        deps: [expand-cluster]
        retry: 9
        retryWait: 5000 # ms
```

When the RM encounters this sequence node, it looks for a sequence called "notify-app-owners". (We would put that sequence in a file named notify-app-owners.yaml.) It replaces the sequence node with all the nodes in the target sequence. Since sequences can have required args, the sequence node must specify the `args:` to pass to the sequence (as if the sequence was a request). The same rules about `args:`, `exepected:`, and `given:` apply. The only difference is that the job args are passed to a sequence instead of a job.

Sequences cannot set job args, so `set:` can be omitted.

The same rules about `deps:` apply (described above). In this example, the "notify-app-owners" sequence is not called until the "expand-cluster" node is complete and successful. Likewise, if another node `deps: [notify-app-owners]`, it is not called until the entire sequence is complete and successful. The sequence node, at this point in the spec, acts like a single node&mdash;it just happens to contain/run other nodes and sequences.

`retry:` and `retryWait:` apply to sequences, too. If any job in the sequence fails, the entire sequence is retried from its beginning.

#### Sequences of Sequences

Sequences "calling" sequences are how large requests are built. Like a job, a sequence is a unit of work&mdash;a bigger unit of work. The "notify-app-owners" sequence, for example, might have several jobs which detremine who the app owners are, what their notification preferences are, and then notify them accordingly. That is one unit of work: notifying app owners. It is also a reusable unit of work.

### Conditional Node

`category: conditional` makes this node a conditional node. `if:` specifies the job arg to use, and `eq:` operates like a switch cases on the `if:` job arg value.

```yaml
      restart-vttablet:
        category: conditional
        if: vitess
        eq:
          yes: restart-vttablet
          default: noop
        args:
          - expected: node
            given: name
          - expected: host
            given: physicalHost
        deps: [bgp-peer-node]
```

In the example above, if `jobArgs["vitess"] == "yes"`, then sequence "restart-vttablet" is called. Else, the `default` is a special, built-in sequence called "noop" which does nothing. In the case that `jobArgs["vitess"] == "yes"` and sequence "restart-vttablet" is called, the node acts exactly like a sequence node.

All values and comparison are expected to be strings. The `if:` job arg must be set with a string value, and the values listed under `eq:` are string values, except "default" which is a special case.

Conditional nodes can be used to switch between alternatives or, like the example above, do nothing in one part of a spec (but do everything else before and after).

## Sequence Expansion

[Sequence expansion](/spincycle/v1.0/learn-more/basic-concepts#sequence-expansion) is possible in sequence and conditional nodes with `each:`:

```yaml
      decomm-nodes:
        category: sequence
        type: decomm-node
        each:
          - nodeHostname:node
          - hosts:host
        args: []
          - expected: archiveData
            given: archiveData
        deps: []
```

`each:` takes a list of job arg names and "expands" the sequence, "decomm-node", _in parallel_ for each job arg value. Expanded sequences are ran in parallel. There are currently no options to control this.

The job args must be type `[]string` of equal lengths. In this example, the job args could be:

```go
nodeHostname := []string{"node1", "node2"}
hsots := []string{"host1", "host2"}
```

The syntax is `arg:alias` ("arg as alias") where each `jobArg[alias]` is initialized from the next value in `arg`. The target sequence should require `alias`.

The `args:` are passed to each expanded sequence as-is, i.e. each "decomm-node" sequence receives `jobArgs[archiveData]`.

A conditional node with sequence expansion expands the sequence that matches `if:` and `eq:`.

The same rules about `deps:` apply (described above).
