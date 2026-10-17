#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-only
# First-start initialization of the live interoperability database (compose.yaml): the role and databases of
# this server, each prepared with schema/postgres.bootstrap.sql as `make db-up` does. Mastodon creates its own
# database as the superuser.
set -eu

psql -v ON_ERROR_STOP=1 -q -U postgres -d postgres <<'SQL'
CREATE ROLE my LOGIN PASSWORD 'my';
CREATE DATABASE my OWNER my;
CREATE DATABASE my_dev OWNER my;
SQL

for db in my my_dev; do
	PGOPTIONS=--client-min-messages=warning psql -v ON_ERROR_STOP=1 -q -U postgres -d "$db" -f /bootstrap/postgres.bootstrap.sql
done
