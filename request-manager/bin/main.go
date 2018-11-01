// Copyright 2017-2018, Square, Inc.

package main

import (
	"log"

	"github.com/square/spincycle/request-manager/app"
	"github.com/square/spincycle/request-manager/server"
)

func main() {
	s := server.NewServer(app.Defaults())
	if err := s.Boot(); err != nil {
		log.Fatalf("Error starting Request Manager: %s", err)
	}
	err := s.Run()
	log.Fatalf("Request Manager stopped: %s", err)
}
