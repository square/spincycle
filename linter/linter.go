// Copyright 2020, Square, Inc.

package linter

import (
	"fmt"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/logrusorgru/aurora"

	"github.com/square/spincycle/v2/linter/app"
	"github.com/square/spincycle/v2/request-manager/graph"
	"github.com/square/spincycle/v2/request-manager/spec"
)

type Linter struct {
	SpecsDir string `arg:"positional,required" help:"path to spin cycle requests directory"`

	Strict bool `arg:"-s, --strict" help:"make all warnings into errors"`

	Sequences string `help:"comma-separated list of sequences for which to output info; sequence must exist in specs"`
	Warnings  string `help:"turn warnings on or off (on | off)"`
	Color     string `help:"turn color on or off (on | off)"`
}

var splitter = "# ------------------------------------------------------------------------------"

func Run(ctx app.Context) bool {
	// 1. Setup
	cmd := Linter{
		Warnings: "on",
		Color:    "on",
	}
	arg.MustParse(&cmd)

	var sequences []string
	if len(cmd.Sequences) != 0 {
		sequences = strings.Split(cmd.Sequences, ",")
	}
	// else we want to print all sequences, but we don't know what they are
	// yet, so we can't fill out `sequences` properly yet

	var printWarnings bool
	switch strings.ToLower(cmd.Warnings) {
	case "on":
		printWarnings = true
	case "off":
		printWarnings = false
	default:
		fmt.Printf("Invalid args %s to --warnings; expected 'on' or 'off'\n", cmd.Warnings)
	}

	var useColor bool
	switch strings.ToLower(cmd.Color) {
	case "on":
		useColor = true
	case "off":
		useColor = false
	default:
		fmt.Printf("Invalid arg %s to --color; expected 'on' or 'off'\n", cmd.Color)
	}
	color := aurora.NewAurora(useColor)

	errorStr := fmt.Sprintf("%s", color.Red("Errors"))
	warningStr := fmt.Sprintf("%s", color.Yellow("Warnings"))

	// 2. Parsing and static parse checks
	allSpecs, parseErrors, parseWarnings, err := ctx.Hooks.LoadSpecs(cmd.SpecsDir)
	if err != nil {
		fmt.Println(err)
		return false
	}
	if len(parseErrors) > 0 {
		fileSeen := map[string]bool{}
		for file, errs := range parseErrors {
			fileSeen[file] = true

			fmt.Println(splitter)
			fmt.Printf("# File: %s\n", file)
			fmt.Print(fmtList(errorStr, errs))

			if printWarnings {
				fmt.Print(fmtList(warningStr, parseWarnings[file]))
			}
		}

		if printWarnings {
			for file, warns := range parseWarnings {
				if fileSeen[file] {
					continue
				}
				fmt.Println(splitter)
				fmt.Printf("# File: %s\n", file)
				fmt.Print(fmtList(warningStr, warns))
			}
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
			fmt.Printf("Args to --sequences not found in specs: %s\n", strings.Join(missing, ", "))
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
	if printWarnings && len(parseWarnings) > 0 {
		warnings := []error{}
		// There should only be one warning per file right now
		for file, warns := range parseWarnings {
			for _, warn := range warns {
				warnings = append(warnings, fmt.Errorf("%s: %s", file, warn))
			}
		}

		fmt.Println(splitter)
		fmt.Print(fmtList("# Parse warnings", warnings))
	}
	spec.ProcessSpecs(&allSpecs)

	// 3. Static checks
	checkFactories, err := ctx.Factories.MakeCheckFactories(allSpecs)
	if err != nil {
		fmt.Sprintf("MakeCheckFactories: %s", err)
		return false
	}
	checkFactories = append(checkFactories, spec.BaseCheckFactory{allSpecs})
	checker, err := spec.NewChecker(checkFactories)
	if err != nil {
		fmt.Println(err)
		return false
	}
	staticErrors, staticWarnings := checker.RunChecks(allSpecs)
	if len(staticErrors) != 0 {
		for _, seq := range sequences {
			toPrint := fmtList(errorStr, staticErrors[seq]) + fmtList(warningStr, staticWarnings[seq])
			if len(toPrint) != 0 {
				printHeader(seq, allSpecs)
				fmt.Print(toPrint)
			}
		}
		return false
	}
	// We'll print warnings later along with graph errors, if any.

	// 4. Graph checks
	idgen, err := ctx.Factories.MakeIDGeneratorFactory()
	if err != nil {
		fmt.Printf("MakeIDGeneratorFactory: %s", err)
		return false
	}
	gr := graph.NewGrapher(allSpecs, idgen)
	_, graphErrors := gr.CheckSequences()
	if len(graphErrors) != 0 {
		for _, seq := range sequences {
			// There are no graph warnings, but if there are errors,
			// we also want to print any static warnings from before.
			toPrint := fmtList(errorStr, graphErrors[seq]) + fmtList(warningStr, staticWarnings[seq])
			if len(toPrint) != 0 {
				printHeader(seq, allSpecs)
				fmt.Print(toPrint)
			}
		}
		return false
	}

	// 5. No errors occurred: print warnings, then exit.
	if printWarnings && len(staticWarnings) != 0 {
		for _, seq := range sequences {
			toPrint := fmtList(warningStr, staticWarnings[seq])
			if len(toPrint) != 0 {
				printHeader(seq, allSpecs)
				fmt.Print(toPrint)
			}
		}
	} else {
		fmt.Println(color.Green("OK, all specs are valid"))
	}

	if cmd.Strict {
		return len(parseWarnings) == 0 && len(staticWarnings) == 0
	}
	return true
}

func printHeader(seqName string, allSpecs spec.Specs) error {
	seqSpec, ok := allSpecs.Sequences[seqName]
	if !ok {
		return fmt.Errorf("linter CLI: could not find specs for sequence %s", seqName)
	}

	fmt.Println(splitter)
	fmt.Printf("# File: %s\n", seqSpec.Filename)
	fmt.Printf("#  Seq: %s\n", seqName)

	return nil
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
