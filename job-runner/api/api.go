// Copyright 2017, Square, Inc.

// Package api implements api route handling.
package api

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/square/spincycle/job-runner/cache"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/router"
)

const (
	API_ROOT           = "/api/v1/"
	REQUEST_ID_PATTERN = "([0-9]+)"
	CACHE_DB_PREFIX    = "API"
)

type API struct {
	Router        *router.Router
	chainRepo     chain.Repo
	runnerFactory runner.RunnerFactory
	cache         cache.Cacher // in-memory cache for storing traversers
}

var hostname func() (string, error) = os.Hostname

func NewAPI(router *router.Router, chainRepo chain.Repo, runnerFactory runner.RunnerFactory, cache cache.Cacher) *API {
	api := &API{
		Router:        router,
		chainRepo:     chainRepo,
		runnerFactory: runnerFactory,
		cache:         cache,
	}

	api.Router.AddRoute(API_ROOT+"job-chains", api.newJobChainHandler, "api-new-job-chain")
	api.Router.AddRoute(API_ROOT+"job-chains/"+REQUEST_ID_PATTERN+"/start", api.startJobChainHandler, "api-start-job-chain")
	api.Router.AddRoute(API_ROOT+"job-chains/"+REQUEST_ID_PATTERN+"/stop", api.stopJobChainHandler, "api-stop-job-chain")
	api.Router.AddRoute(API_ROOT+"job-chains/"+REQUEST_ID_PATTERN+"/status", api.statusJobChainHandler, "api-status-job-chain")

	return api
}

// ============================== CONTROLLERS ============================== //

// POST <API_ROOT>/job-chains
// Do some basic validation on a job chain, and, if it passes, add it to the
// chain repo. If it doesn't pass, return the validation error.
func (api *API) newJobChainHandler(ctx router.HTTPContext) {
	switch ctx.Request.Method {
	case "POST":
		decoder := json.NewDecoder(ctx.Request.Body)
		var jobChain proto.JobChain
		err := decoder.Decode(&jobChain)
		if err != nil {
			ctx.APIError(router.ErrInternal, "Can't decode request body (error: %s)", err)
		}

		c := chain.NewChain(&jobChain)
		// Make sure the chain passed some basic tests.
		err = c.Validate()
		if err != nil {
			ctx.APIError(router.ErrBadRequest, "Invalid chain (error: %s)", err)
		}

		// Save the chain to the repo.
		api.chainRepo.Set(c)
	default:
		ctx.UnsupportedAPIMethod()
	}
}

// PUT <API_ROOT>/job-chains/{requestId}/start
// Start the traverser for a job chain.
func (api *API) startJobChainHandler(ctx router.HTTPContext) {
	switch ctx.Request.Method {
	case "PUT":
		requestIdStr := ctx.Arguments[1]
		requestId, err := strconv.ParseUint(ctx.Arguments[1], 10, 0)
		if err != nil {
			ctx.APIError(router.ErrInvalidParam, "Can't parse requestId (error: %s)", err)
			return
		}

		// Get the chain from the repo.
		c, err := api.chainRepo.Get(uint(requestId))
		if err != nil {
			ctx.APIError(router.ErrNotFound, err.Error())
		}

		// Create a traverser and add it to the cache.
		traverser := chain.NewTraverser(api.chainRepo, api.runnerFactory, c, api.cache)
		err = api.cache.Add(api.cacheDb(), requestIdStr, traverser)
		if err != nil {
			ctx.APIError(router.ErrBadRequest, err.Error())
		}

		// Set the location in the response header to point to this server.
		ctx.Response.Header().Set("Location", chainLocation(requestIdStr, os.Hostname))

		// Start the traverser, and remove it from the cache when it's
		// done running. This could take a very long time to return,
		// so we run it in a goroutine.
		go func() {
			traverser.Run()
			api.cache.Delete(api.cacheDb(), requestIdStr)
		}()
	default:
		ctx.UnsupportedAPIMethod()
	}
}

// PUT <API_ROOT>/job-chains/{requestId}/stop
// Stop the traverser for a job chain.
func (api *API) stopJobChainHandler(ctx router.HTTPContext) {
	switch ctx.Request.Method {
	case "PUT":
		requestIdStr := ctx.Arguments[1]

		val, err := api.cache.Get(api.cacheDb(), requestIdStr)
		if err != nil {
			ctx.APIError(router.ErrNotFound, err.Error())
			return
		}

		traverser, ok := val.(chain.Traverser)
		if !ok {
			ctx.APIError(router.ErrInternal, "Error retreiving traverser from cache.")
			return
		}

		// This is expected to return quickly.
		err = traverser.Stop()
		if err != nil {
			ctx.APIError(router.ErrInvalidParam, "Can't stop the chain (error: %s)", err)
			return
		}

		api.cache.Delete(api.cacheDb(), requestIdStr)
	default:
		ctx.UnsupportedAPIMethod()
	}
}

// GET <API_ROOT>/job-chains/{requestId}/status
// Get the status of a running job chain.
func (api *API) statusJobChainHandler(ctx router.HTTPContext) {
	switch ctx.Request.Method {
	case "GET":
		requestIdStr := ctx.Arguments[1]

		val, err := api.cache.Get(api.cacheDb(), requestIdStr)
		if err != nil {
			ctx.APIError(router.ErrNotFound, err.Error())
			return
		}

		traverser, ok := val.(chain.Traverser)
		if !ok {
			ctx.APIError(router.ErrInternal, "Error retreiving traverser from cache.")
			return
		}

		// This is expected to return quickly.
		statuses := traverser.Status()
		if out, err := marshal(statuses); err != nil {
			ctx.APIError(router.ErrInternal, "Can't encode response (error: %s)", err)
		} else {
			fmt.Fprintln(ctx.Response, string(out))
		}
	default:
		ctx.UnsupportedAPIMethod()
	}
}

// ========================================================================= //

// cacheDb returns a string unique for the api that can be used as the key for
// storing stuff in the cache.
func (api *API) cacheDb() string {
	return CACHE_DB_PREFIX
}

// chainLocation returns the URL location of a job chain
func chainLocation(requestId string, hostname func() (string, error)) string {
	h, _ := hostname()
	return h + API_ROOT + "job-chains/" + requestId
}

// marshal is a helper function to nicely print JSON.
func marshal(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
