// Copyright 2017, Square, Inc.

// Package client provides an HTTP client for interacting with the Job Runner (JR) API.
package jr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/square/spincycle/proto"
)

// A Client is an HTTP client used for interacting with the JR API.
type Client interface {
	// NewJobChain takes a job chain, and sends it to the JR to be run immediately.
	// It returns the URL of the running job chain.
	NewJobChain(baseURL string, jobChain proto.JobChain) (*url.URL, error)
	// ResumeJobChain takes a suspended job chain and sends it to the JR to be
	// resumed. It returns the URL of the running job chain.
	ResumeJobChain(baseURL string, sjc proto.SuspendedJobChain) (*url.URL, error)
	// StopRequest stops the job chain that corresponds to a given request Id. The
	// baseURL should point to the Job Runner running this request.
	StopRequest(baseURL string, requestId string) error
	// RequestStatus gets the status of the job chain that corresponds to a given
	// request Id. The baseURL should point to the Job Runner running this request.
	RequestStatus(baseURL string, requestId string) (proto.JobChainStatus, error)

	// SysStatRunning reports all running jobs.
	SysStatRunning(baseURL string) ([]proto.JobStatus, error)
}

type client struct {
	*http.Client
}

// NewClient takes an http.Client and base API URL and creates a Client.
func NewClient(c *http.Client) Client {
	return &client{
		Client: c,
	}
}

func (c *client) NewJobChain(baseURL string, jobChain proto.JobChain) (*url.URL, error) {
	var chainURL *url.URL

	// POST /api/v1/job-chains
	url := baseURL + "/api/v1/job-chains"

	// Create the payload.
	payload, err := json.Marshal(jobChain)
	if err != nil {
		return chainURL, err
	}

	// Make the request.
	resp, body, err := c.post(url, payload)
	if err != nil {
		return chainURL, err
	}

	if resp.StatusCode != http.StatusOK {
		return chainURL, fmt.Errorf("jr.Client.NewJobChain - unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	// Retrieve the URL of the JR host that's running the job chain.
	chainURL, err = resp.Location()
	if err != nil {
		return chainURL, err
	}

	return chainURL, nil
}

func (c *client) ResumeJobChain(baseURL string, sjc proto.SuspendedJobChain) (*url.URL, error) {
	var chainURL *url.URL

	// POST /api/v1/job-chains/resume
	url := baseURL + "/api/v1/job-chains/resume"

	// Create the payload.
	payload, err := json.Marshal(sjc)
	if err != nil {
		return chainURL, err
	}

	// Make the request.
	resp, body, err := c.post(url, payload)
	if err != nil {
		return chainURL, err
	}

	if resp.StatusCode != http.StatusOK {
		return chainURL, fmt.Errorf("jr.Client.ResumeJobChain - unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	// Retrieve the URL of the JR host that's running the job chain.
	chainURL, err = resp.Location()
	if err != nil {
		return chainURL, err
	}

	return chainURL, nil
}

func (c *client) StopRequest(baseURL string, requestId string) error {
	// PUT /api/v1/job-chains/${requestId}/stop
	url := fmt.Sprintf(baseURL+"/api/v1/job-chains/%s/stop", requestId)

	// Make the request.
	resp, body, err := c.put(url)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}
	return nil
}

func (c *client) RequestStatus(baseURL string, requestId string) (proto.JobChainStatus, error) {
	// GET /api/v1/job-chains/${requestId}/status
	url := fmt.Sprintf(baseURL+"/api/v1/job-chains/%s/status", requestId)

	// Make the request.
	status := proto.JobChainStatus{}
	resp, body, err := c.get(url)
	if err != nil {
		return status, err
	}

	if resp.StatusCode != http.StatusOK {
		return status, fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	// Unmarshal the response.
	err = json.Unmarshal(body, &status)
	if err != nil {
		return status, err
	}

	return status, nil
}

func (c *client) SysStatRunning(baseURL string) ([]proto.JobStatus, error) {
	// GET /api/v1/job-chains/${requestId}/status
	url := baseURL + "/api/v1/status/running"
	resp, body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	var running []proto.JobStatus
	if err := json.Unmarshal(body, &running); err != nil {
		return nil, err
	}

	return running, nil
}

// ------------------------------------------------------------------------- //

func (c *client) get(url string) (*http.Response, []byte, error) {
	// Create the request.
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}

func (c *client) put(url string) (*http.Response, []byte, error) {
	// Create the request.
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return nil, nil, err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}

func (c *client) post(url string, payload []byte) (*http.Response, []byte, error) {
	// Create the request.
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, nil, err
	}

	// Send the request.
	resp, body, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}

func (c *client) do(req *http.Request) (*http.Response, []byte, error) {
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http.Client.Do: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("ioutil.ReadAll: %s", err)
	}

	return resp, body, nil
}
