// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/moderation"
)

var _ appmoderation.AuditRepository = (*ModerationRepository)(nil)

// InsertAuditEntry implements moderation.AuditRepository.
func (r *ModerationRepository) InsertAuditEntry(ctx context.Context, tx uow.Tx, e moderation.AuditEntry) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.InsertAuditLogEntry(ctx, postgresgen.InsertAuditLogEntryParams{
		ID: e.ID, ActorKind: string(e.ActorKind), ActorUserID: toNullString(e.ActorUserID), Action: string(e.Action),
		TargetKind: string(e.TargetKind), TargetID: e.TargetID, TargetLabel: e.TargetLabel,
		SourceEventID: toNullString(e.SourceEventID), OccurredAt: e.OccurredAt,
	}), "insert audit log entry")
}

// ListAuditEntries implements moderation.AuditRepository.
func (r *ModerationRepository) ListAuditEntries(ctx context.Context, q appmoderation.AuditQuery, limit int) ([]moderation.AuditEntry, error) {
	lim, err := toInt32(limit, "limit")
	if err != nil {
		return nil, err
	}
	rows, err := r.db.reader().ListAuditLogEntries(ctx, postgresgen.ListAuditLogEntriesParams{
		ActorUserID: q.ActorUserID, Action: string(q.Action), TargetKind: string(q.TargetKind), TargetID: q.TargetID,
		Before: q.Before, Lim: lim,
	})
	if err != nil {
		return nil, wrap(err, "list audit log entries")
	}
	out := make([]moderation.AuditEntry, len(rows))
	for i, row := range rows {
		out[i] = moderation.AuditEntry{
			ID: row.ID, ActorKind: moderation.AuditActorKind(row.ActorKind), ActorUserID: fromNullString(row.ActorUserID),
			Action: moderation.AuditAction(row.Action), TargetKind: moderation.AuditTargetKind(row.TargetKind),
			TargetID: row.TargetID, TargetLabel: row.TargetLabel, SourceEventID: fromNullString(row.SourceEventID),
			OccurredAt: row.OccurredAt.UTC(),
		}
	}
	return out, nil
}
