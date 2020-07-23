// Copyright 2020, Square, Inc.

package spec

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var checkFailed = fmt.Errorf("Static check(s) failed")

// Parse a single request (YAML) file.
// `logFunc` is a Printf-like function used to log warning(s) should they occur.
// Errors are returned, not logged.
func ParseSpec(specFile string, logFunc func(string, ...interface{})) (Specs, error) {
	var spec Specs

	sequenceData, err := ioutil.ReadFile(specFile)
	if err != nil {
		return spec, err
	}

	/* Emit warning if unexpected or duplicate fields are present. */
	/* Error if specs are incorrectly formatted or fields are of incorrect type. */
	err = yaml.UnmarshalStrict(sequenceData, &spec)
	if err != nil {
		logFunc("Warning: %s\n", err)
		err = yaml.Unmarshal(sequenceData, &spec)
		if err != nil {
			return spec, err
		}
	}

	for sequenceName, sequence := range spec.Sequences {
		sequence.Name = sequenceName

		for nodeName, node := range sequence.Nodes {
			node.Name = nodeName

			// Set various optional fields if they were excluded.
			for i, nodeSet := range node.Sets {
				if nodeSet.As == nil {
					node.Sets[i].As = node.Sets[i].Arg
				}
			}
			for i, nodeArg := range node.Args {
				if nodeArg.Given == nil {
					node.Args[i].Given = node.Args[i].Expected
				}
			}
			if node.Retry > 0 && node.RetryWait == "" {
				node.RetryWait = "0s"
			}
		}

	}

	return spec, nil
}

// Read all specs file in indicated specs directory.
// `logFunc` is a Printf-like function used to log warning(s) should they occur.
// Errors are returned, not logged.
func Parse(specsDir string, logFunc func(string, ...interface{})) (Specs, error) {
	specs := Specs{
		Sequences: map[string]*SequenceSpec{},
	}

	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}
		relPath, err := filepath.Rel(specsDir, path)
		if err != nil {
			logFunc("Warning: failed to get relative directory path for file %s: %s", path, err)
		}

		spec, err := ParseSpec(path, logFunc) // logs warnings but not errors
		if err != nil {
			return fmt.Errorf("error reading spec file %s: %s", relPath, err)
		}

		for name, spec := range spec.Sequences {
			specs.Sequences[name] = spec
		}

		return nil
	})
	if err != nil {
		return specs, fmt.Errorf("error reading spec files: %s", err)
	}

	return specs, nil
}

// Runs checks on allSpecs.
// `logFunc` is a Printf-like function used to log warnings and errors should they occur.
// If any error is logged, this function returns an error.
func RunChecks(allSpecs Specs, logFunc func(string, ...interface{})) error {
	sequenceErrors := makeSequenceErrorChecks()
	nodeErrors := makeNodeErrorChecks(allSpecs)
	nodeWarnings := makeNodeWarningChecks()

	var ret error

	for _, sequence := range allSpecs.Sequences {
		for _, sequenceCheck := range sequenceErrors {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				logFunc("Error: %s\n", err)
				ret = checkFailed
			}
		}

		for _, node := range sequence.Nodes {
			for _, nodeCheck := range nodeErrors {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					logFunc("Error: %s\n", err)
					ret = checkFailed
				}
			}
			for _, nodeCheck := range nodeWarnings {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					logFunc("Warning: %s\n", err)
				}
			}
		}
	}

	return ret
}
