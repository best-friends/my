-- SPDX-License-Identifier: AGPL-3.0-only

-- The instance's costs (data-model 4.12, ADR-0067). Cost items, label and price revisions, end
-- changes, and cancellations are written once; cost_item_heads is the projection. Dates are
-- TEXT as YYYY-MM-DD.

-- name: InsertCostItem :exec
INSERT INTO cost_items (id, schedule, interval, starts_on, created_by_user_id, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: InsertCostItemLabelRevision :exec
INSERT INTO cost_item_label_revisions (
  id, cost_item_id, name, provider, provider_normalized, category, memo, revised_by_user_id, revised_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertCostItemPriceRevision :exec
INSERT INTO cost_item_price_revisions (id, cost_item_id, effective_on, amount_minor, currency, revised_by_user_id, revised_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: InsertCostItemEndChange :exec
INSERT INTO cost_item_end_changes (id, cost_item_id, ends_on, changed_by_user_id, changed_at) VALUES (?, ?, ?, ?, ?);

-- name: InsertCostItemCancellation :exec
INSERT INTO cost_item_cancellations (id, cost_item_id, canceled_by_user_id, canceled_at) VALUES (?, ?, ?, ?);

-- name: UpsertCostItemHead :exec
INSERT INTO cost_item_heads (cost_item_id, label_revision_id, ends_on) VALUES (?, ?, ?)
ON CONFLICT (cost_item_id) DO UPDATE SET label_revision_id = excluded.label_revision_id, ends_on = excluded.ends_on;

-- name: DeleteCostItemHead :exec
DELETE FROM cost_item_heads WHERE cost_item_id = ?;

-- name: GetCostItemCurrent :one
SELECT h.cost_item_id, h.label_revision_id, h.ends_on, i.starts_on
FROM cost_item_heads h JOIN cost_items i ON i.id = h.cost_item_id
WHERE h.cost_item_id = ?;

-- ListCostItems returns up to lim cost items that are not canceled, newest first, with their
-- current label and end: only the item id unless it is empty, and only those before before
-- unless it is empty.
-- name: ListCostItems :many
SELECT i.id, i.schedule, i.interval, i.starts_on, i.created_by_user_id, i.created_at, h.ends_on,
  l.name, l.provider, l.provider_normalized, l.category, l.memo
FROM cost_item_heads h
JOIN cost_items i ON i.id = h.cost_item_id
JOIN cost_item_label_revisions l ON l.id = h.label_revision_id
WHERE (CAST(sqlc.arg(id) AS TEXT) = '' OR h.cost_item_id = sqlc.arg(id))
  AND (CAST(sqlc.arg(before) AS TEXT) = '' OR h.cost_item_id < sqlc.arg(before))
ORDER BY h.cost_item_id DESC
LIMIT sqlc.arg(lim);

-- ListCostItemPriceRevisions returns the price revisions of the cost items ids (a JSON array of
-- strings) in the order they were recorded.
-- name: ListCostItemPriceRevisions :many
SELECT id, cost_item_id, effective_on, amount_minor, currency, revised_at FROM cost_item_price_revisions
WHERE cost_item_id IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT)))
ORDER BY cost_item_id, id;
