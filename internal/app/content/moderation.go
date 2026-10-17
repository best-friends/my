// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// The effects on statuses of the moderation actions on statuses (data-model §5.2), which the
// moderation module records inside the transaction of the action.

// DeleteByModeration records inside tx that the moderation action actionID deleted st: the
// deletion (origin moderation, naming the action), which stays as the tombstone of its URI, and
// the removal from every projection, the same as the author's deletion; a local status's Delete
// goes out like the author's (federation.md §11.1). A status that is already deleted fails with
// ErrAlreadyDeleted, which abandons tx.
func (s *Service) DeleteByModeration(ctx context.Context, tx uow.Tx, st Status, actionID string, at time.Time) error {
	d := content.Deletion{
		StatusID: st.ID, URI: st.URI, AuthorAccountID: st.AuthorAccountID, Origin: content.DeletedByModeration,
		ModerationActionID: actionID, DeletedAt: at,
	}
	if err := s.d.Repository.InsertDeletion(ctx, tx, d); err != nil {
		return err
	}
	return tx.Emit(ctx, StatusDeleted{Deletion: d, Status: st})
}

// MarkSensitiveByModeration records inside tx that the mark_statuses_sensitive action actionID
// shows a status as sensitive until it is lifted (status_sensitivity_overrides).
func (s *Service) MarkSensitiveByModeration(ctx context.Context, tx uow.Tx, statusID, actionID string) error {
	return s.d.Repository.InsertSensitivityOverride(ctx, tx, statusID, actionID)
}

// ClearSensitivityOf ends inside tx what a lifted mark_statuses_sensitive action showed as
// sensitive; the statuses that other actions mark stay marked.
func (s *Service) ClearSensitivityOf(ctx context.Context, tx uow.Tx, actionID string) error {
	return s.d.Repository.DeleteSensitivityOverridesOf(ctx, tx, actionID)
}
