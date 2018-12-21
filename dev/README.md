# Spin Cycle Development Sandbox

The `run-spincycle` script configures and runs a Spin Cycle in a sandbox (all files in a single diretory). This is used for local development (on your laptop); it is _not_ suitable for testing, staging, and certainly not produciton.

Presuming,

1. All Go package dependencies are installed (not described here)
1. The `jobs` symlink points to the jobs repo you want (default: `dev/jobs`)
1. MySQL is running on localhost:3306 with root user having no password
1. Have not run the script before
1. The `dev/specs` directory contains the request specs you want.

then running `run-spincycle` should "Just Work" and "Do the Right Thing", like:

```
$ ./run-spincycle
MySQL
 mysql: /usr/local/bin/mysql
   net: tcp
  addr: 127.0.0.1
  port: 3306
  user: root
  pass: (empty)
    db: spincycle_dev
Wrote Request Manager config: dev/sandbox/rm-config.yaml
Wrote Job Runner config: dev/sandbox/jr-config.yaml
Wrote my.cnf: dev/sandbox/my.cnf
Testing MySQL connection...
Creating database spincycle_dev...
Database spincycle_dev is ready
Building request manager...
Building job runner...
Building spinc...
Started Request Manager (PID: 14474)
Started Job Runner (PID: 14475)

#
# OK, Spin Cycle is running
#

==> dev/sandbox/rm.log <==
{"time":"2018-01-30T18:44:07-08:00","remote_ip":"127.0.0.1","method":"GET","uri":"/api/v1/request-list","status":200, "latency":179,"latency_human":"179.249µs","rx_bytes":0,"tx_bytes":2}

==> dev/sandbox/jr.log <==
```

On success, the script keeps running by tailing the Request Manager (RM) and Job Runner (JR) logs. CTRL-C will terminate and clean up.

Run `run-spincycle --help` for command line options.

## dev/ and sandbox/ Files

*Do not modify any files in `dev/`!* The files are templates which are copied into `dev/sandbox/`, and that directory is ignored by git. All files in `dev/sandbox/` are safe to delete, and they can be regenrated by running `run-spincycle` again.

Files in `sandbox/` are _not_ recreated if they already exist. To regenerate a config file, remove it from `sandbox/` and run the script again. To recompile binaries, either delete them (one or several) or specify `--build`.

## Configuring MySQL

The script parses MySQL connetion values (user, pass, host, port, db) from `db.dsn` in `sandbox/rm-config.yaml` and prints them, like:

```
MySQL
 mysql: /usr/local/bin/mysql
   net: tcp
  addr: 127.0.0.1
  port: 3306
  user: root
  pass: (empty)
    db: spincycle_dev
```

Those values are the defaults. If not correct, the script will fail to connect or create the database. Edit `db.dsn` in `sandbox/rm-config.yaml` as needed and run the script again.

The user needs full privileges on the database.

If the database does not exist, it's created. Or, specify `--truncate` to drop and create the database.

## Troubleshoot

`run-spincycle` is just a Bash script, no magic. If there's a problem, run with `/bin/bash -x` and debug. Also good to start clean:

```
killall request-manager ; killall job-runner
rm -rf dev/sandbox
mysql -e "DROP DATABASE IF EXISTS spincycle_dev"
```
