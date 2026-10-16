// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

var (
	// ErrNotFound reports that no row matches a lookup.
	ErrNotFound = errors.New("content: not found")
	// ErrStatusExists reports a status whose URI another status already has.
	ErrStatusExists = errors.New("content: a status with this URI exists")
	// ErrIdempotencyKeyUsed reports a local status whose author already posted with the same
	// Idempotency-Key.
	ErrIdempotencyKeyUsed = errors.New("content: the idempotency key is used")
	// ErrAlreadyDeleted reports a second deletion of a status or of its URI.
	ErrAlreadyDeleted = errors.New("content: the status is already deleted")
)

// Repository is the persistence port of the content module's statuses (data-model §4.4). The
// database adapters implement it and pass testkit.StatusRepositoryContract.
//
// Methods that take a uow.Tx run inside that transaction; the others read committed data outside
// any transaction. Lookups that find nothing return an error wrapping ErrNotFound.
type Repository interface {
	// Resources and events, written by commands.

	// InsertStatus inserts a status. A URI that another status has fails with ErrStatusExists.
	InsertStatus(ctx context.Context, tx uow.Tx, s content.Status) error
	// InsertLocalStatus inserts the local subtype of a status. An Idempotency-Key that the
	// author already used fails with ErrIdempotencyKeyUsed.
	InsertLocalStatus(ctx context.Context, tx uow.Tx, ls content.LocalStatus) error
	// InsertRevision inserts a revision with its bodies, source, attachments, mentions, tags, and
	// emoji. On PostgreSQL, a Seq that a concurrent revision took fails with an error for which
	// errors.Is(err, uow.ErrRetryable) holds, so that a new attempt reads the new head.
	InsertRevision(ctx context.Context, tx uow.Tx, r content.Revision) error
	// InsertDeletion records the deletion of a status. A status or URI that is already deleted
	// fails with ErrAlreadyDeleted.
	InsertDeletion(ctx context.Context, tx uow.Tx, d content.Deletion) error
	// EnsureTag inserts t unless a tag with its NameNormalized exists, and returns the stored tag.
	EnsureTag(ctx context.Context, tx uow.Tx, t content.Tag) (content.Tag, error)

	// Projections, written only by this module's projectors and rebuild.

	// Head returns the head of a status.
	Head(ctx context.Context, tx uow.Tx, statusID string) (content.Head, error)
	UpsertHead(ctx context.Context, tx uow.Tx, h content.Head) error
	// ReplaceMentions makes accountIDs the mentions of a status in status_mention_index.
	ReplaceMentions(ctx context.Context, tx uow.Tx, statusID string, accountIDs []string) error
	// IsDeleted reports whether a status has a deletion.
	IsDeleted(ctx context.Context, tx uow.Tx, statusID string) (bool, error)
	// FindLiveStatusByURI returns the status with uri unless it is deleted.
	FindLiveStatusByURI(ctx context.Context, tx uow.Tx, uri string) (content.Status, error)
	// ListLiveRepliesTo returns the statuses that reply to uri and are not deleted, ordered by ID.
	ListLiveRepliesTo(ctx context.Context, tx uow.Tx, uri string) ([]content.Status, error)
	UpsertReplyLink(ctx context.Context, tx uow.Tx, l content.ReplyLink) error
	// ReplyLink returns the link from a reply to its parent.
	ReplyLink(ctx context.Context, tx uow.Tx, statusID string) (content.ReplyLink, error)
	// InsertCounters inserts the counters of a status unless it has them.
	InsertCounters(ctx context.Context, tx uow.Tx, c content.Counters) error
	// AddReplies adds delta to the replies count of a status that has counters.
	AddReplies(ctx context.Context, tx uow.Tx, statusID string, delta int64) error
	// AddPost counts one more status of an account, published at publishedAt.
	AddPost(ctx context.Context, tx uow.Tx, accountID string, publishedAt time.Time) error
	// RemovePost counts one status of an account less. It runs after the deletion is inserted
	// and takes LastStatusAt again from the account's statuses that are not deleted; the
	// counter goes away with the last of them.
	RemovePost(ctx context.Context, tx uow.Tx, accountID string) error
	// DeleteStatusProjections removes a status from every status projection: its head, its
	// mentions, its counters, its link to its parent, and the links of its replies to it.
	DeleteStatusProjections(ctx context.Context, tx uow.Tx, statusID string) error

	// Rebuilds empty one projection and derive it again from the statuses, their revisions,
	// and their deletions, by the same rules as the projectors (data-model §9).

	RebuildHeads(ctx context.Context, tx uow.Tx) error
	RebuildMentions(ctx context.Context, tx uow.Tx) error
	RebuildReplyLinks(ctx context.Context, tx uow.Tx) error
	// RebuildCounters also counts the boosts that are not retracted and the favourites of
	// each status, by the rules of RegisterInteractionProjectors.
	RebuildCounters(ctx context.Context, tx uow.Tx) error
	RebuildPostCounters(ctx context.Context, tx uow.Tx) error

	// Projection contents, for verifying the projections: heads, reply links, counters, and post
	// counters ordered by their key; mentions ordered by account ID and status ID.

	ListHeads(ctx context.Context) ([]content.Head, error)
	ListMentions(ctx context.Context) ([]content.Mention, error)
	ListReplyLinks(ctx context.Context) ([]content.ReplyLink, error)
	ListCounters(ctx context.Context) ([]content.Counters, error)
	ListPostCounters(ctx context.Context) ([]content.PostCounter, error)

	// Sources of other modules' projections (ReplayEvents).

	// ListCurrentPosts returns every status that is not deleted with its current revision and
	// the revision's children, ordered by status ID.
	ListCurrentPosts(ctx context.Context, tx uow.Tx) ([]Post, error)

	// Lookups.

	FindStatus(ctx context.Context, id string) (content.Status, error)
	FindStatusByURI(ctx context.Context, uri string) (content.Status, error)
	FindLocalStatus(ctx context.Context, statusID string) (content.LocalStatus, error)
	// FindLocalStatusByIdempotencyKey returns the local status that an author posted with key.
	FindLocalStatusByIdempotencyKey(ctx context.Context, authorAccountID, key string) (content.LocalStatus, error)
	// FindRevision returns a revision with its bodies, source, attachments, mentions, tags, and emoji.
	FindRevision(ctx context.Context, id string) (content.Revision, error)
	// FindCurrentRevision returns the revision that the head of a status points at; a deleted
	// status has none.
	FindCurrentRevision(ctx context.Context, statusID string) (content.Revision, error)
	// ListRevisions returns the revisions of a status, oldest first, without their children
	// (the edit history, FR-109).
	ListRevisions(ctx context.Context, statusID string) ([]content.Revision, error)
	FindDeletion(ctx context.Context, statusID string) (content.Deletion, error)
	// FindDeletionByURI returns the tombstone of a URI.
	FindDeletionByURI(ctx context.Context, uri string) (content.Deletion, error)
	FindTagByName(ctx context.Context, nameNormalized string) (content.Tag, error)
	FindTag(ctx context.Context, id string) (content.Tag, error)
	// ListReplies returns the statuses that reply to uri and are not deleted, ordered by ID.
	ListReplies(ctx context.Context, uri string) ([]content.Status, error)
	FindCounters(ctx context.Context, statusID string) (content.Counters, error)
	// FindPostCounter returns the post counter of an account; ErrNotFound means it has no
	// statuses.
	FindPostCounter(ctx context.Context, accountID string) (content.PostCounter, error)
}
