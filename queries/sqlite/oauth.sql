-- SPDX-License-Identifier: AGPL-3.0-only

-- name: InsertOAuthClientIfAbsent :exec
INSERT INTO oauth_clients (id, client_id, client_secret_hash, name, website, scope, is_first_party, owner_user_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (client_id) DO NOTHING;

-- name: GetOAuthClientByClientID :one
SELECT id, client_id, client_secret_hash, name, website, scope, is_first_party, owner_user_id, created_at
FROM oauth_clients
WHERE client_id = ?;

-- name: ListOAuthClientRedirectURIs :many
SELECT redirect_uri FROM oauth_client_redirect_uris WHERE client_id = ? ORDER BY redirect_uri;

-- name: InsertOAuthClientRedirectURIIfAbsent :exec
INSERT INTO oauth_client_redirect_uris (client_id, redirect_uri) VALUES (?, ?)
ON CONFLICT (client_id, redirect_uri) DO NOTHING;

-- name: DeleteOAuthClientRedirectURI :exec
DELETE FROM oauth_client_redirect_uris WHERE client_id = ? AND redirect_uri = ?;

-- name: InsertAuthorizationCode :exec
INSERT INTO oauth_authorization_codes (code_hash, client_id, user_id, redirect_uri, scope, code_challenge, session_id, issued_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAuthorizationCode :one
SELECT code_hash, client_id, user_id, redirect_uri, scope, code_challenge, session_id, issued_at, expires_at
FROM oauth_authorization_codes
WHERE code_hash = ?;

-- name: InsertAuthorizationCodeRedemptionIfAbsent :execrows
INSERT INTO oauth_authorization_code_redemptions (code_hash, token_id, redeemed_at) VALUES (?, ?, ?)
ON CONFLICT (code_hash) DO NOTHING;

-- name: GetAuthorizationCodeRedemption :one
SELECT code_hash, token_id, redeemed_at FROM oauth_authorization_code_redemptions WHERE code_hash = ?;

-- name: InsertAccessToken :exec
INSERT INTO oauth_access_tokens (
  id, family_id, client_id, user_id, access_token_hash, refresh_token_hash, scope, rotated_from_token_id,
  issued_at, expires_at, refresh_expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAccessTokenByAccessHash :one
SELECT t.id, t.family_id, t.client_id, t.user_id, t.access_token_hash, t.refresh_token_hash, t.scope,
  t.rotated_from_token_id, t.issued_at, t.expires_at, t.refresh_expires_at,
  EXISTS (SELECT 1 FROM oauth_token_revocations r WHERE r.token_id = t.id) AS revoked
FROM oauth_access_tokens t
WHERE t.access_token_hash = ?;

-- name: GetAccessTokenByRefreshHash :one
SELECT t.id, t.family_id, t.client_id, t.user_id, t.access_token_hash, t.refresh_token_hash, t.scope,
  t.rotated_from_token_id, t.issued_at, t.expires_at, t.refresh_expires_at,
  EXISTS (SELECT 1 FROM oauth_token_revocations r WHERE r.token_id = t.id) AS revoked
FROM oauth_access_tokens t
WHERE t.refresh_token_hash = ?;

-- name: InsertTokenRevocationIfAbsent :execrows
INSERT INTO oauth_token_revocations (token_id, reason, revoked_at) VALUES (?, ?, ?)
ON CONFLICT (token_id) DO NOTHING;

-- name: RevokeTokenFamily :exec
INSERT INTO oauth_token_revocations (token_id, reason, revoked_at)
SELECT t.id, sqlc.arg(reason), sqlc.arg(revoked_at) FROM oauth_access_tokens t WHERE t.family_id = sqlc.arg(family_id)
ON CONFLICT (token_id) DO NOTHING;

-- name: RevokeSessionOfTokenFamily :exec
INSERT INTO session_revocations (session_id, reason, revoked_at)
SELECT c.session_id, sqlc.arg(reason), sqlc.arg(revoked_at)
FROM oauth_authorization_code_redemptions r
JOIN oauth_authorization_codes c ON c.code_hash = r.code_hash
WHERE r.token_id = sqlc.arg(family_id) AND c.session_id IS NOT NULL
ON CONFLICT (session_id) DO NOTHING;

-- RevokeTokensOfUser revokes every token of a user whose access or refresh token has not expired
-- at revoked_at and that has no revocation yet.
-- name: RevokeTokensOfUser :exec
INSERT INTO oauth_token_revocations (token_id, reason, revoked_at)
SELECT t.id, sqlc.arg(reason), sqlc.arg(revoked_at) FROM oauth_access_tokens t
WHERE t.user_id = sqlc.arg(user_id)
  AND (t.expires_at > sqlc.arg(revoked_at) OR t.refresh_expires_at > sqlc.arg(revoked_at))
ON CONFLICT (token_id) DO NOTHING;

-- RevokeOtherTokensOfUser revokes every token of a user outside the family keep_family_id whose
-- access or refresh token has not expired at revoked_at and that has no revocation yet.
-- name: RevokeOtherTokensOfUser :exec
INSERT INTO oauth_token_revocations (token_id, reason, revoked_at)
SELECT t.id, sqlc.arg(reason), sqlc.arg(revoked_at) FROM oauth_access_tokens t
WHERE t.user_id = sqlc.arg(user_id) AND t.family_id <> sqlc.arg(keep_family_id)
  AND (t.expires_at > sqlc.arg(revoked_at) OR t.refresh_expires_at > sqlc.arg(revoked_at))
ON CONFLICT (token_id) DO NOTHING;

-- TokenFamilyActive reports whether a token of the family has no revocation and an access or
-- refresh token that has not expired at now: whether the grant can still be used or refreshed.
-- name: TokenFamilyActive :one
SELECT EXISTS (
  SELECT 1 FROM oauth_access_tokens t
  WHERE t.family_id = sqlc.arg(family_id)
    AND (t.expires_at > sqlc.arg(now) OR t.refresh_expires_at > sqlc.arg(now))
    AND NOT EXISTS (SELECT 1 FROM oauth_token_revocations r WHERE r.token_id = t.id)
) AS active;
