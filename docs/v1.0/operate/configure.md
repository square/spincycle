---
layout: default
title: Configure
parent: Operate
nav_order: 2
---

# Configure

The Request Manager (RM) and Job Runner (JR) binaries are configured with a YAML config file and environment variables. Configuration values are loaded in this order:

1. Built-in defaults
2. Config file
3. Environment variables

The built-in defaults are only sufficient to run a local development instance. You can [compile the binaries](/spincycle/v1.0/operate/deploy.html#building) and run them without any options, using only the built-in default configs.

For a production deploy, you must provide a YAML config file or environment variables to configure Spin Cycle to your infrastructure. These are the most important config values:

| Request Manager | Job Runner |
| --------------- | ---------- |
| [server.addr](/spincycle/v1.0/operate/configure.html#rm.server.addr) | [server.addr](/spincycle/v1.0/operate/configure.html#jr.server.addr) |
| [jr_client.url](/spincycle/v1.0/operate/configure.html#rm.jr_client.url) | [rm_client.url](/spincycle/v1.0/operate/configure.html#jr.rm_client.url) |
| [mysql.dsn](/spincycle/v1.0/operate/configure.html#rm.mysql.dsn) | &nbsp; |
| [specs.dir](/spincycle/v1.0/operate/configure.html#rm.specs.dir) | &nbsp; |

The RM and JR log the final config on startup.

## Config File

### Specifying

You can specify a config file when starting the RM and JR:

```sh
$ request-manager /etc/spincycle/rm-config.yaml
```

The RM reads `/etc/spincycle/rm-config.yaml` and fails to start if it does not exist or is invalid.

Else, by default, the RM and JR read `config/<ENVIRONMENT>.yaml` if environment variable `ENVIRONMENT` is set and equal to "development", "staging", or "production". For example:

```sh
$ export ENVIRONMENT=production
$ request-manager
```

The RM reads `config/production.yaml` (relative to the current working directory). Unlike an explicit config file, an implicit config file does not need to exist. If it does not exist, built-in defaults or environment values are used.

### Format

The config file is YAML with multiple sections: `server`, `mysql`, `specs`, etc. The [config package](https://godoc.org/github.com/square/spincycle/config) documents each section. **Note**: struct field and YAML field names are different. YAML field names are lower-case and snake_case. Here is a partial example:

```yaml
---
server:
  addr: 10.0.0.50:32308
  tls:
    cert_file: myorg.crt
    key_file: myorg.key
    ca_file: myorg.ca
mysql:
  dsn: "spincycle@tcp(spin-mysql.local:3306)/spincycle_production"
specs:
  dir: /data/app/spin-rm/specs/
jr_client:
  url: https://spincycle-jr.myorg.local:32307
```

## Environment Variables

Most config options have a corresponding environment variable, like `SPINCYCLE_RM_CLIENT_URL` for `rm_client.url`. Exceptions are noted.

Take a config option, change `.` to `_`, upper-case everything, and add `SPINCYCLE_` prefix.

## Request Manager

<a id="rm.auth.admin_roles">auth.admin_roles</a>: Callers with one of these roles are admins (allowed all ops) for all requests. (_No environment variable._)

<a id="rm.auth.strict">auth.strict</a>: Strict requires all requests to have ACLs, else callers are denied unless they have an admin role. Strict is disabled by default which, with the default auth plugin, allows all callers (no auth). (_No environment variable._)

<a id="rm.jr_client.url">jr_client.url</a>: URL that Request Manager uses to connect to any Job Runner. If TLS enabled on JR, use "https" and configure TLS. In production, this is usually a load balancer address in front of N-many JR instances.

<a id="rm.jr_client.tls">jr_client.tls</a>: Enable TLS when RM connects to any JR at [jr_client.url](#rm.jr_client.url).

<a id="rm.mysql.dsn">mysql.dsn</a>: [DSN](https://github.com/go-sql-driver/mysql#dsn-data-source-name) specifying connection to MySQL. Do use `tls` DSN parameter, specify the TLS config and Spin Cycle will add the `tls` DSN parameter automatically.

<a id="rm.server.addr">server.addr</a>: Network address:port to listen on. To listen on all interfaces on the default port, specify ":32308".

<a id="rm.server.tls">server.tls</a>: Enable TLS for clients (users) and when JR connects to RM.

<a id="rm.specs.dir">specs.dir</a>: Directory containing all request spec files. Subdirectories are ignored. The default is "specs/", relative to current working dir.

## Job Runner

<a id="jr.rm_client.url">rm_client.url</a>: URL that Job Runner uses to connect to any Request Manager. If TLS enabled on RM, use "https" and configure TLS. In production, this is usually a load balancer address in front of N-many RM instances.

<a id="jr.rm_client.tls">rm_client.tls</a>: Enable TLS when JR connects to any RM at [rm_client.url](#jr.rm_client.url).

<a id="jr.server.addr">server.addr</a>: Network address:port to listen on and to report to RM. _This must be the address of the specific JR instance that RM can connect to._ Do not use a load balancer address.

<a id="jr.server.tls">server.tls</a>: Enable TLS for incoming connections from RM.

## TLS

Several sections have a TLS section: `server`, `jr_client`, `rm_client`, and `mysql`. The TLS config at each section is separate, so there are potentially four different TLS configs.

To enable TLS, all three files for a section must be specified. For example, to enable `mysql.tls`, you must specify `mysql.tls.cert_file`, `mysql.tls.key_file`, and `mysql.tls.ca_file`.

<a id="tls.cert_file">tls.cert_file</a>: Certificate key file

<a id="tls.key_file">tls.key_file</a>: Private key file

<a id="tls.ca_file">tls.ca_file</a>: Certificate Authority file
