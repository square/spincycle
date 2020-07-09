---
layout: default
title: Jobs
parent: Develop
nav_order: 2
---

# Jobs

A job is an atomic, reusable unit of work. Under the hood, requests are directed acyclic graphs with jobs as the vertices. Ideally, every job in a request is a single step or task to accomplish the request. Outside a request, the best jobs are generic and reusable. For example, a job that adds a virtual IP address to a network interface is generic and reusable in many different requests. Inevitably, however, some jobs will be specific and used in only a single requests. The main goal is that one job does one thing. More reusable jobs makes your [jobs repo](/spincycle/v2.0/learn-more/jobs-repo) more values, and new requests faster to create.

**Every job must implement the [job.Job interface](https://godoc.org/github.com/square/spincycle/job#Job)**. Spin Cycle only uses this interface, so it has zero knowledge of job implementation details.

## Life Cycle

It is critical to understand the life cycle of a job in a request because it affects how jobs must be written, depending on what the job does. The life cycle has two major and separate phases: create and run. These happen in the Request Manager (RM) and Job Runner (JR), respectively. In each phase is two stages.

### Create

_1. Create_

Jobs are first created when a request is being created by the RM. As the RM processes the request spec, it instantiates jobs by calling the `Make` method on the `jobs.Factory` from your jobs repo to create each job. Jobs should _not_ do any work, logic, etc. when instantiated. Simply create the new job object and return.

After instantiating the job object, the RM calls its `Create` method. This is when and where jobs should do work, logic, etc. Which work depends on the jobs because, at this point, the job is only being created, not ran. The `Create` method is used to "set the stage". For example, a job might query other APIs or a database to ascertain the state of world and set some job args (described later) which influence the creation of other jobs.

_2. Serialize_

Since only the Job Runner runs jobs, after `Create` the RM calls the `Serialize` method. The job should serialize itself into a `[]byte` which is transmitted to the JR.

Serialize only data necessary and sufficient to run later. Do not serialize what can be recreated later. An internal `*sync.Mutex` is an example: do not serialize it, recreate it later.

### Run

_3. Deserialize_

When the JR receives a request from the RM to run, it first deserializes jobs by calling the `Deserialize` method. (Technically, before this it instantiates the job again by calling `jobs.Factory.Make`.) The job should deserialize itself and initialize any internal, private variables. For example, a job might initialize a new `*sync.Mutex`.

The job should not do any work or logic (other than deserialization work and logic) in `Deserialize`.

_4. Run_

The final stage in the life of a job is when the JR calls its `Run` method. As the name implies, this when the job officially runs. However, depending on what the job does, it might not have done everything in `Create`. That is ok. Later, we discuss create-only vs. run-only jobs. Normally, though, a job does all its "heavy lifting" in `Run`. For example, job "add-ip" would run `sudo /sbin/ip addr add "$IP/32" dev "$IFACE"` to add the IP to the network interface. Jobs can be very short or very long-running (hour and days).

The job returns a [job.Return](https://godoc.org/github.com/square/spincycle/job#Return) which is important: if `State != proto.STATE_COMPLETE` (see [proto.go](https://godoc.org/github.com/square/spincycle/proto#pkg-constants)), the JR runner might stop running the whole request. If the job or sequence is configured with retries, the JR will retry, else it will fail the whole request. A complete (successful) job allows the JR to run the next jobs, or wait for other jobs to complete to satisfy dependencies in the graph describing the request.

When a job is done, the JR sends a [job log entry (JLE)](https://godoc.org/github.com/square/spincycle/proto#JobLog) to the RM which stores in it MySQL. Use `spinc log` to see the job log.

## Job Args and Data

Jobs are created with job args: `Create(jobArgs map[string]interface{}) error`. Job args are initialized from request args: the required and optional arguments listed in the request spec, the values of which are provided by the caller when starting the request. Jobs use, set, and modify job args when created in the RM. Job args, like normal function arguments, help determine what a job does. For example, job "shutdown-host" could required job arg "hostname" which determines which host to shut down. That job arg could originate from a request arg (i.e. caller specifies hostname=...) or be determined and set by an earlier job. Either way, job args are used only at creation in the RM, and they form an immutable snapshot of work: request args + job args + jobs = everything the request will do or did do.

This is an important concept: _creating a request yields an immutable snapshot of all work._

Job args, although changing during creation, are ultimately static once the request is created. Everything is stored (by the RM in its MySQL instance) so that we can always look back at any request and see what it worked on. By contrast, if an immutable record was not kept and args were _only_ determined at runtime, we would need to rely on logs to determine which hosts the "shutdown-host" shut down, for example. Instead, this information is recorded with the request in the final job args (and also logged).

Occasionally, there is a need for run-time data: `Run(jobData map[string]interface{}) (Return, error)`. Job data is obtained only at runtime when the JR runs the job. The canonical example is MySQL replication coordinates (a binary log file name and byte offset in that file). Replication coordinates cannot be obtained at creation because they are always changing. To use repl coordinates, you must obtain them at the moment they are used.

_Job args are almost always the correct choice_. You only need to use job data if the information _must_ be obtained when it is used. Else, use job args to ensure that Spin Cycle can record a complete, immutable snapshot of all work it will (or did) do for the request.

### Job Data and Suspending Requests

When jobs are suspended, job data is stored in a JSON file. When jobs are resumed, they are unserialized via [json.Unmarshal](https://golang.org/pkg/encoding/json/#Unmarshal), which may change the types of some data, e.g. all numbers become type `float64`, and all arrays become `[]interface{}`. (See the json documentation for more.) Jobs must be able to handle these altered data types in order for a request to be resumed successfully.

## Job Patterns

Every job must implement the [job.Job interface](https://godoc.org/github.com/square/spincycle/job#Job), but some jobs really only need the `Create` or `Run` methods to do all work. This is normal and produces two common "job patterns".

### Create-only Job

Create-only jobs do all their work in the `Create` method. Usually, they set new job args that subsequent jobs require. For example, let's say request "stop-container" stops a Docker container. Since that container runs on a physical host, the request needs to know the host. There are two ways to solve this:

1. Make the physical hostname a request arg
2. Make a job to determine the physical hostname

Option 1 makes the caller do all the work by providing all the inputs: container hostname and physical host hostname. But option 2 is better because the purpose of Spin Cycle is automation, so make the caller do the least amount of work and automate the rest. A job, let's call it "host-of-container", would require job arg "container-hostname" and from this determine and set job arg "host-hostname". All work would be done in the `Create` method, and the other methods would be stubs:

```go
func (j hostOfContainer) Create(jobArgs map[string]interface{}) error {
    v, ok := jobArgs["container-hostname"]
    if !ok {
        fmt.Errorf("container-hostname job arg not set")
    }
    containerHostname, ok := v.(string)
    if !ok {
        fmt.Errorf("container-hostname job arg value is not a string")
    }

    // Using containerHostname, determine...
    physicalHost := ...

    // Save physical host hostname in job args, used by other jobs
    jobArgs["host-hostname"] = physicalHost

    return nil
}

func (j hostOfContainer) Serialize() ([]byte, error) {
    return nil, err // do nothing
}

func (j hostOfContainer) Deserialize([]byte) error {
    return nil // do nothing
}

func (j hostOfContainer) Run(jobData map[string]interface{}) (Return, error) {
    return proto.Return{State: proto.STATE_COMPLETE}, nil // do nothing
}
```

Even though the job does nothing in `Run`, it _must_ return `proto.Return{State: proto.STATE_COMPLETE}`.

### Run-only Job

Run-only jobs are the reciprocal of create-only jobs: all work is done in the `Run` method, and the other methods are stubs. Usually, a run-only job has to serialize some job args to know how to run later. Extending the previous example of request "stop-container", another job would actually stop the container. Let's call that job "stop-container". It requires two job args: "container-hostname" (provided by caller as a request arg) and "host-hostname" (set by "host-of-container" job):

```go
type StopContainer struct {
    ContainerHostname string
    HostHostname      string
}

func (j *stopContainer) Create(jobArgs map[string]interface{}) error {
    // Validation and type casting not shown
    j.ContainerHostname = jobArgs["container-hostname"]
    j.HostHostname = jobArgs["host-hostname"]
    return nil
}

func (j *stopContainer) Serialize() ([]byte, error) {
    return json.Marshal(j)
}

func (j *stopContainer) Deserialize(bytes []byte) error {
    return json.Unmarshal(bytes, j)
}

func (j *stopContainer) Run(jobData map[string]interface{}) (Return, error) {
    // Stop j.ContainerHostname on j.HostHostname
    // ...
    return proto.Return{State: proto.STATE_COMPLETE}, nil // do nothing
}
```

Granted, the other methods are not pure stubs, but they do no work or logic. `Create` only saves the two job args that `Run` will need. Saving these as public (exported) fields in the job structure is a quick trick for handling `Serialize` and `Deserialize`: package `encoding/json` only works on public fields, so this serializes only the job args and deserializes them back into place. `Run` does all the work.
