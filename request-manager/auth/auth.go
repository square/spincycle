package auth

import (
	"net/http"

	"github.com/square/spincycle/proto"
)

// Caller represents an app or user making a request. Callers are determined by
// the auth plugin, if any.
type Caller struct {
	App   string   // caller is app, or
	User  string   // caller is human (mutually exclusive)
	Roles []string // for User
}

// Auth represents the auth plugin interface. If given, the methods are called
// for every request to authenticate the calling app or user, then authorize it
// for the request and op (see proto.REQUEST_OP.*).
type Auth interface {
	// Authenticate caller from HTTP request.
	Authenticate(*http.Request) (Caller, error)

	// Authorize the created request with user-provided args.
	Authorize(c Caller, op string, req proto.Request) error
}

// AllowAll is the default auth plugin which allows all apps and users.
type AllowAll struct{}

func (a AllowAll) Authenticate(*http.Request) (Caller, error) {
	return Caller{
		App:  "all",
		User: "everyone",
	}, nil
}

func (a AllowAll) Authorize(c Caller, op string, req proto.Request) error {
	return nil
}
