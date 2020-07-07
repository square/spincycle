// Copyright 2018, Square, Inc.

// Package auth provides request authentication and authorization. By default,
// there is no auth; all callers and requests are allowed. The Plugin interface
// allows user-defined auth in combination with user-defined request spec ACLs.
// See docs/auth.md.
package auth

import (
	"fmt"
	"net/http"

	"github.com/square/spincycle/v2/proto"
)

// Caller represents an HTTP client making a request. Callers are determined by
// the Plugin Authenticate method. The default Plugin (AllowAll) returns a zero
// value Caller.
type Caller struct {
	// Name of the caller, whether human (username) or machine (app name). The
	// name is user-defined and only used by Spin Cycle for logging and setting
	// proto.Request.User.
	Name string

	// Roles are user-defined role names, like "admin" or "engineer". Rolls
	// are matched against request ACL roles in specs, which are also user-defined.
	// Roles are case-sensitive and not modified by Spin Cycle in any way.
	Roles []string
}

// Plugin represents the auth plugin. Every request is authenticated and authorized.
// The default Plugin (AllowAll) allows everything: all callers, requests, and ops.
//
// To enable user-defined auth, set App.Context.Plugins.Auth. See docs/customize.md.
type Plugin interface {
	// Authenticate caller from HTTP request. If allowed, return a Caller and nil.
	// Access is denied (HTTP 401) on any error.
	//
	// The returned Caller is user-defined. The most important field is Roles
	// which will be matched against request ACL roles from the specs.
	Authenticate(*http.Request) (Caller, error)

	// Authorize caller to do op for the request. If allowed, return nil.
	// Access is denied (HTTP 401) on any error.
	//
	// This method is post-authorization. Pre-authorization happens automatically
	// by Spin Cycle: it matches a caller role and op to a request ACL role and op.
	// On match, it calls this method. Therefore, when this method is called,
	// it is guaranteed that the caller is authorized for the request and op based
	// on the request ACLs. This method can do further authorization based on
	// the request. For example, if request "stop-host" has arg "hostname", a
	// user-defined Plugin can limit callers to stopping only hosts they own by
	// inspecting the "hostname" arg in the request.
	Authorize(c Caller, op string, req proto.Request) error
}

// AllowAll is the default Plugin which allows all callers and requests (no auth).
type AllowAll struct{}

// Authenticate returns a zero value Caller and nil (allow).
func (a AllowAll) Authenticate(*http.Request) (Caller, error) {
	return Caller{}, nil
}

// Authorize returns nil (allow).
func (a AllowAll) Authorize(c Caller, op string, req proto.Request) error {
	return nil
}

// ACL represents one role-based ACL entry. This ACL is the same as grapher.ACL.
// The latter is mapped to the former in Server.Boot to keep the two packages
// decoupled, i.e. decouple spec syntax from internal data structures.
type ACL struct {
	// User-defined role. This must exactly match a Caller role for the ACL
	// to match.
	Role string

	// Role grants admin access (all ops) to request. Mutually exclusive with Ops.
	Admin bool

	// Ops granted to this role. Mutually exclusive with Admin.
	Ops []string
}

// Manager uses a Plugin to authenticate and authorize. It handles all auth-related
// code and config: user-defined Plugin (or default: AllowAll), request ACLs from
// specs, and options from the config file. Other components use a Manager, not the
// Plugin directly.
type Manager struct {
	plugin     Plugin           // user-defined or AllowAll
	acls       map[string][]ACL // from request specs
	adminRoles []string         // from config file
	strict     bool             // from config file
}

func NewManager(plugin Plugin, acls map[string][]ACL, adminRoles []string, strict bool) Manager {
	return Manager{
		plugin:     plugin,
		acls:       acls,
		adminRoles: adminRoles,
		strict:     strict,
	}
}

// Authenticate wraps the plugin Authenticate method. Unlike Authorize, there
// is no extra logic.
func (m Manager) Authenticate(req *http.Request) (Caller, error) {
	return m.plugin.Authenticate(req)
}

// Authorize authorizes the request based on its ACLs, if any. If the Caller has
// an admin role, it is allowed immediately (no further checks). Else, Caller
// roles are matched to request ACL roles. On match, the op is matched to the
// request ACL ops. On match again, the Plugin Authorize method is called for
// post-authorization. If it returns nil, the request is allowed.
//
// If the request has no ACLs and the caller does not have an admin role, strict
// mode determines the result: allow if disabled (no ACLs = allow all), deny
// if enabled (no ACLs = deny all non-admins).
//
// Any return error denies the request (HTTP 401), and the error message explains why.
func (m Manager) Authorize(caller Caller, op string, req proto.Request) error {
	// Always allow admins, nothing more to check. This is global admin_roles from config:
	// role which are admins for all requests regardless of request-specific ACLs.
	if m.isAdmin(caller) {
		return nil // allow
	}

	// Get ACLs for this request
	acls, ok := m.acls[req.Type]
	if !ok {
		return fmt.Errorf("denied: request %s is not defined", req.Type) // shouldn't happen
	}

	// If no request ACLs and strict, deny. Else (default), allow.
	if len(acls) == 0 {
		if m.strict {
			return fmt.Errorf("denied: request %s has no ACLs and strict auth is enabled", req.Type)
		}
		return nil // not strict, allow
	}

	// Pre-authorize based on request ACL roles and ops. For every request ACL,
	// check if caller has role and role grants the op. Allow on first match.
	rolesMatch := false // at least one role matches
	opMatch := false    // at least one role + op matches
REQUEST_ACLS:
	for _, acl := range acls {
		for _, crole := range caller.Roles {
			// Does caller have this role?
			if crole != acl.Role {
				continue // no
			}
			rolesMatch = true // yes

			// Is it an admin role? If yes, then allow all ops. This is
			// request-specific admin role, not global admin_roles from config.
			if acl.Admin {
				opMatch = true // yes
				break REQUEST_ACLS
			}

			for _, aclop := range acl.Ops {
				// Does role grant caller the op?
				if aclop != op {
					continue // no
				}
				opMatch = true // yes
				break REQUEST_ACLS
			}
		}
	}
	if !rolesMatch {
		reqRoles := make([]string, len(acls))
		for i, acl := range acls {
			reqRoles[i] = acl.Role
		}
		return fmt.Errorf("denied: no caller role matches %s ACL: caller has %v, %s requires one of %v", req.Type, caller.Roles, req.Type, reqRoles)
	}
	if !opMatch {
		return fmt.Errorf("denied: no matching caller role granted %s op for request %s", op, req.Type)
	}

	// Plugin authorize based on op and request args
	if err := m.plugin.Authorize(caller, op, req); err != nil {
		return fmt.Errorf("denied by auth plugin Authorize: %s", err)
	}

	return nil // allow
}

// isAdmin returns true if the caller has an admin role.
func (m Manager) isAdmin(caller Caller) bool {
	if len(m.adminRoles) == 0 {
		return false
	}
	for _, arole := range m.adminRoles {
		for _, crole := range caller.Roles {
			if crole == arole {
				return true
			}
		}
	}
	return false
}
