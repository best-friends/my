// SPDX-License-Identifier: AGPL-3.0-only

package postgres

import (
	"context"

	"github.com/best-friends/my/internal/adapter/db/postgres/postgresgen"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

var _ appcontent.QuoteRepository = (*StatusRepository)(nil)

// InsertQuote implements content.QuoteRepository.
func (r *StatusRepository) InsertQuote(ctx context.Context, tx uow.Tx, q content.Quote) (bool, error) {
	qs, err := queries(tx)
	if err != nil {
		return false, err
	}
	n, err := qs.InsertStatusQuote(ctx, postgresgen.InsertStatusQuoteParams{StatusID: q.StatusID, QuotedUri: q.QuotedURI, Form: string(q.Form)})
	return n > 0, wrap(err, "insert status quote")
}

// FindQuote implements content.QuoteRepository.
func (r *StatusRepository) FindQuote(ctx context.Context, statusID string) (content.Quote, error) {
	row, err := r.db.reader().GetStatusQuote(ctx, statusID)
	if err != nil {
		return content.Quote{}, notFound(err, appcontent.ErrNotFound, "quote of "+statusID)
	}
	return quoteFrom(row), nil
}

// FindQuoteTx implements content.QuoteRepository.
func (r *StatusRepository) FindQuoteTx(ctx context.Context, tx uow.Tx, statusID string) (content.Quote, error) {
	d, err := r.db.in(tx)
	if err != nil {
		return content.Quote{}, err
	}
	return d.StatusRepository().FindQuote(ctx, statusID)
}

// ListQuotesAfter implements content.QuoteRepository.
func (r *StatusRepository) ListQuotesAfter(ctx context.Context, tx uow.Tx, after string, limit int) ([]content.Quote, error) {
	qs, err := queries(tx)
	if err != nil {
		return nil, err
	}
	n, err := toInt32(limit, "page limit")
	if err != nil {
		return nil, err
	}
	rows, err := qs.ListStatusQuotesAfter(ctx, postgresgen.ListStatusQuotesAfterParams{After: after, PageLimit: n})
	if err != nil {
		return nil, wrap(err, "list status quotes")
	}
	out := make([]content.Quote, 0, len(rows))
	for _, row := range rows {
		out = append(out, quoteFrom(row))
	}
	return out, nil
}

// InsertQuoteEvent implements content.QuoteRepository.
func (r *StatusRepository) InsertQuoteEvent(ctx context.Context, tx uow.Tx, e content.QuoteEvent) error {
	qs, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(qs.InsertStatusQuoteEvent(ctx, postgresgen.InsertStatusQuoteEventParams{
		ID: e.ID, StatusID: e.StatusID, Kind: string(e.Kind), AuthorizationUri: toNullString(e.AuthorizationURI), OccurredAt: e.OccurredAt,
	}), "insert status quote event")
}

// ListQuoteEvents implements content.QuoteRepository.
func (r *StatusRepository) ListQuoteEvents(ctx context.Context, tx uow.Tx, statusID string) ([]content.QuoteEvent, error) {
	qs, err := queries(tx)
	if err != nil {
		return nil, err
	}
	rows, err := qs.ListStatusQuoteEvents(ctx, statusID)
	if err != nil {
		return nil, wrap(err, "list status quote events")
	}
	out := make([]content.QuoteEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, content.QuoteEvent{
			ID: row.ID, StatusID: row.StatusID, Kind: content.QuoteEventKind(row.Kind),
			AuthorizationURI: fromNullString(row.AuthorizationUri), OccurredAt: utc(row.OccurredAt),
		})
	}
	return out, nil
}

// UpsertQuoteState implements content.QuoteRepository.
func (r *StatusRepository) UpsertQuoteState(ctx context.Context, tx uow.Tx, s content.QuoteState) error {
	qs, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(qs.UpsertStatusQuoteState(ctx, postgresgen.UpsertStatusQuoteStateParams{
		StatusID: s.StatusID, State: string(s.Approval), AuthorizationUri: toNullString(s.AuthorizationURI),
	}), "upsert status quote state")
}

// FindQuoteState implements content.QuoteRepository.
func (r *StatusRepository) FindQuoteState(ctx context.Context, statusID string) (content.QuoteState, error) {
	row, err := r.db.reader().GetStatusQuoteState(ctx, statusID)
	if err != nil {
		return content.QuoteState{}, notFound(err, appcontent.ErrNotFound, "quote state of "+statusID)
	}
	return quoteStateFrom(row), nil
}

// FindQuoteStateTx implements content.QuoteRepository.
func (r *StatusRepository) FindQuoteStateTx(ctx context.Context, tx uow.Tx, statusID string) (content.QuoteState, error) {
	d, err := r.db.in(tx)
	if err != nil {
		return content.QuoteState{}, err
	}
	return d.StatusRepository().FindQuoteState(ctx, statusID)
}

// ListQuoteStatesByAuthorization implements content.QuoteRepository.
func (r *StatusRepository) ListQuoteStatesByAuthorization(ctx context.Context, uri string) ([]content.QuoteState, error) {
	rows, err := r.db.reader().ListStatusQuoteStatesByAuthorization(ctx, toNullString(uri))
	if err != nil {
		return nil, wrap(err, "list status quote states by authorization")
	}
	return quoteStatesFrom(rows), nil
}

// ListQuoteStates implements content.QuoteRepository.
func (r *StatusRepository) ListQuoteStates(ctx context.Context) ([]content.QuoteState, error) {
	rows, err := r.db.reader().ListAllStatusQuoteStates(ctx)
	if err != nil {
		return nil, wrap(err, "list status quote states")
	}
	return quoteStatesFrom(rows), nil
}

// DeleteAllQuoteStates implements content.QuoteRepository.
func (r *StatusRepository) DeleteAllQuoteStates(ctx context.Context, tx uow.Tx) error {
	qs, err := queries(tx)
	if err != nil {
		return err
	}
	return wrap(qs.DeleteAllStatusQuoteStates(ctx), "delete status quote states")
}

func quoteFrom(row postgresgen.StatusQuote) content.Quote {
	return content.Quote{StatusID: row.StatusID, QuotedURI: row.QuotedUri, Form: content.QuoteForm(row.Form)}
}

func quoteStateFrom(row postgresgen.StatusQuoteState) content.QuoteState {
	return content.QuoteState{StatusID: row.StatusID, Approval: content.QuoteApproval(row.State), AuthorizationURI: fromNullString(row.AuthorizationUri)}
}

func quoteStatesFrom(rows []postgresgen.StatusQuoteState) []content.QuoteState {
	out := make([]content.QuoteState, 0, len(rows))
	for _, row := range rows {
		out = append(out, quoteStateFrom(row))
	}
	return out
}
