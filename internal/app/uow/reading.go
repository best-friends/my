// SPDX-License-Identifier: AGPL-3.0-only

package uow

import "context"

// readingKey keys the transaction that a context reads in.
type readingKey struct{}

// ReadingIn returns a copy of ctx whose repository reads run in tx: an adapter's read queries
// (those that take no Tx) join the transaction that ctx carries when it is the adapter's own,
// and see what tx has written so far. It lets a synchronous projector present what the
// transaction wrote through the read use cases of other modules (the data of the webhook
// messages, ADR-0060), which read without a Tx.
//
// Use the context only inside the function given to UnitOfWork.Tx, from its goroutine, and only
// for reads: a write still takes its Tx.
func ReadingIn(ctx context.Context, tx Tx) context.Context {
	return context.WithValue(ctx, readingKey{}, tx)
}

// ReadingTx returns the transaction that ctx reads in (ReadingIn), and false when it reads
// outside every transaction. Adapters call it to pick the connection of a read.
func ReadingTx(ctx context.Context) (Tx, bool) {
	tx, ok := ctx.Value(readingKey{}).(Tx)
	return tx, ok
}
