// Copyright 2017-2019, Square, Inc.

// Package api provides controllers for each api endpoint. Controllers are
// "dumb wiring"; there is little to no application logic in this package.
// Controllers call and coordinate other packages to satisfy the api endpoint.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/orcaman/concurrent-map"

	"github.com/square/spincycle/job-runner/app"
	"github.com/square/spincycle/job-runner/chain"
	"github.com/square/spincycle/job-runner/status"
	"github.com/square/spincycle/proto"
	v "github.com/square/spincycle/version"
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
	baseURL          string
	// --
	echo *echo.Echo
}

type Config struct {
	AppCtx           app.Context
	TraverserFactory chain.TraverserFactory
	TraverserRepo    cmap.ConcurrentMap
	StatusManager    status.Manager
	ShutdownChan     chan struct{}
	BaseURL          string // returned in location header when starting/resuming job chains
}

// NewAPI creates a new API struct. It initializes an echo web server within the
// struct, and registers all of the API's routes with it.
func NewAPI(cfg Config) *API {
	api := &API{
		appCtx:           cfg.AppCtx,
		traverserFactory: cfg.TraverserFactory,
		traverserRepo:    cfg.TraverserRepo,
		stat:             cfg.StatusManager,
		shutdownChan:     cfg.ShutdownChan,
		baseURL:          cfg.BaseURL,
		// --
		echo: echo.New(),
	}

	// //////////////////////////////////////////////////////////////////////
	// Routes
	// //////////////////////////////////////////////////////////////////////
	api.echo.POST(API_ROOT+"job-chains", api.newJobChainHandler)                 // start running new job chain
	api.echo.POST(API_ROOT+"job-chains/resume", api.resumeJobChainHandler)       // resume suspended job chain
	api.echo.PUT(API_ROOT+"job-chains/:requestId/stop", api.stopJobChainHandler) // stop job chain

	api.echo.GET(API_ROOT+"status/running", api.statusRunningHandler) // return running jobs -> []proto.JobStatus
	api.echo.GET("/version", api.versionHandler)

	// //////////////////////////////////////////////////////////////////////
	// Middleware and hooks
	// //////////////////////////////////////////////////////////////////////
	api.echo.Use(middleware.Recover())
	api.echo.Use(middleware.Logger())

	// Called before every route
	api.echo.Use((func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Spincycle-Version", v.Version())
			return next(c)
		}
	}))

	return api
}

// Run API server.
func (api *API) Run() error {
	var err error
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err = api.echo.StartTLS(api.appCtx.Config.Server.Addr, api.appCtx.Config.Server.TLS.CertFile, api.appCtx.Config.Server.TLS.KeyFile)
	} else {
		err = api.echo.Start(api.appCtx.Config.Server.Addr)
	}
	return err
}

// Stop stops the API when it's running. When Stop is called, Run returns
// immediately. Make sure to wait for Stop to return.
func (api *API) Stop() error {
	var err error
	if api.appCtx.Config.Server.TLS.CertFile != "" && api.appCtx.Config.Server.TLS.KeyFile != "" {
		err = api.echo.TLSServer.Shutdown(context.TODO())
	} else {
		err = api.echo.Server.Shutdown(context.TODO())
	}
	return err
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.echo.ServeHTTP(w, r)
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

	// Convert the payload into a proto.JobChain and validate.
	var jc proto.JobChain
	if err := c.Bind(&jc); err != nil {
		return err
	}
	if err := chain.Validate(jc, true); err != nil {
		return handleError(err)
	}

	// Create a new traverser.
	t, err := api.traverserFactory.Make(&jc)
	if err != nil {
		return handleError(err)
	}

	// Save traverser to the repo.
	wasAbsent := api.traverserRepo.SetIfAbsent(jc.RequestId, t)
	if !wasAbsent {
		return handleError(ErrDuplicateTraverser)
	}

	// Start the traverser, and remove it from the repo when it's
	// done running. This could take a very long time to return,
	// so we run it in a goroutine.
	go func() {
		defer api.traverserRepo.Remove(jc.RequestId)
		t.Run()
	}()

	// Set the location in the response header to point to this server.
	c.Response().Header().Set("Location", api.chainLocation(jc.RequestId))

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
	if err := chain.Validate(*sjc.JobChain, false); err != nil {
		return handleError(err)
	}

	// Create a new traverser.
	t, err := api.traverserFactory.MakeFromSJC(&sjc)
	if err != nil {
		return handleError(err)
	}

	// Save traverser to the repo.
	wasAbsent := api.traverserRepo.SetIfAbsent(sjc.RequestId, t)
	if !wasAbsent {
		return handleError(ErrDuplicateTraverser)
	}

	// Set the location in the response header to point to this server.
	c.Response().Header().Set("Location", api.chainLocation(sjc.RequestId))

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

// GET <API_ROOT>/status/running
func (api *API) statusRunningHandler(c echo.Context) error {
	f := proto.StatusFilter{
		RequestId: c.QueryParam("requestId"),
		OrderBy:   c.QueryParam("orderBy"),
	}
	jobs, err := api.stat.Running(f)
	if err != nil {
		return handleError(err)
	}
	return c.JSON(http.StatusOK, jobs)
}

func (api *API) versionHandler(c echo.Context) error {
	return c.String(http.StatusOK, v.Version())
}

// ------------------------------------------------------------------------- //

func (api *API) chainLocation(requestId string) string {
	return api.baseURL + API_ROOT + "job-chains/" + requestId
}

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
