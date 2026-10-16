-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertProbe :exec
INSERT INTO contract_probes (name) VALUES ($1);

-- name: ListProbeNames :many
SELECT name FROM contract_probes ORDER BY name;

-- name: IncrementProbe :execrows
UPDATE contract_probes SET n = n + 1 WHERE name = $1;

-- name: GetProbeCount :one
SELECT n FROM contract_probes WHERE name = $1;
