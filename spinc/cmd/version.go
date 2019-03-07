// Copyright 2019, Square, Inc.

package cmd

import (
	"fmt"

	"github.com/square/spincycle/spinc/app"
	v "github.com/square/spincycle/version"
)

type Version struct {
}

func NewVersion(ctx app.Context) *Version {
	return &Version{}
}

func (c *Version) Prepare() error {
	return nil
}

func (c *Version) Run() error {
	fmt.Println("spinc " + v.Version())
	return nil
}

func (c *Version) Cmd() string {
	return "version"
}

func (c *Version) Help() string {
	return "'spinc version' prints the Spin Cycle version.\n"
}
