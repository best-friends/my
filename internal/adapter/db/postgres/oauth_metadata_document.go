// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"time"

	"github.com/mycroblog/my/internal/adapter/db/postgres/postgresgen"
	appidentity "github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/app/uow"
	"github.com/mycroblog/my/internal/domain/identity"
)

// UpsertOAuthClientMetadataDocument implements identity.Repository.
func (r *IdentityRepository) UpsertOAuthClientMetadataDocument(ctx context.Context, tx uow.Tx, d identity.ClientMetadataDocument) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.UpsertOAuthClientMetadataDocument(ctx, postgresgen.UpsertOAuthClientMetadataDocumentParams{
		ClientID: d.ClientID, ClientName: d.Name, ClientUri: d.Website, LogoUri: d.LogoURI, Scope: string(d.Scope),
		FetchedAt: d.FetchedAt, ExpiresAt: d.ExpiresAt,
	})
	return wrap(err, "upsert oauth client metadata document")
}

// FindOAuthClientMetadataDocument implements identity.Repository.
func (r *IdentityRepository) FindOAuthClientMetadataDocument(ctx context.Context, clientID string) (identity.ClientMetadataDocument, error) {
	row, err := r.db.reader().GetOAuthClientMetadataDocument(ctx, clientID)
	if err != nil {
		return identity.ClientMetadataDocument{}, notFound(err, appidentity.ErrNotFound, "oauth client metadata document")
	}
	return identity.ClientMetadataDocument{
		ClientID: row.ClientID, Name: row.ClientName, Website: row.ClientUri, LogoURI: row.LogoUri, Scope: identity.Scope(row.Scope),
		FetchedAt: row.FetchedAt, ExpiresAt: row.ExpiresAt,
	}, nil
}

// InsertOAuthClientHostDenial implements identity.Repository.
func (r *IdentityRepository) InsertOAuthClientHostDenial(ctx context.Context, tx uow.Tx, d identity.OAuthClientHostDenial) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertOAuthClientHostDenialIfAllowed(ctx, postgresgen.InsertOAuthClientHostDenialIfAllowedParams{
		ID: d.ID, Host: d.Host, DeniedByUserID: toNullString(d.DeniedByUserID), DeniedAt: d.DeniedAt,
	})
	return n == 1, wrap(err, "insert oauth client host denial")
}

// LiftOAuthClientHostDenials implements identity.Repository.
func (r *IdentityRepository) LiftOAuthClientHostDenials(ctx context.Context, tx uow.Tx, host, byUserID string, at time.Time) (bool, error) {
	q, err := queries(tx)
	if err != nil {
		return false, err
	}
	n, err := q.LiftOAuthClientHostDenials(ctx, postgresgen.LiftOAuthClientHostDenialsParams{
		LiftedByUserID: toNullString(byUserID), LiftedAt: at, Host: host,
	})
	return n > 0, wrap(err, "lift oauth client host denials")
}

// OAuthClientHostDenied implements identity.Repository.
func (r *IdentityRepository) OAuthClientHostDenied(ctx context.Context, host string) (bool, error) {
	denied, err := r.db.reader().OAuthClientHostDenied(ctx, host)
	return denied, wrap(err, "check oauth client host denial")
}
