// Copyright 2017-2018, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/orcaman/concurrent-map"

	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/status"
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

	// Error when Job Runner is shutting down and not starting new job chains
	ErrShuttingDown = errors.New("Job Runner is shutting down - no new job chains are being started")
)

// api provides controllers for endpoints it registers with a router.
type API struct {
	appCtx           app.Context
	traverserFactory chain.TraverserFactory
	traverserRepo    cmap.ConcurrentMap
	stat             status.Manager
	shutdownChan     chan struct{}
	// --
	echo *echo.Echo
}

// NewAPI creates a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
func NewAPI(appCtx app.Context, traverserFactory chain.TraverserFactory, traverserRepo cmap.ConcurrentMap, stat status.Manager, shutdownChan chan struct{}) *API {
	api := &API{
		appCtx:           appCtx,
		traverserFactory: traverserFactory,
		traverserRepo:    traverserRepo,
		stat:             stat,
		shutdownChan:     shutdownChan,
		// --
		echo: echo.New(),
	}

	// //////////////////////////////////////////////////////////////////////
	// Routes
	// //////////////////////////////////////////////////////////////////////
	// Create a new job chain and start running it.
	api.echo.POST(API_ROOT+"job-chains", api.newJobChainHandler)
	// Resume running a suspended job chain.
	api.echo.POST(API_ROOT+"job-chains/resume", api.resumeJobChainHandler)
	// Stop running a job chain.
	api.echo.PUT(API_ROOT+"job-chains/:requestId/stop", api.stopJobChainHandler)
	// Get the status of a job chain.
	api.echo.GET(API_ROOT+"job-chains/:requestId/status", api.statusJobChainHandler)

	api.echo.GET(API_ROOT+"status/running", api.statusRunningHandler)

	// //////////////////////////////////////////////////////////////////////
	// Middleware and hooks
	// //////////////////////////////////////////////////////////////////////
	api.echo.Use(middleware.Recover())
	api.echo.Use(middleware.Logger())

	// Auth hook
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if appCtx.Hooks.Auth == nil {
				return next(c) // no auth
			}
			ok, err := appCtx.Hooks.Auth(c.Request())
			if err != nil {
				return err
			}
			if !ok {
				//c.Response().Header().Set(echo.HeaderWWWAuthenticate, basic+" realm="+realm)
				return echo.ErrUnauthorized // 401
			}
			return next(c) // auth OK
		}
	}))

	// SetUsername hook
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if appCtx.Hooks.SetUsername == nil {
				return next(c) // no auth
			}
			username, err := appCtx.Hooks.SetUsername(c.Request())
			if err != nil {
				return err
			}
			c.Set("username", username)
			return next(c)
		}
	}))

	return api
}

func (api *API) Run() error {
	// Shut down the API when the Job Runner shuts down.
	doneChan := make(chan struct{})
	doneShuttingDown := make(chan struct{})
	go func() {
		select {
		case <-api.shutdownChan:
			api.shutdown()
			close(doneShuttingDown)
		case <-doneChan:
			return
		}
	}()

	// Run API server.
	var err error
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err = api.echo.StartTLS(api.appCtx.Config.Server.ListenAddress, api.appCtx.Config.Server.TLS.CertFile, api.appCtx.Config.Server.TLS.KeyFile)
	} else {
		err = api.echo.Start(api.appCtx.Config.Server.ListenAddress)
	}

	// If API was shut down, wait to make sure it's done shutting down.
	select {
	case <-api.shutdownChan:
		<-doneShuttingDown
	default:
		close(doneChan) // stop the shutdown goroutine
	}
	return err
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.echo.ServeHTTP(w, r)
}

func (api *API) shutdown() {
	// Wait for all traversers to shut down. Timeout if they aren't done
	// within 20 seconds.
	timeout := time.After(20 * time.Second)
WAIT_FOR_TRAVERSERS:
	for !api.traverserRepo.IsEmpty() {
		select {
		case <-time.After(10 * time.Millisecond):
			// Check again if traversers are all done.
		case <-timeout:
			break WAIT_FOR_TRAVERSERS
		}
	}

	// Shut down server.
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err := api.echo.TLSServer.Shutdown(context.TODO())
		if err != nil {
			log.Warnf("error when shutting down API: %s", err)
		}
	} else {
		err := api.echo.Server.Shutdown(context.TODO())
		if err != nil {
			log.Warnf("error when shutting down API: %s", err)
		}
	}
}

// ============================== CONTROLLERS ============================== //

// POST <API_ROOT>/job-chains
// Do some basic validation on a job chain, and, if it passes, add it to the
// chain repo. If it doesn't pass, return the validation error. Then start
// running the job chain.
func (api *API) newJobChainHandler(c echo.Context) error {
	// If Job Runner is shutting down, don't start running any new job chains.
	select {
	case <-api.shutdownChan:
		return handleError(ErrShuttingDown)
	default:
	}

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

// POST <API_ROOT>/job-chains/resume
// Resume running a previously suspended job chain. Do some basic validation on
// the job chain and, if it passes, add it to the chain repo. If it doesn't pass,
// return the validation error. Then start running the job chain.
func (api *API) resumeJobChainHandler(c echo.Context) error {
	// If Job Runner is shutting down, don't start running any new job chains.
	select {
	case <-api.shutdownChan:
		return handleError(ErrShuttingDown)
	default:
	}

	// Convert the payload into a proto.SuspendedJobChain.
	var sjc proto.SuspendedJobChain
	if err := c.Bind(&sjc); err != nil {
		return err
	}

	// Create a new traverser.
	t, err := api.traverserFactory.MakeFromSJC(sjc)
	if err != nil {
		return handleError(err)
	}

	// Save traverser to the repo.
	wasAbsent := api.traverserRepo.SetIfAbsent(sjc.RequestId, t)
	if !wasAbsent {
		return handleError(ErrDuplicateTraverser)
	}

	// Set the location in the response header to point to this server.
	c.Response().Header().Set("Location", chainLocation(sjc.RequestId, os.Hostname))

	// Start the traverser, and remove it from the repo when it's
	// done running. This could take a very long time to return,
	// so we run it in a goroutine.
	go func() {
		defer api.traverserRepo.Remove(sjc.RequestId)
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

// GET <API_ROOT>/status/running
func (api *API) statusRunningHandler(c echo.Context) error {
	running, err := api.stat.Running()
	if err != nil {
		return handleError(ErrTraverserNotFound)
	}
	return c.JSON(http.StatusOK, running)
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
		case ErrShuttingDown:
			return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
}

// chainLocation returns the URL location of a job chain
func chainLocation(requestId string, hostname func() (string, error)) string {
	h, _ := hostname()
	url, _ := url.Parse(h + API_ROOT + "job-chains/" + requestId)
	return url.EscapedPath()
}
