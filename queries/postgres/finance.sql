-- SPDX-License-Identifier: AGPL-3.0-only

-- The instance's costs (data-model 4.12, ADR-0067). Cost items, label and price revisions, end
-- changes, and cancellations are written once; cost_item_heads is the projection.

-- name: InsertCostItem :exec
INSERT INTO cost_items (id, schedule, interval, starts_on, created_by_user_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertCostItemLabelRevision :exec
INSERT INTO cost_item_label_revisions (
  id, cost_item_id, name, provider, provider_normalized, category, memo, revised_by_user_id, revised_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: InsertCostItemPriceRevision :exec
INSERT INTO cost_item_price_revisions (id, cost_item_id, effective_on, amount_minor, currency, revised_by_user_id, revised_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertCostItemEndChange :exec
INSERT INTO cost_item_end_changes (id, cost_item_id, ends_on, changed_by_user_id, changed_at) VALUES ($1, $2, $3, $4, $5);

-- name: InsertCostItemCancellation :exec
INSERT INTO cost_item_cancellations (id, cost_item_id, canceled_by_user_id, canceled_at) VALUES ($1, $2, $3, $4);

-- name: UpsertCostItemHead :exec
INSERT INTO cost_item_heads (cost_item_id, label_revision_id, ends_on) VALUES ($1, $2, $3)
ON CONFLICT (cost_item_id) DO UPDATE SET label_revision_id = excluded.label_revision_id, ends_on = excluded.ends_on;

-- name: DeleteCostItemHead :exec
DELETE FROM cost_item_heads WHERE cost_item_id = $1;

-- name: GetCostItemCurrentForUpdate :one
SELECT h.cost_item_id, h.label_revision_id, h.ends_on, i.starts_on, l.name
FROM cost_item_heads h JOIN cost_items i ON i.id = h.cost_item_id
  JOIN cost_item_label_revisions l ON l.id = h.label_revision_id
WHERE h.cost_item_id = $1
FOR UPDATE OF h;

-- ListCostItems returns up to lim cost items that are not canceled, newest first, with their
-- current label and end: only the item id unless it is empty, and only those before before
-- unless it is empty.
-- name: ListCostItems :many
SELECT i.id, i.schedule, i.interval, i.starts_on, i.created_by_user_id, i.created_at, h.ends_on,
  l.name, l.provider, l.provider_normalized, l.category, l.memo
FROM cost_item_heads h
JOIN cost_items i ON i.id = h.cost_item_id
JOIN cost_item_label_revisions l ON l.id = h.label_revision_id
WHERE (sqlc.arg(id)::text = '' OR h.cost_item_id = sqlc.arg(id))
  AND (sqlc.arg(before)::text = '' OR h.cost_item_id < sqlc.arg(before))
ORDER BY h.cost_item_id DESC
LIMIT sqlc.arg(lim);

-- ListCostItemPriceRevisions returns the price revisions of the cost items ids in the order they
-- were recorded.
-- name: ListCostItemPriceRevisions :many
SELECT id, cost_item_id, effective_on, amount_minor, currency, revised_at FROM cost_item_price_revisions
WHERE cost_item_id = ANY(sqlc.arg(ids)::text[])
ORDER BY cost_item_id, id;

-- The billing imports (ADR-0068 section 5). billing_import_runs and imported_cost_revisions are
-- written once; imported_cost_heads is the projection of the revisions.

-- name: InsertBillingImportRun :exec
INSERT INTO billing_import_runs (id, source, run_key, outcome, error_kind, requests, bytes_billed, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- SumBillingImportRuns sums the requests and the bytes billed of the attempts of a source that
-- occurred in [from, until).
-- name: SumBillingImportRuns :one
SELECT COALESCE(SUM(requests), 0)::bigint AS requests, COALESCE(SUM(bytes_billed), 0)::bigint AS bytes_billed
FROM billing_import_runs
WHERE source = sqlc.arg(source) AND occurred_at >= sqlc.arg(from_at) AND occurred_at < sqlc.arg(until_at);

-- name: CountBillingImportRunsByKey :one
SELECT COUNT(*) FROM billing_import_runs WHERE source = $1 AND run_key = $2;

-- LastBillingImportRun returns the newest attempt of a source, only among the successes unless
-- include_failed is true.
-- name: LastBillingImportRun :one
SELECT id, source, run_key, outcome, error_kind, requests, bytes_billed, occurred_at FROM billing_import_runs
WHERE source = sqlc.arg(source) AND (sqlc.arg(include_failed)::boolean OR outcome = 'succeeded')
ORDER BY occurred_at DESC, id DESC
LIMIT 1;

-- InsertImportedCostRevision writes nothing when an attempt of the same job wrote the key.
-- name: InsertImportedCostRevision :execrows
INSERT INTO imported_cost_revisions (
  id, source, billing_month, service, charge_type, category, starts_on, ends_on, amount_minor, currency, kind,
  run_id, run_key, recorded_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (source, billing_month, service, charge_type, run_key) DO NOTHING;

-- name: UpsertImportedCostHead :exec
INSERT INTO imported_cost_heads (source, billing_month, service, charge_type, revision_id) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (source, billing_month, service, charge_type) DO UPDATE SET revision_id = excluded.revision_id;

-- ListImportedCostHeads returns the newest revision of every key, of one source unless source is
-- empty and of the billing months months unless it is empty.
-- name: ListImportedCostHeads :many
SELECT r.id, r.source, r.billing_month, r.service, r.charge_type, r.category, r.starts_on, r.ends_on, r.amount_minor,
  r.currency, r.kind, r.run_id, r.run_key, r.recorded_at
FROM imported_cost_heads h JOIN imported_cost_revisions r ON r.id = h.revision_id
WHERE (sqlc.arg(source)::text = '' OR h.source = sqlc.arg(source))
  AND (cardinality(sqlc.arg(months)::text[]) = 0 OR h.billing_month = ANY(sqlc.arg(months)::text[]))
ORDER BY h.source, h.billing_month, h.service, h.charge_type;

-- LockImportedCostHeads locks the heads of a source's billing months, so that two imports of
-- the source compare and write one after the other.
-- name: LockImportedCostHeads :exec
SELECT 1 FROM imported_cost_heads
WHERE source = sqlc.arg(source) AND billing_month = ANY(sqlc.arg(months)::text[])
FOR UPDATE;
