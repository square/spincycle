// Copyright 2017, Square, Inc.

// Prompt provides user input handling.
package prompt

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrNack = errors.New("negative ack")
)

var InputBufferSize int = 1000

type Prompter interface {
	Prompt() error
}

type Item struct {
	Name      string
	Desc      string
	Required  bool
	Skip      bool
	IsDefault bool
	Default   string
	Value     string
	Valid     []string
	Validator func(string) error
}

type GuidedPrompt struct {
	items []Item
	in    io.Reader
	out   io.Writer
}

func NewGuidedPrompt(items []Item, in io.Reader, out io.Writer) *GuidedPrompt {
	p := &GuidedPrompt{
		items: items,
		in:    in,
		out:   out,
	}
	return p
}

func (p *GuidedPrompt) Prompt() error {
	bytes := make([]byte, InputBufferSize)
	for n := range p.items {
		i := &p.items[n]

		if i.Skip {
			continue
		}

		fmt.Fprintln(p.out)

		desc := "# " + i.Desc
		if len(i.Valid) > 0 {
			desc += " (valid: " + strings.Join(i.Valid, ", ") + ")"
		}
		if i.Default != "" {
			desc += " (default: " + i.Default + ")"
		}
		fmt.Fprintln(p.out, desc)

	ITEM_LOOP:
		for {
			fmt.Fprintf(p.out, "%s=", i.Name)
			nbytes, err := p.in.Read(bytes)
			if err != nil {
				fmt.Println(err)
			}
			val := strings.TrimSpace(string(bytes[0:nbytes]))
			if len(val) > 0 {
				if i.Validator != nil {
					if err := i.Validator(val); err != nil {
						fmt.Fprintf(p.out, "# Invalid value: %s\n", err)
						continue ITEM_LOOP
					}
				}
				if len(i.Valid) > 0 {
					valid := false
					for _, v := range i.Valid {
						if val != v {
							continue
						}
						valid = true
						break
					}
					if !valid {
						fmt.Fprintf(p.out, "# Invalid value: %s\n", val)
						continue ITEM_LOOP
					}
				}
				i.Value = val // ok
			} else {
				// No value entered
				if i.Required {
					fmt.Fprintf(p.out, "# Value required for %s\n", i.Name)
					continue ITEM_LOOP
				} else {
					i.IsDefault = true
					if i.Default != "" {
						i.Value = i.Default // ok
						fmt.Fprintf(p.out, "%s=%s\n", i.Name, i.Value)
					}
				}
			}
			break ITEM_LOOP // ok, item done
		}
	}
	return nil
}

type ConfirmationPrompt struct {
	prompt string
	word   string
	in     io.Reader
	out    io.Writer
}

func NewConfirmationPrompt(prompt, word string, in io.Reader, out io.Writer) *ConfirmationPrompt {
	p := &ConfirmationPrompt{
		prompt: prompt,
		word:   word,
		in:     in,
		out:    out,
	}
	return p
}

func (p *ConfirmationPrompt) Prompt() error {
	fmt.Fprintf(p.out, p.prompt)
	bytes := make([]byte, InputBufferSize)
	nbytes, err := p.in.Read(bytes)
	if err != nil {
		fmt.Println(err)
	}
	val := strings.TrimSpace(string(bytes[0:nbytes]))
	if val != p.word {
		return ErrNack
	}
	return nil
}
