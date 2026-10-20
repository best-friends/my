// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/identity"
)

// The CSV exports and imports of a user's follows, blocks, mutes, and bookmarks in Mastodon's
// formats (FR-008, data-model §4.1). An export is written on request from what the user has
// now. An import records its items (the file's entries, and for an overwrite what the user has
// that the file does not list) in one transaction with its first job; the job carries out a
// batch of items, resolving remote accounts and posts over the network and sending follows
// through the graph module, records each item's result, and enqueues the next batch.

const (
	// JobProcessImport carries out a batch of the items of an import. Its payload is ImportJob.
	JobProcessImport = "identity.process_import"
	// JobExpireImports removes the imports older than ImportTTL.
	JobExpireImports = "identity.expire_imports"
	// ImportExpireInterval is the period of the JobExpireImports schedule.
	ImportExpireInterval = time.Hour
	// ImportTTL is how long an import and its results stay after it was created.
	ImportTTL = 7 * 24 * time.Hour
	// ImportListLimit is the number of imports ListImports returns.
	ImportListLimit = 10
	// ImportItemAttempts is the number of runs of JobProcessImport that an item may fail with an
	// error before it is recorded as failed, so that the rest of the import goes on.
	ImportItemAttempts = 5
	// importBatch is the number of items one run of JobProcessImport carries out.
	importBatch = 20
	// importItemTimeout bounds one item, which may fetch from a remote server.
	importItemTimeout = 30 * time.Second
	// importExpireBatch bounds the imports that one run of JobExpireImports removes.
	importExpireBatch = 100
)

// ErrImportInProgress reports a new import while another import of the user is being carried out.
var ErrImportInProgress = errors.New("identity: an import is being carried out")

// ImportRepository is the persistence of the imports. The database adapters' identity
// repositories implement it and pass testkit.AccountRepositoryContract.
type ImportRepository interface {
	InsertDataImport(ctx context.Context, tx uow.Tx, i identity.DataImport) error
	// InsertDataImportItems inserts items of one import.
	InsertDataImportItems(ctx context.Context, tx uow.Tx, items []identity.DataImportItem) error
	// InsertDataImportResult records how an item ended; a second result of the same item does
	// nothing, so that the first one stands.
	InsertDataImportResult(ctx context.Context, tx uow.Tx, r identity.DataImportResult) error
	// FindDataImport returns an import with its numbers. It fails with ErrNotFound when there is
	// none.
	FindDataImport(ctx context.Context, id string) (identity.DataImportProgress, error)
	// ListDataImports returns up to limit imports of a user with their numbers, newest first.
	ListDataImports(ctx context.Context, userID string, limit int) ([]identity.DataImportProgress, error)
	// ListPendingDataImportItems returns up to limit items of an import without a result, in
	// position order.
	ListPendingDataImportItems(ctx context.Context, importID string, limit int) ([]identity.DataImportItem, error)
	// ListDataImportsCreatedBefore returns the IDs of up to limit imports created before t,
	// oldest first.
	ListDataImportsCreatedBefore(ctx context.Context, t time.Time, limit int) ([]string, error)
	// DeleteDataImport deletes an import with its items and their results; deleting a missing
	// one succeeds.
	DeleteDataImport(ctx context.Context, tx uow.Tx, id string) error
}

// ImportTarget is something a local account has of a kind: an account it follows, blocks, or
// mutes, a status it bookmarked, or a domain it mutes, with the row of the CSV export that
// names it.
type ImportTarget struct {
	// ID is the ID of the account or the status, or the domain.
	ID  string
	Row identity.ExportRow
}

// ImportTargets reads and changes what a local account has of each kind; the composition root
// implements it with the graph and content modules and the resolution of remote accounts and
// posts. Addresses of this instance's accounts are username@AccountDomain.
type ImportTargets interface {
	// List returns what accountID has of kind now, newest first.
	List(ctx context.Context, accountID string, kind identity.ImportKind) ([]ImportTarget, error)
	// Add resolves the target of an ImportAdd item, fetching it from its server when needed, and
	// adds it to what accountID has of kind with the item's settings; it is safe to repeat. It
	// fails with an *ImportItemError when the item cannot be carried out; other errors are
	// worth retrying.
	Add(ctx context.Context, accountID string, kind identity.ImportKind, item identity.DataImportItem) error
	// Remove removes the account, status, or domain targetID from what accountID has of kind; removing
	// what is not there succeeds.
	Remove(ctx context.Context, accountID string, kind identity.ImportKind, targetID string) error
}

// ImportItemError reports an item that cannot be carried out, and why.
type ImportItemError struct {
	Failure identity.ImportFailure
	Err     error
}

func (e *ImportItemError) Error() string {
	return fmt.Sprintf("identity: import item failed (%s): %v", e.Failure, e.Err)
}

func (e *ImportItemError) Unwrap() error { return e.Err }

// ImportDeps are the ports of Imports.
type ImportDeps struct {
	UnitOfWork uow.UnitOfWork
	Repository ImportRepository
	// Outbox enqueues the jobs of an import in the transactions that record it.
	Outbox  queue.Outbox
	Targets ImportTargets
	// Host and AccountDomain are this instance's domains: an address on either, or without a
	// domain, names a local account.
	Host, AccountDomain string
	// CanonicalDomain returns the canonical form of a domain of a domain mute (IDNA A-labels in
	// lower case), to compare a file's domains with the muted ones; nil compares them in lower
	// case.
	CanonicalDomain func(domain string) (string, error)
	Clock           kernel.Clock
	IDs             kernel.IDGenerator
	// Logger logs the items recorded as failed after errors; nil means slog.Default().
	Logger *slog.Logger
}

// Imports runs the CSV exports and imports.
type Imports struct{ d ImportDeps }

// NewImports returns the CSV exports and imports on d.
func NewImports(d ImportDeps) *Imports {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.AccountDomain == "" {
		d.AccountDomain = d.Host
	}
	return &Imports{d: d}
}

// ImportJob is the payload of JobProcessImport.
type ImportJob struct {
	ImportID string `json:"import_id"`
}

// WriteExport writes the CSV file of what the local account accountID has of kind now, as
// Mastodon exports it (identity.WriteExportCSV).
func (m *Imports) WriteExport(ctx context.Context, accountID string, kind identity.ImportKind, w io.Writer) error {
	targets, err := m.d.Targets.List(ctx, accountID, kind)
	if err != nil {
		return err
	}
	rows := make([]identity.ExportRow, len(targets))
	for i, t := range targets {
		rows[i] = t.Row
	}
	return identity.WriteExportCSV(w, kind, rows)
}

// ImportPreview is what an import of a file would do.
type ImportPreview struct {
	// Entries are the file's entries, in file order.
	Entries []identity.ImportEntry
	// Skipped are the lines that are not entries.
	Skipped []identity.ImportSkipped
	// Present is the number of entries that the user has already.
	Present int
	// Removals are what an overwrite removes: what the user has that the file does not list.
	// Empty for a merge.
	Removals []ImportTarget
}

// ImportInput is a file to import.
type ImportInput struct {
	UserID, AccountID string
	Kind              identity.ImportKind
	Mode              identity.ImportMode
	Text              string
}

// PreviewImport reads the file of in and compares it with what the user has now, without
// changing anything. A file with more than identity.MaxImportEntries entries fails with
// identity.ErrTooManyImportEntries; one that is not CSV with the reader's error.
func (m *Imports) PreviewImport(ctx context.Context, in ImportInput) (ImportPreview, error) {
	entries, skipped, err := identity.ParseImportCSV(in.Kind, in.Text)
	if err != nil {
		return ImportPreview{}, err
	}
	current, err := m.d.Targets.List(ctx, in.AccountID, in.Kind)
	if err != nil {
		return ImportPreview{}, err
	}
	listed := make(map[string]bool, len(entries))
	for _, e := range entries {
		listed[m.key(in.Kind, e.Target)] = true
	}
	p := ImportPreview{Entries: entries, Skipped: skipped}
	for _, t := range current {
		switch {
		case listed[m.key(in.Kind, t.Row.Target)]:
			p.Present++
		case in.Mode == identity.ImportOverwrite:
			p.Removals = append(p.Removals, t)
		}
	}
	return p, nil
}

// key returns the form of a target that identifies it: an address in lower case, with this
// instance's domains (or none) as AccountDomain; a post's URL as it is; a domain in canonical
// form.
func (m *Imports) key(kind identity.ImportKind, target string) string {
	switch kind {
	case identity.ImportBookmarks:
		return target
	case identity.ImportDomainMutes:
		if m.d.CanonicalDomain != nil {
			if d, err := m.d.CanonicalDomain(target); err == nil {
				return d
			}
		}
		d, _ := identity.DomainKey(target)
		return d
	}
	username, domain, ok := identity.SplitAddress(target)
	if !ok {
		return target
	}
	if domain == "" || strings.EqualFold(domain, m.d.Host) || strings.EqualFold(domain, m.d.AccountDomain) {
		domain = m.d.AccountDomain
	}
	return strings.ToLower(username) + "@" + strings.ToLower(domain)
}

// StartImport records an import of the file of in and enqueues its first job in the same
// transaction: an item for each entry and, for an overwrite, one that removes each of
// PreviewImport's removals. While another import of the user has items without a result, it
// fails with ErrImportInProgress; a file that PreviewImport refuses fails the same way.
func (m *Imports) StartImport(ctx context.Context, in ImportInput) (identity.DataImportProgress, error) {
	recent, err := m.d.Repository.ListDataImports(ctx, in.UserID, 1)
	if err != nil {
		return identity.DataImportProgress{}, err
	}
	if len(recent) > 0 && !recent[0].Finished() {
		return identity.DataImportProgress{}, ErrImportInProgress
	}
	p, err := m.PreviewImport(ctx, in)
	if err != nil {
		return identity.DataImportProgress{}, err
	}
	imp := identity.DataImport{
		ID: m.d.IDs.NewID(), UserID: in.UserID, AccountID: in.AccountID, Kind: in.Kind, Mode: in.Mode,
		CreatedAt: kernel.Normalize(m.d.Clock.Now()),
	}
	items := make([]identity.DataImportItem, 0, len(p.Entries)+len(p.Removals))
	for _, e := range p.Entries {
		items = append(items, identity.DataImportItem{
			ImportID: imp.ID, Position: len(items), Action: identity.ImportAdd, Target: e.Target,
			ShowBoosts: e.ShowBoosts, Notify: e.Notify, HideNotifications: e.HideNotifications,
		})
	}
	for _, r := range p.Removals {
		items = append(items, identity.DataImportItem{ImportID: imp.ID, Position: len(items), Action: identity.ImportRemove, Target: r.ID})
	}
	payload, err := json.Marshal(ImportJob{ImportID: imp.ID})
	if err != nil {
		return identity.DataImportProgress{}, err
	}
	err = m.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := m.d.Repository.InsertDataImport(ctx, tx, imp); err != nil {
			return err
		}
		if err := m.d.Repository.InsertDataImportItems(ctx, tx, items); err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return m.d.Outbox.EnqueueTx(ctx, tx, processJob(imp.ID, payload, 0))
	})
	if err != nil {
		return identity.DataImportProgress{}, err
	}
	return identity.DataImportProgress{DataImport: imp, Total: len(items)}, nil
}

// processJob returns the job that carries out the items of an import from position on.
func processJob(importID string, payload []byte, position int) queue.NewJob {
	return queue.NewJob{
		Type: JobProcessImport, Payload: payload,
		IdempotencyKey: JobProcessImport + ":" + importID + ":" + strconv.Itoa(position),
		// One batch of an import at a time.
		OrderingKey: JobProcessImport + ":" + importID,
	}
}

// ListImports returns the latest ImportListLimit imports of a user, newest first.
func (m *Imports) ListImports(ctx context.Context, userID string) ([]identity.DataImportProgress, error) {
	return m.d.Repository.ListDataImports(ctx, userID, ImportListLimit)
}

// RegisterJobs registers the processing of the imports and the hourly removal of the old ones.
func (m *Imports) RegisterJobs(reg *queue.JobHandlerRegistry, sched *queue.Scheduler) {
	reg.Register(JobProcessImport, m.handleProcess)
	reg.Register(JobExpireImports, func(ctx context.Context, _ queue.Job) queue.Result {
		if _, err := m.ExpireImports(ctx); err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
	sched.Add(queue.Schedule{Name: JobExpireImports, Every: ImportExpireInterval, Job: queue.NewJob{Type: JobExpireImports}})
}

func (m *Imports) handleProcess(ctx context.Context, j queue.Job) queue.Result {
	var p ImportJob
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.ImportID == "" {
		return queue.Fail(fmt.Errorf("identity: invalid %s payload: %w", JobProcessImport, err))
	}
	if err := m.ProcessImport(ctx, p.ImportID, j.Attempt >= ImportItemAttempts); err != nil {
		return queue.Retry(0, err)
	}
	return queue.Done()
}

// ProcessImport carries out the next batch of items of an import that have no result, records
// each one's result, and enqueues the job of the next batch when there may be more. An item
// that fails with an *ImportItemError is recorded as failed; another error stops the batch to be
// retried, unless giveUp is set: then the item is recorded as failed (unavailable) and the batch
// goes on. A missing import, removed in the meantime, is done.
func (m *Imports) ProcessImport(ctx context.Context, importID string, giveUp bool) error {
	imp, err := m.d.Repository.FindDataImport(ctx, importID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	items, err := m.d.Repository.ListPendingDataImportItems(ctx, importID, importBatch)
	if err != nil {
		return err
	}
	for _, item := range items {
		res := identity.DataImportResult{ImportID: importID, Position: item.Position, Outcome: identity.ImportDone}
		var itemErr *ImportItemError
		switch err := m.carryOut(ctx, imp.DataImport, item); {
		case err == nil:
		case errors.As(err, &itemErr):
			res.Outcome, res.Failure = identity.ImportFailed, itemErr.Failure
		case giveUp && ctx.Err() == nil:
			m.d.Logger.WarnContext(ctx, "identity: an import item failed", "import_id", importID, "position", item.Position, "error", err)
			res.Outcome, res.Failure = identity.ImportFailed, identity.ImportFailureUnavailable
		default:
			return fmt.Errorf("identity: import %s item %d: %w", importID, item.Position, err)
		}
		res.ProcessedAt = kernel.Normalize(m.d.Clock.Now())
		if err := m.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return m.d.Repository.InsertDataImportResult(ctx, tx, res) }); err != nil {
			return err
		}
	}
	if len(items) < importBatch {
		return nil
	}
	payload, err := json.Marshal(ImportJob{ImportID: importID})
	if err != nil {
		return err
	}
	next := items[len(items)-1].Position + 1
	return m.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return m.d.Outbox.EnqueueTx(ctx, tx, processJob(importID, payload, next))
	})
}

// carryOut carries out one item, bounded by importItemTimeout.
func (m *Imports) carryOut(ctx context.Context, imp identity.DataImport, item identity.DataImportItem) error {
	ctx, cancel := context.WithTimeout(ctx, importItemTimeout)
	defer cancel()
	if item.Action == identity.ImportRemove {
		return m.d.Targets.Remove(ctx, imp.AccountID, imp.Kind, item.Target)
	}
	return m.d.Targets.Add(ctx, imp.AccountID, imp.Kind, item)
}

// ExpireImports removes up to importExpireBatch imports older than ImportTTL, with their items
// and results, and returns how many it removed.
func (m *Imports) ExpireImports(ctx context.Context) (int, error) {
	ids, err := m.d.Repository.ListDataImportsCreatedBefore(ctx, kernel.Normalize(m.d.Clock.Now()).Add(-ImportTTL), importExpireBatch)
	if err != nil {
		return 0, err
	}
	for i, id := range ids {
		if err := m.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return m.d.Repository.DeleteDataImport(ctx, tx, id) }); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

// EraseAccountData implements account.DataEraser: the deleted account's user loses every import
// (data-model §8).
func (m *Imports) EraseAccountData(ctx context.Context, t account.ErasureTarget, _ string, limit int) (account.ErasureStep, error) {
	if t.UserID == "" {
		return account.ErasureStep{Done: true}, nil
	}
	is, err := m.d.Repository.ListDataImports(ctx, t.UserID, limit)
	if err != nil {
		return account.ErasureStep{}, err
	}
	for _, i := range is {
		if err := m.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return m.d.Repository.DeleteDataImport(ctx, tx, i.ID) }); err != nil {
			return account.ErasureStep{}, err
		}
	}
	return account.ErasureStep{Done: len(is) < limit}, nil
}
