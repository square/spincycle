// Copyright 2017-2018, Square, Inc.

package main

import (
	"log"

	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/server"
)

func main() {
	err := server.Run(app.Defaults())
	log.Fatal("Request Manager stopped: %s", err)
}
