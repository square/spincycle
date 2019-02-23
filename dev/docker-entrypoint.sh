#!/bin/sh
set -ex
cd /app/spin-rm
if [[ $INIT_SPINCYCLE_DB ]]; then
  mysql -h mysql -e "DROP DATABASE IF EXISTS spincycle_development"
  mysql -h mysql -e "CREATE DATABASE spincycle_development"
  mysql -h mysql -D spincycle_development < resources/request_manager_schema.sql
fi
exec bin/request-manager
