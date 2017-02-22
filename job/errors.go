package job

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownJobType = errors.New("unknown job type")
	ErrJobNotFound    = errors.New("job not found")
	ErrNilJob         = errors.New("nil job")
)

type ErrArgNotSet struct {
	Arg string
}

func (e ErrArgNotSet) Error() string {
	return fmt.Sprintf("%s not set in job args", e.Arg)
}
