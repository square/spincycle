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
	v "github.com/square/spincycle/v2/version"
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
	count      int    `arg:"-"` // warning + error counter
	anyWarning bool   `arg:"-"` // whether any warnings we would output with --warnings=true occurred
}

var splitter = "# ------------------------------------------------------------------------------"

func (linter *Linter) Version() string {
	return "linter " + v.Version()
}

func Run() bool {
	// 1. Setup
	linter := Linter{
		Strict:   false,
		Warnings: true,
		Color:    true,
		SpecsDir: "./",

		count:      1,
		anyWarning: false,
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
			header := splitter + "\n" + fmt.Sprintf("# File: %s\n", file)
			linter.printCheckResult(header, result)
		}
		return false
	}
	// Check that we did indeed read some specs.
	if len(allSpecs.Sequences) == 0 {
		fmt.Fprintf(os.Stderr, "%s\n", color.Red("No specs found in directory"))
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
	if fileResults.AnyWarning {
		linter.anyWarning = true

		if linter.Warnings {
			warnings := []error{}
			// There should only be one warning per file right now
			for file, result := range fileResults.Results {
				for _, warn := range result.Warnings {
					warnings = append(warnings, fmt.Errorf("%s: %s", file, warn))
				}
			}

			fmt.Println(splitter)
			fmt.Print(linter.fmtList("# Syntax warnings\n", warnings))
		}
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
		errorPrinted := false // whether we printed anything
		for _, seq := range sequences {
			header, err := fmtHeader(seq, allSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			errorPrinted = linter.printCheckResult(header, seqResults.Results[seq]) || errorPrinted
		}
		if !errorPrinted {
			fmt.Printf("%s\n", color.Red("Sequence not listed in --sequences failed static check; fix errors to perform graph checks"))
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
		errorPrinted := false // whether we printed anything
		for _, seq := range sequences {
			header, err := fmtHeader(seq, allSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			errorPrinted = linter.printCheckResult(header, seqResults.Results[seq]) || errorPrinted
		}
		if errorPrinted {
			return false
		}
	}

	// 5. No errors occurred. Print warnings and decide whether we need to
	// print anything else, then exit.
	if seqResults.AnyWarning {
		// No errors occurred; print all warnings now.
		for _, seq := range sequences {
			header, err := fmtHeader(seq, allSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err)
			}
			linter.printCheckResult(header, seqResults.Results[seq])
		}
	}

	if linter.anyWarning && !linter.Warnings {
		// Case: warnings occurred, but were surpressed
		fmt.Println(color.Yellow("Warnings occurred and suppressed"))
	} else if !linter.anyWarning {
		// Case: no warnings and no errors
		fmt.Println(color.Green("OK, all specs are valid"))
	} // else Case: some warnings printed, no errors

	if linter.Strict {
		return !linter.anyWarning
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

func (linter *Linter) fmtList(header string, list []error) string {
	if len(list) == 0 {
		return ""
	}
	str := fmt.Sprintf("%s\n", header)
	for _, elt := range list {
		str += fmt.Sprintf("%4d: %s\n\n", linter.count, elt)
		linter.count++
	}
	return str
}

func (linter *Linter) printCheckResult(header string, result *spec.CheckResult) (didPrint bool) {
	if result == nil {
		return false
	}
	toPrint := linter.fmtList(linter.errorStr, result.Errors)
	if len(result.Warnings) != 0 {
		linter.anyWarning = true
	}
	if linter.Warnings {
		toPrint += linter.fmtList(linter.warningStr, result.Warnings)
	}
	if len(toPrint) == 0 {
		return false
	}
	fmt.Println(header)
	fmt.Print(toPrint)
	return true
}
