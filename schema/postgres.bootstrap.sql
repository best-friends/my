-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Idempotent setup that runs before Atlas applies schema/postgres.sql, on both the target
-- and the dev database (ADR-0023). It holds what Atlas Community Edition cannot manage.
-- Every statement must be safe to run repeatedly.

-- Trigram indexes for search (ADR-0009).
CREATE EXTENSION IF NOT EXISTS pg_trgm;
