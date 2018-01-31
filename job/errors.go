// Copyright 2017-2018, Square, Inc.

package job

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrUnknownJobType is returned by a job.Factory when the factory cannot
	// make the requested job type.
	ErrUnknownJobType = errors.New("unknown job type")

	// ErrRunTimeout should be returned by a job when it times out.
	ErrRunTimeout = errors.New("run timeout")

	// ErrWrongJobId is returned when the external job factory (EJF) creates a
	// job that does not return the same Id values. This is a bug in the EJF,
	// not Spin Cycle.
	ErrWrongJobId = errors.New("wrong job.Id")
)

// ErrArgNotSet should be returned by a job when a required key is not set in jobArgs.
// For example:
//
//   val, ok := jobArgs["srcHost"]
//   if !ok {
//     return job.ErrArgNotSet{"srcHost"}
//   }
//
type ErrArgNotSet struct {
	Arg string
}

func (e ErrArgNotSet) Error() string {
	return fmt.Sprintf("%s not set in job args", e.Arg)
}

// --------------------------------------------------------------------------

// ErrDataNotSet should be returned by a job when a required key is not set in jobData.
type ErrDataNotSet struct {
	Key string
}

func (e ErrDataNotSet) Error() string {
	return fmt.Sprintf("%s not set in job data", e.Key)
}

// --------------------------------------------------------------------------

// ErrWrongDataType should be returned by a job when a jobData value is not
// the expected type. For example:
//
//   type Cluster struct { ... }
//   v := jobData["cluster"]
//   cluster, ok := v.(Cluster)
//   if !ok {
//     return job.NewErrWrongDataType("cluster", v, Cluster{})
//   }
//
type ErrWrongDataType struct {
	Key        string
	GotType    reflect.Type
	ExpectType reflect.Type
}

func NewErrWrongDataType(key string, got, expect interface{}) ErrWrongDataType {
	return ErrWrongDataType{
		Key:        key,
		GotType:    reflect.TypeOf(got),
		ExpectType: reflect.TypeOf(expect),
	}
}

func (e ErrWrongDataType) Error() string {
	return fmt.Sprintf("%s in job data is type %s, expected type %s", e.Key, e.GotType, e.ExpectType)
}

// --------------------------------------------------------------------------

// ErrWrongArgType should be returned by a job when a jobArgs value is not
// the expected type. For example:
//
//   v := jobArgs["sliceOfStrings"]
//   sliceOfStrings, ok := v.([]string])
//   if !ok {
//     return job.NewErrWrongArgType("sliceOfStrings", v, []string{})
//   }
//
type ErrWrongArgType struct {
	Key        string
	GotType    reflect.Type
	ExpectType reflect.Type
}

func NewErrWrongArgType(key string, got, expect interface{}) ErrWrongArgType {
	return ErrWrongArgType{
		Key:        key,
		GotType:    reflect.TypeOf(got),
		ExpectType: reflect.TypeOf(expect),
	}
}

func (e ErrWrongArgType) Error() string {
	return fmt.Sprintf("%s in job args is type %s, expected type %s", e.Key, e.GotType, e.ExpectType)
}
