---
layout: default
title: Basic Concepts
parent: Learn More
nav_order: 1
---

# Basic Concepts

_Requests_, _sequences_, and _jobs_ are the building blocks of Spin Cycle:

![Generic Request-Sequence-Job Diagram](/spincycle/assets/img/request_sequence_job_generic.svg)

### Request

A request is something users can ask Spin Cycle to do. For example, request "stop-mysql" allows users to stop MySQL. You create requests; Spin Cycle has no built-in requests. Requests are any functionality you want to expose to users. For example, the database team at Square exposes "stop-host" and "start-host" requests so that the hardware team can safely stop and start database hosts.

From the user's point of view, everything is a request. Requests can have required and optional arguments ("args"), the values of which are provided by the user. A "stop-host" request probably has a required "hostname" arg to let the user specify which host to stop. Apart from the request args, the request is a black box to the user: users don't know (and shouldn't know) how requests are accomplished.

### Sequence

A sequence is a unique set of jobs. (Don't worry what jobs are, those are explained next.) Sequences are an implementation detail of requests that users don't see. You&mdash;a Spin Cycle operator and developer&mdash;create sequences and combine them to create requests. In the diagram above, two sequences comprise the request.

Ideally, sequences are reusable, but they don't have to be. For example, the database team at Square has a "provision-database-node" sequence which provisions a single database instance. This sequence is used by different requests. For example, it's used by the request that provisions a new database cluster, and it's used by the request that adds a database node to an existing cluster.

### Job

A job is an atomic, reusable unit of work. Of the three building blocks, jobs are the most important because they do actual work. (Under the hood, requests are directed acyclic graphs, sequences are subgraphs, and jobs are vertices/nodes.) For example:

* _add-ip-addr_: Add an IP address to a network interface
* _wait-port_: Wait for a port to open (process listening on port)
* _install-pkg_: Install a package (apt/yum/etc.)
* _create-cname-record_: Create a CNAME DNS record
* _set-mysql-read-only_: Enable MySQL read-only (disable writes)

These jobs are very small and complete (atomic), very reusable, and do actual work. If they seem too small in isolation, remember: multiple jobs form sequences, and sequences can be a much larger units of work. Instead of having "big" jobs that do multiple things, write small jobs that do one thing, then combine the jobs into a sequence to logically encapsulate the bigger unit of work. For example, let's pretend that stopping MySQL involves:

1. Enable read-only
2. Disconnect clients
3. Stop mysqld

A single job could do all three steps, but it's better to write three jobs&mdash;one for each step&mdash;and create a sequence called "stop-mysql" that executes the three jobs. This makes each job reusable in other sequences, and makes the sequence reusable in other requests. Also, smaller jobs are easier to understand, write, debug, and test&mdash;especially for other engineers.

You will write _a lot_ of jobs. For example, the request to provision a new database cluster at Square is over 100 jobs. Spin Cycle requires understanding and programming every step of an infrastructure task. It's rigorous but required for platform automation at scale.

## Request Specs

"Specs" are YAML files where you define requests, sequences, and jobs in sequences. The spec for the diagram above looks like:

```yaml
---
sequences:
  stop-host:
    request: true
    args:
      required:
        - name: hostname
          desc: "Hostname of machine to stop"
    nodes:
      job-a:
        category: job
        type: jobs/a
        args:
          - expected: hostname
            given: hostname
        sets:
          - arg: new-arg
        deps: []
      job-b:
        category: job
        type: jobs/b
        args: []
          - expected: new-arg
            given: new-arg
        sets: []
        deps: [job-a]
      do-seq-2:
        category: sequence
        type: seq-2
        args: []
        deps: [job-b]
```
Don't worry about the details, neither the diagram nor the spec are complete, they're only basic examples to give you an idea what sequence specs look like.

Request specs are necessarily meticulous because you must specify every sequence, job, arg, and how they connect. (Under the hood, Spin Cycle creates a directed acyclic graph.) This requires upfront work, but trust us: the long-term benefits greatly exceed the short-term costs. One reason why: reusable sequences and jobs develop a flywheel effect, so new requests become quicker to create because the building blocks already exist.

## Sequence Expansion

Sequence expansion refers to generating requests from static specs and variables args. Consider request "shutdown-host" to shutdown a physical host that runs a variable number of virtualized hosts. For example, a physical host could be running virtual host1, virtual host2, etc. We want "shutdown-host" to require only the physical hostname, automatically determine the virtual hosts on it, and shut them down in parallel. Solution:

![Sequence Expansion](/spincycle/assets/img/sequence_expansion.svg)

One job (not shown) takes required arg "physicalHost", determines the virtual hosts, and sets a new arg "hosts" (list of virtual hostnames on "physicalHost"). Sequence 1 is expanded on each value in "hosts", passing the current list element as arg "hostname" to the expanded sequence (Sequence 1-1 and Sequence 1-2). The lone job in the sequence expects args "hostname", which it uses to stop that virtual host.

Sequence 1 is statically defined in the request spec. When requested, the user arg "physicalHost" and a job (not shown) sets arg "hosts". With sequence expansion, the static spec yields a different request based on the args.

Sequence expansion is not required but it should be used. For example, instead of expanding a sequence, a job could iterate on "hosts". Spin Cycle doesn't prevent this, but it should be avoided unless needed. Sequence expansion has many benefits. One benefit is parallelism: expanded sequence run in parallel. Another benefit is isolation: expanded sequences execute independently, which is important for error handling, sequence retry, etc.

Sequence expansion is to Spin Cycle what [xargs](http://man7.org/linux/man-pages/man1/xargs.1.html) is to the command line.

## Request Manager 

The Request Manager (RM) is the user-facing API. Users and clients only communicate with the RM. As its name suggests, it's responsible for requests: generating from specs, storing in MySQL, status, logging, etc. Client authentication and authorization happens in the RM. From the user's point of view, Spin Cycle and the Request Manager are the same.

Behind the scenes, the RM is one of two APIs that comprise Spin Cycle (the Job Runner is the other). The RM has its own configuration and is a separate build and deployable.

## Job Runner

The Job Runner (JR) is an API that runs jobs. Only the RM communicates with the JR. There are no user-facing JR API endpoints. After the RM generates and stores a request, it sends the request to the JR which runs the jobs. Since requests are directed acyclic graph under the hood, the JR is graph traverser. It executes jobs in the correct order and handles dependencies, retries, errors, etc. When a job completes (or is retried), the JR sends a job log entry (JLE) to the RM which stores it. When requested by a user through the RM, the JR reports the real-time job status of every job currently running. When a JR instance is stopped, it suspends running jobs and sends them back to any RM instance, which tries to resume the jobs by sending them back to any available JR instance. This is the basic functionality of Spin Cycle high availability.

## Job Factory

The job factory makes jobs. Spin Cycle has no built-in jobs and doesn't introspect or automatically discover jobs. Instead, you provide a job factory which Spin Cycle uses to make jobs specified in requests. For example, in the request spec above there is:

```yaml
job-a:
  category: job
  type: jobs/a
```

Spin Cycle calls the job factory to make a `jobs/a` job. The job name is `job-a` (from the spec), and Spin Cycle gives the newly created job an ID that's unique in the request. Internally, Spin Cycle uses the job ID because job type and name are not unique (a request can reuse the same job type, and sequences can reuse the same job name).

Both Request Manager and Job Runner use the job factory. The RM makes jobs when generating the request, and the JR makes jobs when running the request. Don't worry about the difference and details; it will become more clear in the development section of the docs.

## High-level System Diagram

The aforementioned concepts and components form Spin Cycle at a high level:

![Spin Cycle High-Level](/spincycle/assets/img/spincycle_high_level.svg)

There's more to learn and do, but everything relates to the basic concepts presented here.
