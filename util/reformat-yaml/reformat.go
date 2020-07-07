package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

func main() {
	/* Process arguments. */
	args := os.Args
	if len(args) != 3 {
		fmt.Printf("Usage: %s [input file path] [output file path]\n", args[0])
		os.Exit(0)
	}
	filename := args[1]
	ofilename := args[2]

	/* Set up IO. */

	f, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()

	of, err := os.Create(ofilename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer of.Close()

	/* Process files. */
	scanner := bufio.NewScanner(f)
	var indent string // the YAML indent currently being used
	var line uint     // line number
	var sets bool     // whether we're currently within a "sets "block
	for scanner.Scan() {
		inp := scanner.Text()
		line++

		// Don't process comments
		var comment string
		if i := strings.IndexRune(inp, '#'); i != -1 {
			// Preserve at least one unit of preceeding whitespace if it exists
			if i != 0 && unicode.IsSpace([]rune(inp)[i-1]) {
				i--
			}
			comment = inp[i:]
			inp = inp[:i]
		}

		// Case: first line of `sets` block
		if i := strings.Index(inp, "sets:"); i != -1 {
			if i%4 != 0 {
				fmt.Printf("%s line %d: Expected four (equal) indents on line\n", filename, line)
				os.Exit(1)
			}
			indent = inp[:i] + inp[:i/4]

			// Case: `sets: [...]` notation
			if j := strings.Index(inp, "["); j != -1 {
				// Assume this indicates the end of the list
				k := strings.LastIndex(inp, "]")
				if k == -1 {
					fmt.Printf("%s line %d: Found [ but no matching ]", filename, line)
					os.Exit(1)
				}

				// Case: non-empty list
				if j+1 < k {
					args := strings.Split(inp[j+1:k], ",")
					inp = inp[:i+5] + comment
					for _, arg := range args {
						arg = strings.TrimSpace(arg)
						inp += fmt.Sprintf("\n%s- arg: %s", indent, arg)
					}
					// Case: empty list
				} else {
					inp += comment
				}
				// Case: `sets:\n -` notation
			} else {
				sets = true
				inp += comment
			}
			// Case: currently in a `sets` block
		} else if sets {
			// Case: still in a `sets` block
			if strings.Contains(inp, indent+"-") {
				i := strings.IndexRune(inp, '-')
				arg := strings.TrimSpace(inp[i+1:])
				if strings.Index(arg, "arg:") == 0 {
					fmt.Printf("%s line %d: already in sets/as format\n", filename, line)
					inp += comment
				} else {
					inp = fmt.Sprintf("%s- arg: %s%s", indent, arg, comment)
				}
				// Case: exit `sets` block
			} else {
				sets = false
				inp += comment
			}
			// Case: not a `sets` block
		} else {
			inp += comment
		}

		of.WriteString(inp + "\n")
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
