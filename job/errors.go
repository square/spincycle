package job

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrUnknownJobType = errors.New("unknown job type")
	ErrJobNotFound    = errors.New("job not found")
	ErrNilJob         = errors.New("nil job")
	ErrConnectTimeout = errors.New("connect timeout")
	ErrRunTimeout     = errors.New("run timeout")
)

type ErrArgNotSet struct {
	Arg string
}

func (e ErrArgNotSet) Error() string {
	return fmt.Sprintf("%s not set in job args", e.Arg)
}

// --------------------------------------------------------------------------

type ErrDataNotSet struct {
	Key string
}

func (e ErrDataNotSet) Error() string {
	return fmt.Sprintf("%s not set in job data", e.Key)
}

// --------------------------------------------------------------------------

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
	return fmt.Sprintf("%s in job data is type %s, expected type %s", e.GotType, e.ExpectType)
}
