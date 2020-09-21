// Copyright 2020, Square, Inc.

package linter

import (
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/logrusorgru/aurora"

	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/id"
	"github.com/square/spincycle/v2/request-manager/spec"
)

// Note that go-arg help message will show defaults if the default is not false.
type Linter struct {
	SpecsDir string `arg:"positional" help:"path to spin cycle requests directory [default: current working dir]"`

	Strict   bool `arg:"-s, --strict" help:"make all warnings into errors [default: false]"`
	Warnings bool `help:"turn warnings on or off"`
	Color    bool `help:"turn color on or off"`

	Sequences string `help:"comma-separated list of sequences for which to output info; sequence must exist in specs [default: all]"`

	errorStr   string `arg:"-"`
	warningStr string `arg:"-"`
}

var splitter = "# ------------------------------------------------------------------------------"

func Run() bool {
	// 1. Setup
	linter := Linter{
		Strict:   false,
		Warnings: true,
		Color:    true,
		SpecsDir: "./",
	}
	arg.MustParse(&linter)

	var sequences []string
	if len(linter.Sequences) != 0 {
		sequences = strings.Split(linter.Sequences, ",")
	}
	// else we want to print all sequences, but we don't know what they are
	// yet, so we can't fill out `sequences` properly yet

	color := aurora.NewAurora(linter.Color)
	linter.errorStr = fmt.Sprintf("%s", color.Red("Errors"))
	linter.warningStr = fmt.Sprintf("%s", color.Yellow("Warnings"))

	// 2. Parsing and static parse checks
	allSpecs, fileResults, err := spec.ParseSpecsDir(linter.SpecsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		return false
	}
	if fileResults.AnyError {
		for file, result := range fileResults.Results {
			header := splitter + fmt.Sprintf("# File: %s\n", file)
			linter.printCheckResult(header, result)
		}
		return false
	}
	// Check that all sequences provided by --sequences arg are actually in
	// the specs.
	for _, seq := range sequences {
		missing := []string{}
		if _, ok := allSpecs.Sequences[seq]; !ok {
			missing = append(missing, seq)
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "Args to --sequences not found in specs: %s\n", strings.Join(missing, ", "))
			return false
		}
	}
	// If no --sequences were specified, then we'll output all sequences. Update
	// `sequences` to reflect that.
	if len(sequences) == 0 {
		sequences = make([]string, 0, len(allSpecs.Sequences))
		for seq, _ := range allSpecs.Sequences {
			sequences = append(sequences, seq)
		}
	}
	// There may be other warnings/errors from later checks, but those will
	// be grouped by seqence. These can't be, so we'll just print them all
	// together now.
	if linter.Warnings && fileResults.AnyWarning {
		warnings := []error{}
		// There should only be one warning per file right now
		for file, result := range fileResults.Results {
			for _, warn := range result.Warnings {
				warnings = append(warnings, fmt.Errorf("%s: %s", file, warn))
			}
		}

		fmt.Println(splitter)
		fmt.Print(fmtList("# Parse warnings", warnings))
	}
	spec.ProcessSpecs(&allSpecs)

	// 3. Static checks
	checkFactories := []spec.CheckFactory{spec.DefaultCheckFactory{allSpecs}, spec.BaseCheckFactory{allSpecs}}
	checker, err := spec.NewChecker(checkFactories)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		return false
	}
	seqResults := checker.RunChecks(allSpecs)
	if seqResults.AnyError {
		for _, seq := range sequences {
			header, err := fmtHeader(seq, allSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			linter.printCheckResult(header, seqResults.Results[seq])
		}
		return false
	}
	// We'll print warnings later along with graph errors, if any.

	// 4. Graph checks
	idgen := id.NewGeneratorFactory(4, 100)
	gr := graph.NewGrapher(allSpecs, idgen)
	_, graphResults := gr.CheckSequences()
	seqResults.Union(graphResults)
	if seqResults.AnyError {
		for _, seq := range sequences {
			header, err := fmtHeader(seq, allSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			linter.printCheckResult(header, seqResults.Results[seq])
		}
		return false
	}

	// 5. No errors occurred: print warnings, then exit.
	if seqResults.AnyWarning {
		for _, seq := range sequences {
			header, err := fmtHeader(seq, allSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			linter.printCheckResult(header, seqResults.Results[seq])
		}
	} else {
		fmt.Println(color.Green("OK, all specs are valid"))
	}

	if linter.Strict {
		return !fileResults.AnyWarning && !seqResults.AnyWarning
	}
	return true
}

func fmtHeader(seqName string, allSpecs spec.Specs) (string, error) {
	seqSpec, ok := allSpecs.Sequences[seqName]
	if !ok {
		return "", fmt.Errorf("linter CLI: could not find specs for sequence %s", seqName)
	}

	return splitter + "\n" +
		fmt.Sprintf("# File: %s\n", seqSpec.Filename) +
		fmt.Sprintf("#  Seq: %s\n", seqName), nil
}

func fmtList(header string, list []error) string {
	if len(list) == 0 {
		return ""
	}
	str := fmt.Sprintf("%s\n", header)
	for i, elt := range list {
		str += fmt.Sprintf("%4d: %s\n\n", i+1, elt)
	}
	return str
}

func (ctx *Linter) printCheckResult(header string, result *spec.CheckResult) {
	if result == nil {
		return
	}
	toPrint := fmtList(ctx.errorStr, result.Errors)
	if ctx.Warnings {
		toPrint += fmtList(ctx.warningStr, result.Warnings)
	}
	if len(toPrint) != 0 {
		fmt.Println(header)
		fmt.Print(toPrint)
	}
}
