// Copyright 2017-2018, Square, Inc.

// Package test provides helper functions for tests.
package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

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
	return InitJobsWithSequenceRetry(count, 0)
}

// InitJobs initializes some proto jobs.
func InitJobsWithSequenceRetry(jobCount int, sequenceRetryCount uint) map[string]proto.Job {
	jobs := make(map[string]proto.Job)
	for i := 1; i <= jobCount; i++ {
		bytes := make([]byte, 10)
		rand.Read(bytes)
		job := proto.Job{
			Id:    fmt.Sprintf("job%d", i),
			Bytes: bytes,
			State: proto.STATE_PENDING,
		}

		// Set sequence data
		job.SequenceId = "job1"
		if i == 1 {
			job.SequenceRetry = sequenceRetryCount
		} else {
			job.SequenceRetry = 0
		}

		jobs[job.Id] = job
	}
	return jobs
}

func Dump(v interface{}) {
	bytes, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(bytes))
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandSeq(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
