// Copyright 2017, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"github.com/orcaman/concurrent-map"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/proto"
)

const (
	API_ROOT = "/api/v1/"
)

var (
	// Errors related to getting and setting traversers in the traverser repo.
	ErrDuplicateTraverser = errors.New("traverser already exists")
	ErrTraverserNotFound  = errors.New("traverser not found")
	ErrInvalidTraverser   = errors.New("traverser found, but type is invalid")
)

// api provides controllers for endpoints it registers with a router.
type API struct {
	// An echo web server.
	echo *echo.Echo

	// Factory for creating traversers.
	traverserFactory chain.TraverserFactory

	// Repo for storing and retrieving traversers.
	traverserRepo cmap.ConcurrentMap
}

// NewAPI cretes a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
func NewAPI(traverserFactory chain.TraverserFactory, traverserRepo cmap.ConcurrentMap) *API {
	api := &API{
		echo:             echo.New(),
		traverserFactory: traverserFactory,
		traverserRepo:    traverserRepo,
	}

	// //////////////////////////////////////////////////////////////////////
	// Routes
	// //////////////////////////////////////////////////////////////////////
	// Create a new job chain and start running it.
	api.echo.POST(API_ROOT+"job-chains", api.newJobChainHandler)
	// Stop running a job chain.
	api.echo.PUT(API_ROOT+"job-chains/:requestId/stop", api.stopJobChainHandler)
	// Get the status of a job chain.
	api.echo.GET(API_ROOT+"job-chains/:requestId/status", api.statusJobChainHandler)

	return api
}

// ServeHTTP allows the API to statisfy the http.HandlerFunc interface.
func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	es := standard.WithConfig(engine.Config{})
	es.SetHandler(api.echo)
	es.ServeHTTP(w, r)
}

// Use adds middleware to the echo web server in the API. See
// https://echo.labstack.com/middleware for more details.
func (api *API) Use(middleware ...echo.MiddlewareFunc) {
	api.echo.Use(middleware...)
}

// ============================== CONTROLLERS ============================== //

// POST <API_ROOT>/job-chains
// Do some basic validation on a job chain, and, if it passes, add it to the
// chain repo. If it doesn't pass, return the validation error. Then start
// running the job chain.
func (api *API) newJobChainHandler(c echo.Context) error {
	// Convert the payload into a proto.JobChain.
	var jc proto.JobChain
	if err := c.Bind(&jc); err != nil {
		return err
	}

	// Create a new traverser.
	t, err := api.traverserFactory.Make(jc)
	if err != nil {
		return handleError(err)
	}

	// Save traverser to the repo.
	wasAbsent := api.traverserRepo.SetIfAbsent(jc.RequestId, t)
	if !wasAbsent {
		return handleError(ErrDuplicateTraverser)
	}

	// Set the location in the response header to point to this server.
	c.Response().Header().Set("Location", chainLocation(jc.RequestId, os.Hostname))

	// Start the traverser, and remove it from the repo when it's
	// done running. This could take a very long time to return,
	// so we run it in a goroutine.
	go func() {
		defer api.traverserRepo.Remove(jc.RequestId)
		t.Run()
	}()

	return nil
}

// PUT <API_ROOT>/job-chains/{requestId}/stop
// Stop the traverser for a job chain.
func (api *API) stopJobChainHandler(c echo.Context) error {
	requestId := c.Param("requestId")

	// Get the traverser to the repo.
	val, exists := api.traverserRepo.Get(requestId)
	if !exists {
		return handleError(ErrTraverserNotFound)
	}
	traverser, ok := val.(chain.Traverser)
	if !ok {
		return handleError(ErrInvalidTraverser)
	}

	// Stop the traverser. This is expected to return quickly.
	err := traverser.Stop()
	if err != nil {
		return handleError(err)
	}

	// Remove the traverser from the repo.
	api.traverserRepo.Remove(requestId)

	return nil
}

// GET <API_ROOT>/job-chains/{requestId}/status
// Get the status of a running job chain.
func (api *API) statusJobChainHandler(c echo.Context) error {
	requestId := c.Param("requestId")

	// Get the traverser to the repo.
	val, exists := api.traverserRepo.Get(requestId)
	if !exists {
		return handleError(ErrTraverserNotFound)
	}
	traverser, ok := val.(chain.Traverser)
	if !ok {
		return handleError(ErrInvalidTraverser)
	}

	// This is expected to return quickly.
	statuses, err := traverser.Status()
	if err != nil {
		return handleError(err)
	}

	// Return the statuses.
	return c.JSON(http.StatusOK, statuses)
}

// ------------------------------------------------------------------------- //

func handleError(err error) *echo.HTTPError {
	switch err.(type) {
	case chain.ErrInvalidChain:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		switch err {
		case ErrTraverserNotFound:
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		case ErrDuplicateTraverser:
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return nil
}

// chainLocation returns the URL location of a job chain
func chainLocation(requestId string, hostname func() (string, error)) string {
	h, _ := hostname()
	return h + API_ROOT + "job-chains/" + requestId
}
