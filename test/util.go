// Copyright 2017, Square, Inc.

// Package test provides helper functions for tests.
package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/square/spincycle/proto"
)

// MakeHTTPRequest is a helper function for making an http request. The response
// body of the http request is unmarshalled into the struct pointed to by the
// respStruct argument (if it's not nil). The status code of the response and
// the response headers are returned.
func MakeHTTPRequest(httpVerb, url string, payload []byte, respStruct interface{}) (int, http.Header, error) {
	var statusCode int
	// Make the http request.
	req, err := http.NewRequest(httpVerb, url, bytes.NewReader(payload))
	if err != nil {
		return statusCode, http.Header{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := (http.DefaultClient).Do(req)
	if err != nil {
		return statusCode, http.Header{}, err
	}
	defer res.Body.Close()

	if respStruct != nil {
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(respStruct)
		if err != nil {
			return statusCode, res.Header, fmt.Errorf("error decoding response body")
		}
	}

	return res.StatusCode, res.Header, nil
}

// InitJobs initializes some proto jobs.
func InitJobs(count int) map[string]proto.Job {
	jobs := make(map[string]proto.Job)
	for i := 1; i <= count; i++ {
		bytes := make([]byte, 10)
		rand.Read(bytes)
		job := proto.Job{
			Id:    fmt.Sprintf("job%d", i),
			Bytes: bytes,
		}
		jobs[job.Id] = job
	}
	return jobs
}
