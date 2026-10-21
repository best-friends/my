// SPDX-License-Identifier: AGPL-3.0-only

// Package finance holds the use cases of the instance's costs (ADR-0067, FR-901, FR-902): the
// cost items that are recorded with their schedule, label, prices, and end, their cancellation,
// and the totals by week, month, and year that the domain computes from them on every read. The
// adapters of Repository live in internal/adapter/db/*; their contract test suite is
// testkit.FinanceRepositoryContract.
package finance

import (
	"context"
	"errors"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/finance"
)

// ErrNotFound reports an unknown or canceled cost item.
var ErrNotFound = errors.New("finance: not found")

// LabelRevision is one revision of a cost item's label (cost_item_label_revisions).
type LabelRevision struct {
	ID                 string
	CostItemID         string
	Label              finance.Label
	ProviderNormalized string
	RevisedByUserID    string // "" for the CLI
	RevisedAt          time.Time
}

// PriceRevision is one revision of a cost item's price (cost_item_price_revisions).
type PriceRevision struct {
	CostItemID      string
	Revision        finance.PriceRevision
	RevisedByUserID string // "" for the CLI
}

// EndChange is a change of a cost item's end (cost_item_end_changes).
type EndChange struct {
	ID              string
	CostItemID      string
	EndsOn          *finance.Date // nil for no end
	ChangedByUserID string        // "" for the CLI
	ChangedAt       time.Time
}

// Cancellation is the cancellation of a cost item (cost_item_cancellations).
type Cancellation struct {
	ID               string
	CostItemID       string
	CanceledByUserID string // "" for the CLI
	CanceledAt       time.Time
}

// Head is the current label and end of a cost item that is not canceled (cost_item_heads).
type Head struct {
	CostItemID      string
	LabelRevisionID string
	EndsOn          *finance.Date // nil for no end
}

// Current is the head of a cost item with its start date, read in the transaction of a change.
type Current struct {
	Head     Head
	StartsOn finance.Date
}

// Repository is the persistence port of the finance module.
type Repository interface {
	// InsertItem inserts the cost_items row of item (its ID, recurrence, start, creator, and
	// creation time); the label revision, price revision, and head that complete it are inserted
	// in the same transaction.
	InsertItem(ctx context.Context, tx uow.Tx, item finance.CostItem) error
	InsertLabelRevision(ctx context.Context, tx uow.Tx, r LabelRevision) error
	InsertPriceRevision(ctx context.Context, tx uow.Tx, r PriceRevision) error
	InsertEndChange(ctx context.Context, tx uow.Tx, c EndChange) error
	InsertCancellation(ctx context.Context, tx uow.Tx, c Cancellation) error
	// UpsertHead sets the head of a cost item, and DeleteHead removes it (the projectors of
	// CostItemChanged and CostItemCanceled).
	UpsertHead(ctx context.Context, tx uow.Tx, h Head) error
	DeleteHead(ctx context.Context, tx uow.Tx, costItemID string) error
	// CurrentForUpdate returns the head and start of a cost item in tx, locking the head on the
	// databases that lock rows; it fails with ErrNotFound for an unknown or canceled item.
	CurrentForUpdate(ctx context.Context, tx uow.Tx, costItemID string) (Current, error)
	// FindItem returns a cost item that is not canceled, with its current label and end and every
	// price revision; it fails with ErrNotFound otherwise.
	FindItem(ctx context.Context, id string) (finance.CostItem, error)
	// ListItems returns up to limit cost items that are not canceled, newest first, before the
	// item before ("" for the first page).
	ListItems(ctx context.Context, before string, limit int) ([]finance.CostItem, error)
	// AllItems returns every cost item that is not canceled, newest first: what the totals read.
	AllItems(ctx context.Context) ([]finance.CostItem, error)

	ProjectionRepository
}

// ProjectionRepository is the part of Repository that rebuilds the finance projection
// (data-model §9).
type ProjectionRepository interface {
	// RebuildHeads empties cost_item_heads and derives it from the cost items that are not
	// canceled, with the newest label revision and end change of each.
	RebuildHeads(ctx context.Context, tx uow.Tx) error
}
