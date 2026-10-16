-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Probe table for the UnitOfWork contract tests (testkit.UnitOfWorkContract). Test-only: tests
-- apply it on top of schema/sqlite.sql, and it is not part of the product schema.

CREATE TABLE contract_probes (
  name TEXT NOT NULL PRIMARY KEY,
  n INTEGER NOT NULL DEFAULT 0
) STRICT;
