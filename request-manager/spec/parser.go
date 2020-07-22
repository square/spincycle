// Copyright 2017-2020, Square, Inc.

package spec

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

/* Parse a single request (YAML) file. */
func ParseSpec(specFile string) (spec Specs, err, warning error) {
	sequenceData, err := ioutil.ReadFile(specFile)
	if err != nil {
		return
	}

	/* Emit warning if unexpected or duplicate fields are present. */
	/* Error if specs are incorrectly formatted or fields are of incorrect type. */
	err = yaml.UnmarshalStrict(sequenceData, &spec)
	if err != nil {
		warning = err
		err = yaml.Unmarshal(sequenceData, &spec)
		if err != nil {
			return
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

	return
}

/* Runs checks on allSpecs. */
func RunChecks(allSpecs Specs) (errors, warnings []error) {
	sequenceErrors := makeSequenceErrorChecks()
	nodeErrors := makeNodeErrorChecks(allSpecs)
	nodeWarnings := makeNodeWarningChecks()

	for _, sequence := range allSpecs.Sequences {
		for _, sequenceCheck := range sequenceErrors {
			if err := sequenceCheck.CheckSequence(*sequence); err != nil {
				errors = append(errors, err)
			}
		}

		for _, node := range sequence.Nodes {
			for _, nodeCheck := range nodeErrors {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					errors = append(errors, err)
				}
			}
			for _, nodeCheck := range nodeWarnings {
				if err := nodeCheck.CheckNode(sequence.Name, *node); err != nil {
					warnings = append(warnings, err)
				}
			}
		}
	}

	return
}

/* Read and check all specs file in indicated specs directory. */
func Parse(specsDir string, printf func(string, ...interface{})) (Specs, error) {
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

		spec, err, warning := ParseSpec(path)
		if err != nil {
			return fmt.Errorf("error reading spec file %s: %s", info.Name(), err)
		}
		if warning != nil {
			printf("Warning: %s: %s\n", info.Name(), warning.Error())
		}
		for name, spec := range spec.Sequences {
			specs.Sequences[name] = spec
		}

		return nil
	})
	if err != nil {
		return specs, err
	}

	errors, warnings := RunChecks(specs)
	if len(errors) > 0 {
		for _, err = range errors {
			printf("Error: %s\n", err.Error())
		}
		return specs, fmt.Errorf("specs file(s) failed static check(s), run spinc-linter on specs or see log for more details")
	}
	for _, warning := range warnings {
		printf("Warning: %s\n", warning.Error())
	}

	return specs, nil
}
