// Copyright 2017, Square, Inc.

package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/router"
)

const (
	API_ROOT           = "/api/v1/"
	REQUEST_ID_PATTERN = "([0-9]+)"
)

type API struct {
	Router        *router.Router
	chainRepo     chain.Repo
	runnerFactory runner.RunnerFactory
	activeT       map[uint]chain.Traverser // Active Traversers, keyed on requestId.
	activeTMutex  *sync.RWMutex            // Protection for activeT.
}

func NewAPI(router *router.Router, chainRepo chain.Repo, runnerFactory runner.RunnerFactory) *API {
	api := &API{
		Router:        router,
		chainRepo:     &chain.FakeRepo{},
		runnerFactory: runnerFactory,
		activeT:       make(map[uint]chain.Traverser),
		activeTMutex:  &sync.RWMutex{},
	}

	api.Router.AddRoute(API_ROOT+"job-chains", api.newJobChainHandler, "api-new-job-chain")
	api.Router.AddRoute(API_ROOT+"job-chains/"+REQUEST_ID_PATTERN+"/stop", api.stopJobChainHandler, "api-stop-job-chain")
	api.Router.AddRoute(API_ROOT+"job-chains/"+REQUEST_ID_PATTERN+"/status", api.statusJobChainHandler, "api-status-job-chain")

	return api
}

// ============================== CONTROLLERS ============================== //

// POST <API_ROOT>/job-chains
// Add a new job chain to the Job Runner API and start traversing it.
func (api *API) newJobChainHandler(ctx router.HTTPContext) {
	switch ctx.Request.Method {
	case "POST":
		decoder := json.NewDecoder(ctx.Request.Body)
		var jobChain proto.JobChain
		err := decoder.Decode(&jobChain)
		if err != nil {
			ctx.APIError(router.ErrInternal, "Can't decode request body (error: %s)", err)
		}

		traverser := chain.NewTraverser(api.chainRepo, api.runnerFactory, chain.NewChain(&jobChain))
		go func() {
			api.addTraverser(jobChain.RequestId, traverser)
			// This could take a very long time to return.
			traverser.Run()
			api.removeTraverser(jobChain.RequestId)
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
		requestId, err := strconv.ParseUint(ctx.Arguments[1], 10, 0)
		if err != nil {
			ctx.APIError(router.ErrInvalidParam, "Can't parse requestId (error: %s)", err)
			return
		}
		traverser, err := api.getTraverser(uint(requestId))
		if err != nil {
			ctx.APIError(router.ErrNotFound, err.Error())
			return
		}

		// This is expected to return quickly.
		traverser.Stop()

		api.removeTraverser(uint(requestId))
	default:
		ctx.UnsupportedAPIMethod()
	}
}

// GET <API_ROOT>/job-chains/{requestId}/status
// Get the status of a running job chain.
func (api *API) statusJobChainHandler(ctx router.HTTPContext) {
	switch ctx.Request.Method {
	case "GET":
		requestId, err := strconv.ParseUint(ctx.Arguments[1], 10, 0)
		if err != nil {
			ctx.APIError(router.ErrInvalidParam, "Can't parse requestId (error: %s)", err)
			return
		}
		traverser, err := api.getTraverser(uint(requestId))
		if err != nil {
			ctx.APIError(router.ErrNotFound, err.Error())
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

// addTraverser adds a Traverser to activeT.
func (api *API) addTraverser(requestId uint, traverser chain.Traverser) {
	api.activeTMutex.Lock()
	api.activeT[requestId] = traverser
	api.activeTMutex.Unlock()
}

// removeTraverser removes a Traverser from activeT.
func (api *API) removeTraverser(requestId uint) {
	api.activeTMutex.Lock()
	delete(api.activeT, requestId)
	api.activeTMutex.Unlock()
}

// getTraverser tries to get a specifc Traverser from activeT. If the Traverser
// does not exist, an error is returned.
func (api *API) getTraverser(requestId uint) (chain.Traverser, error) {
	api.activeTMutex.RLock()
	defer api.activeTMutex.RUnlock()
	t, ok := api.activeT[requestId]
	if !ok {
		return nil, fmt.Errorf("Could not find a running traverser for requestId=%d on this host.", requestId)
	}
	return t, nil
}

// marshal is a helper function to nicely print JSON.
func marshal(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
