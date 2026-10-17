-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertInstanceRule :exec
INSERT INTO instance_rules (id, created_at) VALUES ($1, $2);

-- name: InsertInstanceRuleRevision :exec
INSERT INTO instance_rule_revisions (id, rule_id, position, revised_by_user_id, revised_at) VALUES ($1, $2, $3, $4, $5);

-- name: InsertInstanceRuleText :exec
INSERT INTO instance_rule_texts (rule_revision_id, locale, text, hint) VALUES ($1, $2, $3, $4);

-- Records a retirement; a rule that is already retired keeps its first one.
-- name: InsertInstanceRuleRetirement :execrows
INSERT INTO instance_rule_retirements (rule_id, retired_by_user_id, retired_at) VALUES ($1, $2, $3)
ON CONFLICT (rule_id) DO NOTHING;

-- The rule with its latest revision and its retirement (NULL retired_at when in effect).
-- name: GetInstanceRule :one
SELECT r.id, r.created_at, v.id AS revision_id, v.position, v.revised_by_user_id, v.revised_at,
  t.retired_by_user_id, t.retired_at
FROM instance_rules r
JOIN instance_rule_revisions v ON v.id = (SELECT MAX(x.id) FROM instance_rule_revisions x WHERE x.rule_id = r.id)
LEFT JOIN instance_rule_retirements t ON t.rule_id = r.id
WHERE r.id = $1;

-- The rules in effect by position, then ID.
-- name: ListInstanceRulesInEffect :many
SELECT r.id, r.created_at, v.id AS revision_id, v.position, v.revised_by_user_id, v.revised_at
FROM instance_rule_heads h
JOIN instance_rules r ON r.id = h.rule_id
JOIN instance_rule_revisions v ON v.id = h.revision_id
ORDER BY h.position, h.rule_id;

-- Every rule: those in effect by position, then the retired ones by position, each then by ID.
-- name: ListAllInstanceRules :many
SELECT r.id, r.created_at, v.id AS revision_id, v.position, v.revised_by_user_id, v.revised_at,
  t.retired_by_user_id, t.retired_at
FROM instance_rules r
JOIN instance_rule_revisions v ON v.id = (SELECT MAX(x.id) FROM instance_rule_revisions x WHERE x.rule_id = r.id)
LEFT JOIN instance_rule_retirements t ON t.rule_id = r.id
ORDER BY t.rule_id IS NOT NULL, v.position, r.id;

-- name: ListInstanceRuleTexts :many
SELECT rule_revision_id, locale, text, hint FROM instance_rule_texts
WHERE rule_revision_id = ANY(sqlc.arg(revision_ids)::text[])
ORDER BY rule_revision_id, locale;

-- name: GetInstanceRuleHead :one
SELECT rule_id, revision_id, position FROM instance_rule_heads WHERE rule_id = $1;

-- name: UpsertInstanceRuleHead :exec
INSERT INTO instance_rule_heads (rule_id, revision_id, position) VALUES ($1, $2, $3)
ON CONFLICT (rule_id) DO UPDATE SET revision_id = excluded.revision_id, position = excluded.position;

-- name: DeleteInstanceRuleHead :exec
DELETE FROM instance_rule_heads WHERE rule_id = $1;

-- name: DeleteAllInstanceRuleHeads :exec
DELETE FROM instance_rule_heads;

-- name: ListInstanceRuleHeads :many
SELECT rule_id, revision_id, position FROM instance_rule_heads ORDER BY position, rule_id;

-- name: ListInstanceRuleRevisionHeads :many
SELECT rule_id, id, position FROM instance_rule_revisions ORDER BY id;

-- name: ListRetiredInstanceRuleIDs :many
SELECT rule_id FROM instance_rule_retirements ORDER BY rule_id;
