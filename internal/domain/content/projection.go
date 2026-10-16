// SPDX-License-Identifier: AGPL-3.0-only

package content

import "time"

// Head points at the current revision of a status that is not deleted (the status_heads
// projection).
type Head struct {
	StatusID      string
	RevisionID    string
	Seq           int
	LastRevisedAt time.Time
}

// Edited reports whether the status has been revised after it was posted.
func (h Head) Edited() bool { return h.Seq > 1 }

// HeadOf returns the head that points at revision r.
func HeadOf(r Revision) Head {
	return Head{StatusID: r.StatusID, RevisionID: r.ID, Seq: r.Seq, LastRevisedAt: r.RevisedAt}
}

// Supersedes reports whether revision r should replace the current head h, which is the zero
// value when the status has no head yet.
func (r Revision) Supersedes(h Head) bool {
	return h.RevisionID == "" || r.Seq > h.Seq
}

// ReplyLink connects a reply to its parent once both are known and neither is deleted
// (the status_reply_links projection).
type ReplyLink struct {
	StatusID              string
	ParentStatusID        string
	ParentAuthorAccountID string
}

// Mention says that the current revision of a status mentions an account
// (the status_mention_index projection).
type Mention struct {
	AccountID string
	StatusID  string
}

// Counters are the counters of a status that is not deleted (the status_counters projection).
// RepliesCount counts the known replies that are not deleted and whose visibility
// CountsAsReply, BoostsCount the boosts that are not retracted, and FavouritesCount the
// favourites.
type Counters struct {
	StatusID        string
	RepliesCount    int64
	BoostsCount     int64
	FavouritesCount int64
}

// PostCounter counts the statuses of an account that are not deleted
// (the account_post_counters projection). An account without statuses has no counter.
type PostCounter struct {
	AccountID     string
	StatusesCount int64
	// LastStatusAt is the latest PublishedAt among those statuses.
	LastStatusAt time.Time
}
