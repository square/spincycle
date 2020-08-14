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
// 'logFunc' is a Printf-like function used to log warning(s) should they occur.
// Errors are returned, not logged.
func ParseSpec(specFile string, logFunc func(string, ...interface{})) (Specs, error) {
	var spec Specs

	sequenceData, err := ioutil.ReadFile(specFile)
	if err != nil {
		return spec, err
	}

	/* Emit warning if unexpected or duplicate fields are present. */
	/* Error if specs are incorrectly formatted or fields are of incorrect type. */
	warn := yaml.UnmarshalStrict(sequenceData, &spec)
	if warn != nil {
		err = yaml.Unmarshal(sequenceData, &spec)
		if err != nil {
			return spec, err
		}
		logFunc("Warning: %s: %s", specFile, warn)
	}

	return spec, nil
}

// Read all specs file in indicated specs directory.
// 'logFunc' is a Printf-like function used to log warning(s) should they occur.
// Errors are returned, not logged.
func ParseSpecsDir(specsDir string, logFunc func(string, ...interface{})) (Specs, error) {
	specs := Specs{
		Sequences: map[string]*Sequence{},
	}

	failedFiles := []string{}
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

		spec, err := ParseSpec(path, logFunc) // logs warnings but not errors
		if err != nil {
			failedFiles = append(failedFiles, fmt.Sprintf("%s: %s", relPath, err))
			return nil
		}

		for name, spec := range spec.Sequences {
			if _, ok := seqFile[name]; ok {
				failedFiles = append(failedFiles, fmt.Sprintf("%s: sequence %s already seen in file %s", relPath, name, seqFile[name]))
			} else {
				specs.Sequences[name] = spec
				seqFile[name] = relPath
			}
		}

		return nil
	})
	if err != nil {
		return specs, fmt.Errorf("error traversing specs directory: %s", err)
	}
	if len(failedFiles) > 0 {
		multiple := ""
		if len(failedFiles) > 1 {
			multiple = "s"
		}
		return specs, fmt.Errorf("error in file%s:\n%s", multiple, strings.Join(failedFiles, "\n"))
	}

	return specs, nil
}

// Specs require some processing after we've loaded them, but before we run the checker on them.
// Function modifies specs passed in.
func ProcessSpecs(specs *Specs) error {
	for sequenceName, sequence := range specs.Sequences {
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

	return nil
}
