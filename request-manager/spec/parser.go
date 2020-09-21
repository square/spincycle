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

// Parse a single request (YAML) file.
func ParseSpec(specFile string) (Specs, *CheckResult) {
	spec := Specs{}
	result := &CheckResult{}

	sequenceData, err := ioutil.ReadFile(specFile)
	if err != nil {
		result.Errors = []error{err}
		return spec, result
	}

	// Emit warning if unexpected or duplicate fields are present.
	// Error if specs are incorrectly formatted or fields are of incorrect type.
	warn := yaml.UnmarshalStrict(sequenceData, &spec)
	if warn != nil {
		result.Warnings = []error{warn}
		err = yaml.Unmarshal(sequenceData, &spec)
		if err != nil {
			result.Errors = []error{err}
			return spec, result
		}
	}

	return spec, result
}

// Read all specs file in indicated specs directory.
// CheckResults are keyed on file name.
func ParseSpecsDir(specsDir string) (Specs, *CheckResults, error) {
	specs := Specs{
		Sequences: map[string]*Sequence{},
	}
	fileResults := NewCheckResults()

	seqFile := map[string]string{} // sequence name --> file it was first seen in
	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".yaml") {
			return nil
		}
		relPath, err := filepath.Rel(specsDir, path)
		if err != nil { // if we can't get the relative path, just use the full path
			relPath = path
		}

		spec, result := ParseSpec(path)
		fileResults.AddResult(relPath, result)
		if len(result.Errors) != 0 {
			return nil
		}

		// Set the file name of the sequences here. ParseSpec can't do it
		// because it only knows the absolute path.
		for _, seqSpec := range spec.Sequences {
			seqSpec.Filename = relPath
		}

		for name, spec := range spec.Sequences {
			if _, ok := seqFile[name]; ok {
				fileResults.AddError(relPath, fmt.Errorf("sequence %s already seen in file %s", name, seqFile[name]))
			} else {
				specs.Sequences[name] = spec
				seqFile[name] = relPath
			}
		}

		return nil
	})

	if err != nil {
		return specs, fileResults, fmt.Errorf("error traversing specs directory: %s", err)
	}

	return specs, fileResults, nil
}

// Specs require some processing after we've loaded them, but before we run the checker on them.
// Function modifies specs passed in.
func ProcessSpecs(specs *Specs) {
	for sequenceName, sequence := range specs.Sequences {
		sequence.Name = sequenceName

		for nodeName, node := range sequence.Nodes {
			node.Name = nodeName

			// Set various optional fields if they were excluded.
			for i, nodeSet := range node.Sets {
				if nodeSet != nil && nodeSet.As == nil {
					node.Sets[i].As = node.Sets[i].Arg
				}
			}
			for i, nodeArg := range node.Args {
				if nodeArg != nil && nodeArg.Given == nil {
					node.Args[i].Given = node.Args[i].Expected
				}
			}
			if node.Retry > 0 && node.RetryWait == "" {
				node.RetryWait = "0s"
			}
		}
	}
}
