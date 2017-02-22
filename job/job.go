// Copyright 2017, Square, Inc.

// Package job provides job-related interfaces, data structures, and errors.
// To avoid an import cycle, this package must not have external dependencies
// because everything else depends on it.
package job

// A Job is the smallest, reusable building block in Spin Cycle that has meaning
// by itself. A job should, ideally, do one thing. For example: "DownSIP" brings
// down a SIP. This job is meaningful by itself and highly reusable.
//
// Spin Cycle defines the Job interface, but jobs are provided by an external
// repo (imported in external/jobs.go). This is known as "BYOJ": bring your own
// jobs. A job must implement and be able to accomplish its purpose only through
// this interface because Spin Cycle only uses this interface.
//
// The Job interface has two sides: one for the Request Manager (RM), the other
// for the Job Runner (JR). The RM calls Create and Serialize, and the JR calls
// the other methods. The call sequence is: Create, Serialize, Deserialize, Run.
type Job interface {
	// Create allows the job to get and save internal data needed to run later.
	// The job can save jobArgs and set new ones for other jobs.
	//
	// This method is only called once by the Request Manager when constructing
	// a job chain. Construction of the job chain fails if an error is returned.
	Create(jobArgs map[string]string) error

	// Serialize returns all internal data needed to run later. The reciprocal
	// method is Deserialize.
	//
	// This method is only called once by the Request Manager when constructing
	// a job chain. Construction of the job chain fails if an error is returned.
	Serialize() ([]byte, error)

	// Deserialize sets internal data needed to run later. The reciprocal
	// method is Serialize.
	//
	// This method is only called once by the Job Runner when reconstructing
	// a job chain. Reconstruction of the job chain fails if an error is
	// returned.
	Deserialize([]byte) error

	// Run runs the job using its interal data and the run-time jobData from
	// previously-ran (upstream) jobs. Run can modify jobData. Run is expected
	// to block, but the job must respond to Stop and Status while running.
	// The returned error, if any, indicates a problem before or after running
	// the job. The job error and exit code, if any, is returned in the Return
	// structure.
	//
	// Currently, the Job Runner only calls this method once. Resuming a job is
	// not currently supported.
	Run(jobData map[string]string) (Return, error)

	// Stop stops a job. The Job Runner calls this method when stopping a job
	// chain before it has completed. The job must respond to Stop while Run
	// is executing. Stop is expected to block but also return quickly.
	Stop() error

	// Status returns the real-time status of the job. The job must respond
	// to Status while Run is executing. Status is expected to block but also
	// return quickly.
	Status() string

	// Name returns the name of the job.
	Name() string

	// Type returns the type of the job.
	Type() string
}

// A Factory instantiates a Job of the given type. A factory only instantiates
// a new Job object, it must not call any Job interface methods on the newly
// create job. If an error is returned, the returned Job should be ignored.
//
// Spin Cycle does not and should not know how to instantiate jobs because they
// are external (imported in external/jobs.go). Once a job is instantiated with
// external, implementation-specific details, Spin Cycle only needs to know and
// use the Job interface methods.
type Factory interface {
	Make(jobType, jobName string) (Job, error)
}

// Return represents return values and output from a job. State and Exit
// indicate how the job completed. Success is STATE_COMPLETE and exit 0.
// If the job completes with an error, the result is STATE_COMPLETE and a
// non-zero Exit. Any other State indicates the job did not complete. Error
// only indicates an internal Go error, not a job error. For example: an RPC
// client connection error to indicate the job was never attempted because
// the RPC server couldn't be reached.
type Return struct {
	State  byte   // proto/STATE_ const
	Exit   int64  // Unix exit code
	Error  error  // Go error
	Stdout string // stdout ouput
	Stderr string // stderr output
}

// A Repo stores jobs, abstracting away the actual storage method.
type Repo interface {
	Add(Job) error
	Remove(Job) error
	Get(jobName string) (Job, error)
}
