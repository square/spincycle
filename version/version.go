// Copyright 2017-2019, Square, Inc.

// Package version provides the Spin Cycle version.
package version

const VERSION = "2.0.4"

// BUILD is appended to VERSION if set: "VERSION+BUILD". The "+" is included automatically.
var BUILD string = ""

// Version returns the semver-compatible (https://semver.org/) version string.
func Version() string {
	v := VERSION // 1.0.0
	if BUILD != "" {
		v += "+" + BUILD // 1.0.0+sq1
	}
	return v
}
