package auth

import (
	"fmt"
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

// Plugin represents the auth plugin. Every request is authenticated and authorized.
type Plugin interface {
	// Authenticate caller from HTTP request.
	Authenticate(*http.Request) (Caller, error)

	// Authorize the created request with user-provided args.
	Authorize(c Caller, op string, req proto.Request) error
}

// AllowAll is the default auth plugin which allows all callers and requests (no auth).
type AllowAll struct{}

func (a AllowAll) Authenticate(*http.Request) (Caller, error) {
	return Caller{
		App:   "all",
		User:  "all",
		Roles: []string{"all"},
	}, nil
}

func (a AllowAll) Authorize(c Caller, op string, req proto.Request) error {
	return nil
}

// ACL represents one role-based ACL entry. This ACL is the same as grapher.ACL.
// The latter is mapped to the former in Server.Boot to keep the two packages
// decoupled, i.e. decouple spec syntax from internal data structures.
type ACL struct {
	Role  string   // user-defined role
	Admin bool     // all ops allowed if true
	Ops   []string // proto.REQUEST_OP_*
}

// Manager provides request authorization using the auth plugin and a list of ACLs.
type Manager struct {
	plugin     Plugin
	acls       map[string][]ACL // request-name (type) -> acl: (if any)
	adminRoles []string
	strict     bool
}

func NewManager(plugin Plugin, acls map[string][]ACL, adminRoles []string, strict bool) Manager {
	return Manager{
		plugin:     plugin,
		acls:       acls,
		adminRoles: adminRoles,
		strict:     strict,
	}
}

// Authorize authorizes the request based on its ACLs, if any. First, this checks
// the caller's roles (provided by the auth plugin Authenticate method) against
// the request ACL roles. If there's a match which allows the request, it calls
// the auth plugin Authorize method to let it authorize the request based on request
// args. For example, the caller might not be allowed to run the request on certain
// hosts. If authorized, returns nil; else, returns an error.
func (m Manager) Authorize(caller Caller, op string, req proto.Request) error {
	// Always allow admins, nothing more to check
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

			// Is it an admin role? If yes, then allow all ops.
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
