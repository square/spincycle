---
layout: default
title: Auth
parent: Operate
nav_order: 4
---

# Auth

Spin Cycle supports role-based authentication (RBAC). As an open-source product, Spin Cycle provides the auth framework and you provide the implementation details. There are no built-in or default roles; roles are user-defined and determined during authentication. By default, auth is disabled: everyone can do everything.

Auth is a whitlelist. To allow a request, callers (any HTTP client) must have a role that matches a role defined for the request. Authentication assigns roles to callers. Requests are given [ACLs (access control lists)](https://godoc.org/github.com/square/spincycle/request-manager/auth#ACL) when the specs are written. Authorization matches the former (caller roles) to the latter (request ACLs). If there is a match, the request is allowed; else, the request is denied.

Global admin roles can be configured. This makes it easy to ensure admins always have access, without having to define access for every request. See [auth.admin_roles](/spincycle/v1.0/operate/configure#rm.auth.admin_roles).

Enabling auth requires writing an auth plugin and defining request ACLs.

## Auth Plugin

An [auth.Plugin](https://godoc.org/github.com/square/spincycle/request-manager/auth#Plugin) is required to enable authentication. Primarily, the `Authenticate` method contains user-specific logic for determining the [Caller](https://godoc.org/github.com/square/spincycle/request-manager/auth#Caller) and its roles.

Spin Cycle does pre-authorization: before calling the `Authorize` method of the auth plugin, Spin Cycle matches caller roles to the request ACL. (Or, if caller has an admin role, authorization is successful regardless of request ACLs.) If there is a match, the `Authorize` method is called. The plugin can do further authorization based on request-specific details.

Since the auth plugin is code, see [Extensions](/spincycle/v1.0/develop/extensions) for enabling the plugin and custom building Spin Cycle.

## Request ACLs

Request [access control lists (ACLs)](https://godoc.org/github.com/square/spincycle/request-manager/auth#ACL) are defined in request specs:

```yaml
sequences:
  restart-app:
    request: true
    args:
      required:
        - name: app
          desc: "App name"
    acl:
      - role: eng
        admin: true
      - role: ba
        ops: ["start"]
    nodes:
```

The request spec snippet above, for request "restart-app", has two ACLs. The first defines that callers with the "eng" role are request admins, i.e. allowed to do anything with the request. The second defines that callers with the "ba" role can start the request. Access is denied if the caller does not have one of these two roles, or a role listed in [auth.admin_roles](/spincycle/v1.0/operate/configure#rm.auth.admin_roles).

"ops" is currently a placeholder for future authorization. The allowed values are "start" and "stop".

Spin Cycle automatically pre-authorizes caller based on request ACLs. If allowed, it calls the `Authorize` method of the auth plugin which can do further authorization. For example, this request has an `app` arg. The auth plugin could authorize callers to restart only apps they own.
