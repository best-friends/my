-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Probe table for the UnitOfWork contract tests (testkit.UnitOfWorkContract). Test-only: tests
-- apply it on top of schema/postgres.sql, and it is not part of the product schema.

CREATE TABLE contract_probes (
  name text COLLATE "C" NOT NULL PRIMARY KEY,
  n bigint NOT NULL DEFAULT 0
);
