-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Queries of the quotes of statuses (FR-114, ADR-0069, data-model section 4.4), owned by
-- internal/app/content.

-- InsertStatusQuote records the status that a status quotes; a status that has one keeps it.
-- name: InsertStatusQuote :execrows
INSERT INTO status_quotes (status_id, quoted_uri, form) VALUES (@status_id, @quoted_uri, @form)
ON CONFLICT (status_id) DO NOTHING;

-- name: GetStatusQuote :one
SELECT status_id, quoted_uri, form FROM status_quotes WHERE status_id = @status_id;

-- name: ListStatusQuotesAfter :many
SELECT status_id, quoted_uri, form FROM status_quotes
WHERE status_id > @after ORDER BY status_id LIMIT @page_limit;

-- name: InsertStatusQuoteEvent :exec
INSERT INTO status_quote_events (id, status_id, kind, authorization_uri, occurred_at)
VALUES (@id, @status_id, @kind, @authorization_uri, @occurred_at);

-- name: ListStatusQuoteEvents :many
SELECT id, status_id, kind, authorization_uri, occurred_at FROM status_quote_events
WHERE status_id = @status_id ORDER BY id;

-- name: UpsertStatusQuoteState :exec
INSERT INTO status_quote_states (status_id, state, authorization_uri) VALUES (@status_id, @state, @authorization_uri)
ON CONFLICT (status_id) DO UPDATE SET state = excluded.state, authorization_uri = excluded.authorization_uri;

-- name: GetStatusQuoteState :one
SELECT status_id, state, authorization_uri FROM status_quote_states WHERE status_id = @status_id;

-- name: ListStatusQuoteStatesByAuthorization :many
SELECT status_id, state, authorization_uri FROM status_quote_states
WHERE authorization_uri = @authorization_uri ORDER BY status_id;

-- name: ListAllStatusQuoteStates :many
SELECT status_id, state, authorization_uri FROM status_quote_states ORDER BY status_id;

-- name: DeleteAllStatusQuoteStates :exec
DELETE FROM status_quote_states;

-- name: PurgeStatusQuoteStates :exec
DELETE FROM status_quote_states WHERE status_id = ANY(sqlc.arg(ids)::text[]);

-- name: PurgeStatusQuoteEvents :exec
DELETE FROM status_quote_events WHERE status_id = ANY(sqlc.arg(ids)::text[]);

-- name: PurgeStatusQuotes :exec
DELETE FROM status_quotes WHERE status_id = ANY(sqlc.arg(ids)::text[]);
