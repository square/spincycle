// Copyright 2017, Square, Inc.

// Package client provides an HTTP client for interacting with the Job Runner (JR) API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/square/spincycle/proto"
)

// A JRClient is an HTTP client used for interacting with the JR API.
type JRClient interface {
	// NewJobChain takes a job chain and sends it to the JR.
	NewJobChain(proto.JobChain) error
	// StartRequest starts the job chain that corresponds to a given request Id.
	StartRequest(string) error
	// StopRequest stops the job chain that corresponds to a given request Id.
	StopRequest(string) error
	// RequestStatus gets the status of the job chain that corresponds to a given request Id.
	RequestStatus(string) (*proto.JobChainStatus, error)
}

type jrClient struct {
	*http.Client
	baseUrl string
}

// NewJRClient takes an http.Client and base API URL and creates a JRClient.
func NewJRClient(c *http.Client, baseUrl string) JRClient {
	return &jrClient{
		Client:  c,
		baseUrl: baseUrl,
	}
}

func (c *jrClient) NewJobChain(jobChain proto.JobChain) error {
	// POST /api/v1/job-chains
	url := c.baseUrl + "/api/v1/job-chains"

	// Create the payload.
	payload, err := json.Marshal(jobChain)
	if err != nil {
		return err
	}

	// Make the request.
	resp, body, err := c.post(url, payload)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	return nil
}

func (c *jrClient) StartRequest(requestId string) error {
	// PUT /api/v1/job-chains/${requestId}/start
	url := fmt.Sprintf(c.baseUrl+"/api/v1/job-chains/%s/start", requestId)

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

func (c *jrClient) StopRequest(requestId string) error {
	// PUT /api/v1/job-chains/${requestId}/stop
	url := fmt.Sprintf(c.baseUrl+"/api/v1/job-chains/%s/stop", requestId)

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

func (c *jrClient) RequestStatus(requestId string) (*proto.JobChainStatus, error) {
	// GET /api/v1/job-chains/${requestId}/status
	url := fmt.Sprintf(c.baseUrl+"/api/v1/job-chains/%s/status", requestId)

	// Make the request.
	resp, body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsuccessful status code: %d (response body: %s)",
			resp.StatusCode, string(body))
	}

	// Unmarshal the response.
	status := &proto.JobChainStatus{}
	err = json.Unmarshal(body, status)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// ------------------------------------------------------------------------- //

func (c *jrClient) get(url string) (*http.Response, []byte, error) {
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

func (c *jrClient) put(url string) (*http.Response, []byte, error) {
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

func (c *jrClient) post(url string, payload []byte) (*http.Response, []byte, error) {
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

func (c *jrClient) do(req *http.Request) (*http.Response, []byte, error) {
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
