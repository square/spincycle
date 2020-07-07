package auth_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/square/spincycle/v2/proto"
	"github.com/square/spincycle/v2/request-manager/auth"
	"github.com/square/spincycle/v2/test/mock"
)

func TestManagerAuthenticate(t *testing.T) {
	caller := auth.Caller{
		Name:  "test",
		Roles: []string{"test"},
	}
	plugin := mock.AuthPlugin{
		AuthenticateFunc: func(req *http.Request) (auth.Caller, error) {
			return caller, nil
		},
	}

	m := auth.NewManager(plugin, map[string][]auth.ACL{}, nil, true)
	gotCaller, err := m.Authenticate(nil)
	if err != nil {
		t.Error(err)
	}
	if gotCaller.Name != caller.Name {
		t.Errorf("got Caller.Name=%s, expected %s", gotCaller.Name, caller.Name)
	}
}

func TestManagerWithACLs(t *testing.T) {
	acls := map[string][]auth.ACL{
		"req1": []auth.ACL{
			{
				Role:  "role1",
				Admin: true,
			},
			{
				Role: "role2",
				Ops:  []string{"start", "stop"},
			},
		},
	}

	authCalled := false
	var authErr error
	plugin := mock.AuthPlugin{
		AuthorizeFunc: func(caller auth.Caller, op string, req proto.Request) error {
			authCalled = true
			return authErr
		},
	}
	adminRoles := []string{"finch"}
	m := auth.NewManager(plugin, acls, adminRoles, true)

	caller := auth.Caller{
		Name:  "dn",
		Roles: []string{"dev"}, // no admin role
	}
	req := proto.Request{
		Id:   "abc",
		Type: "req1",
	}

	// Should be denied (return error) because caller doesn't have an admin role
	// and or any matching role
	authCalled = false
	err := m.Authorize(caller, proto.REQUEST_OP_START, req)
	t.Log(err)
	if err == nil {
		t.Errorf("allowed, expected Authorize to return err")
	}
	if authCalled == true {
		t.Errorf("auth plugin Authorize called, expected it NOT to be called")
	}

	// Should be denied (return error) because caller has role (role2) but it
	// doens't grant the bogus op, "foo".
	authCalled = false
	caller.Roles = []string{"role2"}
	err = m.Authorize(caller, "foo", req)
	t.Log(err)
	if err == nil {
		t.Errorf("allowed, expected Authorize to return err")
	}
	if authCalled == true {
		t.Errorf("auth plugin Authorize called, expected it NOT to be called")
	}

	// Set admin role to caller and it should be allowed even though the request
	// ACL has no explicit role for the admin role. It's allowed because caller
	// has the admin role. It doesn't even check Authorize plugin method.
	authCalled = false
	caller.Roles = adminRoles
	err = m.Authorize(caller, proto.REQUEST_OP_START, req)
	if err != nil {
		t.Errorf("not allowed (%s), expected Authorize to return nil", err)
	}
	if authCalled == true {
		t.Errorf("auth plugin Authorize called, expected it NOT to be called")
	}

	// Normal match allowed: req has role2 with op "start". When pre-authorization
	// passes, then the plugin Authorize method is called.
	authCalled = false
	caller.Roles = []string{"role2"}
	err = m.Authorize(caller, proto.REQUEST_OP_START, req)
	if err != nil {
		t.Errorf("not allowed (%s), expected Authorize to return nil", err)
	}
	if authCalled == false {
		t.Errorf("auth plugin Authorize not called, expected it to be called")
	}

	// Admin role match allowed: role1 is admin role, so even though op is fake,
	// doesn't matter. Admin roles can do anything wrt pre-auth. The auth plugin
	// can still deny it later.
	authCalled = false
	caller.Roles = []string{"role1"}
	err = m.Authorize(caller, "fake", req)
	if err != nil {
		t.Errorf("not allowed (%s), expected Authorize to return nil", err)
	}
	if authCalled == false {
		t.Errorf("auth plugin Authorize not called, expected it to be called")
	}

	// If auth plugin Authorize denies, manager denies
	authCalled = false
	authErr = fmt.Errorf("fake auth error")
	err = m.Authorize(caller, proto.REQUEST_OP_START, req)
	if err == nil {
		t.Errorf("allowed, expected Authorize to return err")
	}
	if authCalled == false {
		t.Errorf("auth plugin Authorize not called, expected it to be called")
	}

	// Don't allow nonexistent requests
	authCalled = false
	err = m.Authorize(caller, "foo", proto.Request{Type: "foo"})
	t.Log(err)
	if err == nil {
		t.Errorf("allowed, expected Authorize to return err")
	}
	if authCalled == true {
		t.Errorf("auth plugin Authorize called, expected it NOT to be called")
	}
}

func TestManagerNoACLs(t *testing.T) {
	acls := map[string][]auth.ACL{
		"req1": []auth.ACL{},
	}

	authCalled := false
	var authErr error
	plugin := mock.AuthPlugin{
		AuthorizeFunc: func(caller auth.Caller, op string, req proto.Request) error {
			authCalled = true
			return authErr
		},
	}
	m := auth.NewManager(plugin, acls, nil, true) // true = STRICT MODE

	caller := auth.Caller{
		Name:  "dn",
		Roles: []string{"dev"}, // no admin role
	}
	req := proto.Request{
		Id:   "abc",
		Type: "req1",
	}

	// Should be denied (return error) because no ACLs + strict mode
	authCalled = false
	err := m.Authorize(caller, proto.REQUEST_OP_START, req)
	t.Log(err)
	if err == nil {
		t.Errorf("allowed, expected Authorize to return err")
	}
	if authCalled == true {
		t.Errorf("auth plugin Authorize called, expected it NOT to be called")
	}

	// But turn strict mode off and no ACLs = allow all
	m = auth.NewManager(plugin, acls, nil, false) // false = strict mode off
	authCalled = false
	err = m.Authorize(caller, proto.REQUEST_OP_START, req)
	if err != nil {
		t.Errorf("not allowed (%s), expected Authorize to return nil", err)
	}
	if authCalled == true {
		t.Errorf("auth plugin Authorize called, expected it NOT to be called")
	}
}

func TestAllowAll(t *testing.T) {
	all := auth.AllowAll{}

	caller, err := all.Authenticate(nil)
	if err != nil {
		t.Errorf("not allowed (%s), expected Authorize to return nil", err)
	}

	err = all.Authorize(caller, "", proto.Request{})
	if err != nil {
		t.Errorf("not allowed (%s), expected Authorize to return nil", err)
	}
}
