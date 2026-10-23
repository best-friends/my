# Desired-state schema

The database schema of each dialect (ADR-0023): Atlas Community Edition diffs it against the target database and applies
the difference (`scripts/task.sh db:apply DB_DIALECT=sqlite|postgres`; for PostgreSQL, against the `public` schema of the
target database). Never write migration SQL by hand.

## Layout

- `sqlite/` and `postgres/` hold one file per module, `<NN>-<module>.sql` (ADR-0082). The modules and their numbers are those
  of [data-model §4](../docs/architecture/data-model/04-tables/README.md); `10-infrastructure.sql` holds the tables that
  adapters own.
- Atlas (`file://schema/<dialect>`), sqlc (`sqlc.yaml`), and the Go embedding (`schema.go`) read the files in name order, so a
  statement may only use what an earlier statement or file creates: an index or comment goes after its table. Number a new
  module's file after the files it depends on.
- A table, its indexes, and its comments go in the file of the module that owns the table, also when an index serves
  another module's queries.
- Edit the same file in every dialect together: both directories have the same files, and each file declares the same
  tables in the same order (`schema_test.go`).
- `postgres.bootstrap.sql` runs before the schema on every PostgreSQL database (`scripts/task.sh db:up`, `db:bootstrap`).

## Conventions

Tables follow [the logical data model](../docs/architecture/data-model/README.md), mapping logical types per its
[§1.2](../docs/architecture/data-model/01-conventions.md#12-論理型).

SQLite:

- id, string, text, iri, lang, and enum are TEXT; timestamp is INTEGER Unix milliseconds; bool is INTEGER 0/1; bytes is
  BLOB. Every table is STRICT.

PostgreSQL:

- id is uuid (ADR-0078; its byte order is time order for UUIDv7, and Go sees it as a string), except in the columns that
  hold a value which is not always a UUID the server generated (provider and webhook identifiers, client_id, audit
  target_id, emoji_key, run keys, cursors), which stay text COLLATE "C"; string, text, iri, lang, and enum are text;
  timestamp is timestamptz; bool is boolean; bytes is bytea.
- Atlas Community Edition does not manage extensions, sequences, views, or triggers; keep them out of the schema.
  Extensions go in `postgres.bootstrap.sql`.
