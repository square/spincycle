package retry

import (
	"time"
)

type TryFunc func() error
type LogFunc func(error)

// Retry a function repeatedly and log its errors.
// https://upgear.io/blog/simple-golang-retry-function/
func Do(tries int, sleep time.Duration, tryFunc TryFunc, logFunc LogFunc) error {
	if err := tryFunc(); err != nil {
		if tries--; tries > 0 {
			if logFunc != nil {
				logFunc(err)
			}
			time.Sleep(sleep)
			return Do(tries, sleep, tryFunc, logFunc)
		}
		return err
	}
	return nil
}
