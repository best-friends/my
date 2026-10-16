// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
)

// AccountRepository implements account.Repository on PostgreSQL.
type AccountRepository struct {
	db *DB
}

var _ appaccount.Repository = (*AccountRepository)(nil)

// AccountRepository returns the account module's repository on d.
func (d *DB) AccountRepository() *AccountRepository { return &AccountRepository{db: d} }

// InsertAccount implements account.Repository.
func (r *AccountRepository) InsertAccount(ctx context.Context, tx uow.Tx, a account.Account) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertAccount(ctx, postgresgen.InsertAccountParams{
		ID: a.ID, Uri: a.URI, Domain: a.Domain, ActorType: string(a.ActorType), CreatedAt: a.CreatedAt,
	})
	if isUniqueViolation(err, "accounts_instance_actor") {
		return fmt.Errorf("insert account %s: %w", a.ID, appaccount.ErrInstanceActorExists)
	}
	return wrap(err, "insert account")
}

// InsertLocalAccount implements account.Repository.
func (r *AccountRepository) InsertLocalAccount(ctx context.Context, tx uow.Tx, la account.LocalAccount) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertLocalAccount(ctx, postgresgen.InsertLocalAccountParams{
		AccountID: la.AccountID, UserID: la.UserID, Username: la.Username, UsernameNormalized: la.UsernameNormalized,
	})
	if isUniqueViolation(err, "local_accounts_username_normalized") {
		return fmt.Errorf("insert local account %q: %w", la.Username, appaccount.ErrUsernameTaken)
	}
	return wrap(err, "insert local account")
}

// InsertProfileRevision implements account.Repository.
func (r *AccountRepository) InsertProfileRevision(ctx context.Context, tx uow.Tx, rev account.ProfileRevision) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	seq, err := toInt32(rev.Seq, "profile revision seq")
	if err != nil {
		return err
	}
	err = q.InsertProfileRevision(ctx, postgresgen.InsertProfileRevisionParams{
		ID:            rev.ID,
		AccountID:     rev.AccountID,
		Seq:           seq,
		DisplayName:   rev.DisplayName,
		NoteHtml:      rev.NoteHTML,
		Locked:        rev.Locked,
		Bot:           rev.Bot,
		Discoverable:  rev.Discoverable,
		Indexable:     rev.Indexable,
		AvatarMediaID: toNullString(rev.AvatarMediaID),
		HeaderMediaID: toNullString(rev.HeaderMediaID),
		RevisedAt:     rev.RevisedAt,
	})
	if isUniqueViolation(err, "account_profile_revisions_account_id_seq") {
		// A concurrent revision took this seq after the command read the head at READ COMMITTED.
		// A new attempt reads the new head and takes the next seq.
		return uow.MarkRetryable(wrap(err, "insert profile revision"))
	}
	if err != nil {
		return wrap(err, "insert profile revision")
	}
	for _, f := range rev.Fields {
		pos, err := toInt32(f.Position, "profile field position")
		if err != nil {
			return err
		}
		err = q.InsertProfileField(ctx, postgresgen.InsertProfileFieldParams{
			RevisionID: rev.ID, Position: pos, Name: f.Name, ValueHtml: f.ValueHTML,
		})
		if err != nil {
			return wrap(err, "insert profile field")
		}
	}
	return nil
}

// InsertKey implements account.Repository.
func (r *AccountRepository) InsertKey(ctx context.Context, tx uow.Tx, k account.Key, sealedPrivateKey []byte) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertAccountKey(ctx, postgresgen.InsertAccountKeyParams{
		ID: k.ID, AccountID: k.AccountID, KeyID: k.KeyID, Algorithm: string(k.Algorithm),
		PublicKeyPem: k.PublicKeyPEM, CreatedAt: k.CreatedAt,
	})
	if err != nil {
		return wrap(err, "insert account key")
	}
	if sealedPrivateKey == nil {
		return nil
	}
	err = q.InsertLocalAccountPrivateKey(ctx, postgresgen.InsertLocalAccountPrivateKeyParams{
		AccountKeyID: k.ID, PrivateKeyEnc: sealedPrivateKey,
	})
	return wrap(err, "insert private key")
}

// InsertLifecycleEvent implements account.Repository.
func (r *AccountRepository) InsertLifecycleEvent(ctx context.Context, tx uow.Tx, e account.LifecycleEvent) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.InsertLifecycleEvent(ctx, postgresgen.InsertLifecycleEventParams{
		ID:                 e.ID,
		AccountID:          e.AccountID,
		Kind:               string(e.Kind),
		InitiatedBy:        string(e.InitiatedBy),
		ModerationActionID: toNullString(e.ModerationActionID),
		DeletionDueAt:      e.DeletionDueAt,
		OccurredAt:         e.OccurredAt,
	})
	return wrap(err, "insert lifecycle event")
}

// UpsertHandle implements account.Repository.
func (r *AccountRepository) UpsertHandle(ctx context.Context, tx uow.Tx, h account.Handle) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.UpsertAccountHandle(ctx, postgresgen.UpsertAccountHandleParams(h)), "upsert account handle")
}

// DeleteAllHandles implements account.Repository.
func (r *AccountRepository) DeleteAllHandles(ctx context.Context, tx uow.Tx) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.DeleteAllAccountHandles(ctx), "delete account handles")
}

// ProfileHead implements account.Repository.
func (r *AccountRepository) ProfileHead(ctx context.Context, tx uow.Tx, accountID string) (account.ProfileHead, error) {
	q, err := queries(tx)
	if err != nil {
		return account.ProfileHead{}, err
	}
	row, err := q.GetProfileHead(ctx, accountID)
	if err != nil {
		return account.ProfileHead{}, notFound(err, appaccount.ErrNotFound, "profile head of "+accountID)
	}
	return account.ProfileHead{AccountID: row.AccountID, RevisionID: row.RevisionID, Seq: int(row.Seq)}, nil
}

// UpsertProfileHead implements account.Repository.
func (r *AccountRepository) UpsertProfileHead(ctx context.Context, tx uow.Tx, h account.ProfileHead) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.UpsertProfileHead(ctx, postgresgen.UpsertProfileHeadParams{AccountID: h.AccountID, RevisionID: h.RevisionID})
	return wrap(err, "upsert profile head")
}

// DeleteAllProfileHeads implements account.Repository.
func (r *AccountRepository) DeleteAllProfileHeads(ctx context.Context, tx uow.Tx) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.DeleteAllProfileHeads(ctx), "delete profile heads")
}

// LifecycleState implements account.Repository.
func (r *AccountRepository) LifecycleState(ctx context.Context, tx uow.Tx, accountID string) (account.LifecycleState, error) {
	q, err := queries(tx)
	if err != nil {
		return account.LifecycleState{}, err
	}
	row, err := q.GetLifecycleState(ctx, accountID)
	if err != nil {
		return account.LifecycleState{}, notFound(err, appaccount.ErrNotFound, "lifecycle state of "+accountID)
	}
	return lifecycleState(row), nil
}

func lifecycleState(row postgresgen.AccountLifecycleState) account.LifecycleState {
	return account.LifecycleState{
		AccountID:     row.AccountID,
		State:         account.LifecycleStateName(row.State),
		Since:         utc(row.Since),
		DeletionDueAt: utcPtr(row.DeletionDueAt),
	}
}

// UpsertLifecycleState implements account.Repository.
func (r *AccountRepository) UpsertLifecycleState(ctx context.Context, tx uow.Tx, s account.LifecycleState) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	err = q.UpsertLifecycleState(ctx, postgresgen.UpsertLifecycleStateParams{
		AccountID: s.AccountID, State: string(s.State), Since: s.Since, DeletionDueAt: s.DeletionDueAt,
	})
	return wrap(err, "upsert lifecycle state")
}

// DeleteAllLifecycleStates implements account.Repository.
func (r *AccountRepository) DeleteAllLifecycleStates(ctx context.Context, tx uow.Tx) error {
	q, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(q.DeleteAllLifecycleStates(ctx), "delete lifecycle states")
}

// ListLocalAccounts implements account.Repository.
func (r *AccountRepository) ListLocalAccounts(ctx context.Context, tx uow.Tx) ([]account.LocalAccount, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListLocalAccounts(ctx)
	if err != nil {
		return nil, wrap(err, "list local accounts")
	}
	out := make([]account.LocalAccount, len(rows))
	for i, row := range rows {
		out[i] = account.LocalAccount(row)
	}
	return out, nil
}

// ListProfileRevisionSeqs implements account.Repository.
func (r *AccountRepository) ListProfileRevisionSeqs(ctx context.Context, tx uow.Tx) ([]account.ProfileHead, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListProfileRevisionSeqs(ctx)
	if err != nil {
		return nil, wrap(err, "list profile revisions")
	}
	out := make([]account.ProfileHead, len(rows))
	for i, row := range rows {
		out[i] = account.ProfileHead{AccountID: row.AccountID, RevisionID: row.ID, Seq: int(row.Seq)}
	}
	return out, nil
}

// ListLifecycleEvents implements account.Repository.
func (r *AccountRepository) ListLifecycleEvents(ctx context.Context, tx uow.Tx) ([]account.LifecycleEvent, error) {
	q, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListLifecycleEvents(ctx)
	if err != nil {
		return nil, wrap(err, "list lifecycle events")
	}
	out := make([]account.LifecycleEvent, len(rows))
	for i, row := range rows {
		out[i] = account.LifecycleEvent{
			ID:                 row.ID,
			AccountID:          row.AccountID,
			Kind:               account.LifecycleKind(row.Kind),
			InitiatedBy:        account.Initiator(row.InitiatedBy),
			ModerationActionID: fromNullString(row.ModerationActionID),
			DeletionDueAt:      utcPtr(row.DeletionDueAt),
			OccurredAt:         utc(row.OccurredAt),
		}
	}
	return out, nil
}

// ListHandles implements account.Repository.
func (r *AccountRepository) ListHandles(ctx context.Context) ([]account.Handle, error) {
	rows, err := r.db.reader().ListAccountHandles(ctx)
	if err != nil {
		return nil, wrap(err, "list account handles")
	}
	out := make([]account.Handle, len(rows))
	for i, row := range rows {
		out[i] = account.Handle(row)
	}
	return out, nil
}

// ListProfileHeads implements account.Repository.
func (r *AccountRepository) ListProfileHeads(ctx context.Context) ([]account.ProfileHead, error) {
	rows, err := r.db.reader().ListProfileHeads(ctx)
	if err != nil {
		return nil, wrap(err, "list profile heads")
	}
	out := make([]account.ProfileHead, len(rows))
	for i, row := range rows {
		out[i] = account.ProfileHead{AccountID: row.AccountID, RevisionID: row.RevisionID, Seq: int(row.Seq)}
	}
	return out, nil
}

// ListLifecycleStates implements account.Repository.
func (r *AccountRepository) ListLifecycleStates(ctx context.Context) ([]account.LifecycleState, error) {
	rows, err := r.db.reader().ListLifecycleStates(ctx)
	if err != nil {
		return nil, wrap(err, "list lifecycle states")
	}
	out := make([]account.LifecycleState, len(rows))
	for i, row := range rows {
		out[i] = lifecycleState(row)
	}
	return out, nil
}

func accountFrom(row postgresgen.Account) account.Account {
	return account.Account{
		ID: row.ID, URI: row.Uri, Domain: row.Domain, ActorType: account.ActorType(row.ActorType), CreatedAt: utc(row.CreatedAt),
	}
}

// FindAccount implements account.Repository.
func (r *AccountRepository) FindAccount(ctx context.Context, id string) (account.Account, error) {
	row, err := r.db.reader().GetAccount(ctx, id)
	if err != nil {
		return account.Account{}, notFound(err, appaccount.ErrNotFound, "account "+id)
	}
	return accountFrom(row), nil
}

// FindInstanceActor implements account.Repository.
func (r *AccountRepository) FindInstanceActor(ctx context.Context) (account.Account, error) {
	row, err := r.db.reader().GetInstanceActor(ctx)
	if err != nil {
		return account.Account{}, notFound(err, appaccount.ErrNotFound, "instance actor")
	}
	return accountFrom(row), nil
}

// FindLocalAccount implements account.Repository.
func (r *AccountRepository) FindLocalAccount(ctx context.Context, accountID string) (account.LocalAccount, error) {
	row, err := r.db.reader().GetLocalAccount(ctx, accountID)
	if err != nil {
		return account.LocalAccount{}, notFound(err, appaccount.ErrNotFound, "local account "+accountID)
	}
	return account.LocalAccount(row), nil
}

// FindHandle implements account.Repository.
func (r *AccountRepository) FindHandle(ctx context.Context, usernameNormalized, domain string) (account.Handle, error) {
	row, err := r.db.reader().GetAccountHandleByName(ctx, postgresgen.GetAccountHandleByNameParams{
		UsernameNormalized: usernameNormalized, Domain: domain,
	})
	if err != nil {
		return account.Handle{}, notFound(err, appaccount.ErrNotFound, fmt.Sprintf("handle %s@%s", usernameNormalized, domain))
	}
	return account.Handle(row), nil
}

// FindHandleOf implements account.Repository.
func (r *AccountRepository) FindHandleOf(ctx context.Context, accountID string) (account.Handle, error) {
	row, err := r.db.reader().GetAccountHandle(ctx, accountID)
	if err != nil {
		return account.Handle{}, notFound(err, appaccount.ErrNotFound, "handle of "+accountID)
	}
	return account.Handle(row), nil
}

// FindCurrentProfile implements account.Repository.
func (r *AccountRepository) FindCurrentProfile(ctx context.Context, accountID string) (account.ProfileRevision, error) {
	q := r.db.reader()
	head, err := q.GetProfileHead(ctx, accountID)
	if err != nil {
		return account.ProfileRevision{}, notFound(err, appaccount.ErrNotFound, "profile head of "+accountID)
	}
	row, err := q.GetProfileRevision(ctx, head.RevisionID)
	if err != nil {
		return account.ProfileRevision{}, notFound(err, appaccount.ErrNotFound, "profile revision "+head.RevisionID)
	}
	fields, err := q.ListProfileFields(ctx, row.ID)
	if err != nil {
		return account.ProfileRevision{}, wrap(err, "list profile fields")
	}
	rev := account.ProfileRevision{
		ID:            row.ID,
		AccountID:     row.AccountID,
		Seq:           int(row.Seq),
		DisplayName:   row.DisplayName,
		NoteHTML:      row.NoteHtml,
		Locked:        row.Locked,
		Bot:           row.Bot,
		Discoverable:  row.Discoverable,
		Indexable:     row.Indexable,
		AvatarMediaID: fromNullString(row.AvatarMediaID),
		HeaderMediaID: fromNullString(row.HeaderMediaID),
		RevisedAt:     utc(row.RevisedAt),
	}
	for _, f := range fields {
		rev.Fields = append(rev.Fields, account.ProfileField{Position: int(f.Position), Name: f.Name, ValueHTML: f.ValueHtml})
	}
	return rev, nil
}

// FindLifecycleState implements account.Repository.
func (r *AccountRepository) FindLifecycleState(ctx context.Context, accountID string) (account.LifecycleState, error) {
	row, err := r.db.reader().GetLifecycleState(ctx, accountID)
	if err != nil {
		return account.LifecycleState{}, notFound(err, appaccount.ErrNotFound, "lifecycle state of "+accountID)
	}
	return lifecycleState(row), nil
}

// ListKeys implements account.Repository.
func (r *AccountRepository) ListKeys(ctx context.Context, accountID string) ([]account.Key, error) {
	rows, err := r.db.reader().ListAccountKeys(ctx, accountID)
	if err != nil {
		return nil, wrap(err, "list account keys")
	}
	out := make([]account.Key, len(rows))
	for i, row := range rows {
		out[i] = account.Key{
			ID: row.ID, AccountID: row.AccountID, KeyID: row.KeyID, Algorithm: account.KeyAlgorithm(row.Algorithm),
			PublicKeyPEM: row.PublicKeyPem, CreatedAt: utc(row.CreatedAt),
		}
	}
	return out, nil
}

// FindSealedPrivateKey implements account.Repository.
func (r *AccountRepository) FindSealedPrivateKey(ctx context.Context, accountKeyID string) ([]byte, error) {
	sealed, err := r.db.reader().GetLocalAccountPrivateKey(ctx, accountKeyID)
	if err != nil {
		return nil, notFound(err, appaccount.ErrNotFound, "private key of "+accountKeyID)
	}
	return sealed, nil
}
