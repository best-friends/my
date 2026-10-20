// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"slices"
	"time"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// EnsureOAuthClient implements identity.Repository.
func (r *IdentityRepository) EnsureOAuthClient(ctx context.Context, tx uow.Tx, c identity.OAuthClient) (identity.OAuthClient, error) {
	q, err := queries(tx)
	if err != nil {
		return identity.OAuthClient{}, err
	}
	err = q.InsertOAuthClientIfAbsent(ctx, postgresgen.InsertOAuthClientIfAbsentParams{
		ID: c.ID, ClientID: c.ClientID, ClientSecretHash: c.SecretHash, Name: c.Name, Website: toNullString(c.Website),
		Scope: string(c.Scope), IsFirstParty: c.FirstParty, OwnerUserID: toNullString(c.OwnerUserID),
		CreatedAt: c.CreatedAt,
	})
	if err != nil {
		return identity.OAuthClient{}, wrap(err, "insert oauth client")
	}
	row, err := q.GetOAuthClientByClientID(ctx, c.ClientID)
	if err != nil {
		return identity.OAuthClient{}, wrap(err, "get oauth client")
	}
	return oauthClientFromRow(row), nil
}

// ReplaceOAuthClientRedirectURIs implements identity.Repository.
func (r *IdentityRepository) ReplaceOAuthClientRedirectURIs(ctx context.Context, tx uow.Tx, clientID string, uris []string) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	current, err := q.ListOAuthClientRedirectURIs(ctx, clientID)
	if err != nil {
		return wrap(err, "list oauth client redirect uris")
	}
	for _, u := range current {
		if slices.Contains(uris, u) {
			continue
		}
		err := q.DeleteOAuthClientRedirectURI(ctx, postgresgen.DeleteOAuthClientRedirectURIParams{ClientID: clientID, RedirectUri: u})
		if err != nil {
			return wrap(err, "delete oauth client redirect uri")
		}
	}
	for _, u := range uris {
		err := q.InsertOAuthClientRedirectURIIfAbsent(ctx, postgresgen.InsertOAuthClientRedirectURIIfAbsentParams{ClientID: clientID, RedirectUri: u})
		if err != nil {
			return wrap(err, "insert oauth client redirect uri")
		}
	}
	return nil
}

// InsertAuthorizationCode implements identity.Repository.
func (r *IdentityRepository) InsertAuthorizationCode(ctx context.Context, tx uow.Tx, c identity.AuthorizationCode) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertAuthorizationCode(ctx, postgresgen.InsertAuthorizationCodeParams{
		CodeHash: c.CodeHash, ClientID: c.ClientID, UserID: c.UserID, RedirectUri: c.RedirectURI, Scope: string(c.Scope),
		CodeChallenge: c.CodeChallenge, SessionID: toNullString(c.SessionID), IssuedAt: c.IssuedAt, ExpiresAt: c.ExpiresAt,
	})
	return wrap(err, "insert authorization code")
}

// InsertAuthorizationCodeRedemption implements identity.Repository.
func (r *IdentityRepository) InsertAuthorizationCodeRedemption(ctx context.Context, tx uow.Tx, red identity.AuthorizationCodeRedemption) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertAuthorizationCodeRedemptionIfAbsent(ctx, postgresgen.InsertAuthorizationCodeRedemptionIfAbsentParams{
		CodeHash: red.CodeHash, TokenID: red.TokenID, RedeemedAt: red.RedeemedAt,
	})
	return n == 1, wrap(err, "insert authorization code redemption")
}

// InsertAccessToken implements identity.Repository.
func (r *IdentityRepository) InsertAccessToken(ctx context.Context, tx uow.Tx, t identity.AccessToken) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	var refreshExpires *time.Time
	if t.RefreshTokenHash != nil {
		refreshExpires = &t.RefreshExpiresAt
	}
	err = q.InsertAccessToken(ctx, postgresgen.InsertAccessTokenParams{
		ID: t.ID, FamilyID: t.FamilyID, ClientID: t.ClientID, UserID: t.UserID,
		AccessTokenHash: t.AccessTokenHash, RefreshTokenHash: t.RefreshTokenHash, Scope: string(t.Scope),
		RotatedFromTokenID: toNullString(t.RotatedFromTokenID), IssuedAt: t.IssuedAt,
		ExpiresAt: t.ExpiresAt, RefreshExpiresAt: refreshExpires,
	})
	return wrap(err, "insert access token")
}

// InsertTokenRevocation implements identity.Repository.
func (r *IdentityRepository) InsertTokenRevocation(ctx context.Context, tx uow.Tx, rev identity.TokenRevocation) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertTokenRevocationIfAbsent(ctx, postgresgen.InsertTokenRevocationIfAbsentParams{
		TokenID: rev.TokenID, Reason: string(rev.Reason), RevokedAt: rev.RevokedAt,
	})
	return n == 1, wrap(err, "insert token revocation")
}

// RevokeTokenFamily implements identity.Repository.
func (r *IdentityRepository) RevokeTokenFamily(ctx context.Context, tx uow.Tx, familyID string, reason identity.TokenRevocationReason, at time.Time) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.RevokeTokenFamily(ctx, postgresgen.RevokeTokenFamilyParams{Reason: string(reason), RevokedAt: at, FamilyID: familyID})
	return wrap(err, "revoke token family")
}

// TokenFamilyActive implements identity.Repository.
func (r *IdentityRepository) TokenFamilyActive(ctx context.Context, familyID string, now time.Time) (bool, error) {
	active, err := r.db.reader().TokenFamilyActive(ctx, postgresgen.TokenFamilyActiveParams{FamilyID: familyID, Now: now})
	return active, wrap(err, "check token family")
}

// RevokeSessionOfTokenFamily implements identity.Repository.
func (r *IdentityRepository) RevokeSessionOfTokenFamily(ctx context.Context, tx uow.Tx, familyID string, reason identity.RevocationReason, at time.Time) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.RevokeSessionOfTokenFamily(ctx, postgresgen.RevokeSessionOfTokenFamilyParams{Reason: string(reason), RevokedAt: at, FamilyID: familyID})
	return wrap(err, "revoke the session of a token family")
}

// FindOAuthClient implements identity.Repository.
func (r *IdentityRepository) FindOAuthClient(ctx context.Context, clientID string) (identity.OAuthClient, []string, error) {
	q := r.db.reader()
	row, err := q.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		return identity.OAuthClient{}, nil, notFound(err, appidentity.ErrNotFound, "oauth client")
	}
	uris, err := q.ListOAuthClientRedirectURIs(ctx, row.ID)
	if err != nil {
		return identity.OAuthClient{}, nil, wrap(err, "list oauth client redirect uris")
	}
	return oauthClientFromRow(row), uris, nil
}

// FindAuthorizationCode implements identity.Repository.
func (r *IdentityRepository) FindAuthorizationCode(ctx context.Context, codeHash []byte) (identity.AuthorizationCode, error) {
	row, err := r.db.reader().GetAuthorizationCode(ctx, codeHash)
	if err != nil {
		return identity.AuthorizationCode{}, notFound(err, appidentity.ErrNotFound, "authorization code")
	}
	return identity.AuthorizationCode{
		CodeHash: row.CodeHash, ClientID: row.ClientID, UserID: row.UserID, RedirectURI: row.RedirectUri,
		Scope: identity.Scope(row.Scope), CodeChallenge: row.CodeChallenge, SessionID: fromNullString(row.SessionID),
		IssuedAt: utc(row.IssuedAt), ExpiresAt: utc(row.ExpiresAt),
	}, nil
}

// FindAuthorizationCodeRedemption implements identity.Repository.
func (r *IdentityRepository) FindAuthorizationCodeRedemption(ctx context.Context, codeHash []byte) (identity.AuthorizationCodeRedemption, error) {
	row, err := r.db.reader().GetAuthorizationCodeRedemption(ctx, codeHash)
	if err != nil {
		return identity.AuthorizationCodeRedemption{}, notFound(err, appidentity.ErrNotFound, "authorization code redemption")
	}
	return identity.AuthorizationCodeRedemption{CodeHash: row.CodeHash, TokenID: row.TokenID, RedeemedAt: utc(row.RedeemedAt)}, nil
}

// FindAccessTokenByAccessHash implements identity.Repository.
func (r *IdentityRepository) FindAccessTokenByAccessHash(ctx context.Context, hash []byte) (identity.AccessToken, bool, error) {
	row, err := r.db.reader().GetAccessTokenByAccessHash(ctx, hash)
	if err != nil {
		return identity.AccessToken{}, false, notFound(err, appidentity.ErrNotFound, "access token")
	}
	return accessTokenFromRow(row), row.Revoked, nil
}

// FindAccessTokenByRefreshHash implements identity.Repository.
func (r *IdentityRepository) FindAccessTokenByRefreshHash(ctx context.Context, hash []byte) (identity.AccessToken, bool, error) {
	row, err := r.db.reader().GetAccessTokenByRefreshHash(ctx, hash)
	if err != nil {
		return identity.AccessToken{}, false, notFound(err, appidentity.ErrNotFound, "access token")
	}
	return accessTokenFromRow(postgresgen.GetAccessTokenByAccessHashRow(row)), row.Revoked, nil
}

func oauthClientFromRow(row postgresgen.OauthClient) identity.OAuthClient {
	return identity.OAuthClient{
		ID: row.ID, ClientID: row.ClientID, SecretHash: row.ClientSecretHash, Name: row.Name, Website: fromNullString(row.Website),
		Scope: identity.Scope(row.Scope), FirstParty: row.IsFirstParty, OwnerUserID: fromNullString(row.OwnerUserID),
		CreatedAt: utc(row.CreatedAt),
	}
}

func accessTokenFromRow(row postgresgen.GetAccessTokenByAccessHashRow) identity.AccessToken {
	t := identity.AccessToken{
		ID: row.ID, FamilyID: row.FamilyID, ClientID: row.ClientID, UserID: row.UserID,
		AccessTokenHash: row.AccessTokenHash, RefreshTokenHash: row.RefreshTokenHash, Scope: identity.Scope(row.Scope),
		RotatedFromTokenID: fromNullString(row.RotatedFromTokenID), IssuedAt: utc(row.IssuedAt), ExpiresAt: utc(row.ExpiresAt),
	}
	if row.RefreshExpiresAt != nil {
		t.RefreshExpiresAt = utc(*row.RefreshExpiresAt)
	}
	return t
}

// RevokeTokensOfUser implements identity.Repository.
func (r *IdentityRepository) RevokeTokensOfUser(ctx context.Context, tx uow.Tx, userID string, reason identity.TokenRevocationReason, at time.Time) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.RevokeTokensOfUser(ctx, postgresgen.RevokeTokensOfUserParams{Reason: string(reason), RevokedAt: at, UserID: userID})
	return wrap(err, "revoke the tokens of a user")
}
