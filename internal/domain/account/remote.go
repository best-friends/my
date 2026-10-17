// SPDX-License-Identifier: AGPL-3.0-only

package account

import (
	"bytes"
	"time"
)

// RemoteActorTTL is how long a fetched remote actor stays fresh. After it, the actor is fetched
// again the next time it is used, and the periodic refresh fetches the ones that local accounts
// follow (federation.md §6.2, ADR-0019 §7).
const RemoteActorTTL = 24 * time.Hour

// RemoteAccount is the remote subtype of an account: an actor of another server.
type RemoteAccount struct {
	AccountID   string
	FirstSeenAt time.Time
}

// RemoteSnapshot is one fetch of a remote actor's document whose content differed from the
// previous fetch (the remote_account_snapshots event). Optional URLs are "" when the document
// has none; the totals are nil when it does not say.
type RemoteSnapshot struct {
	ID                string
	AccountID         string
	PreferredUsername string
	ProfileURL        string
	InboxURL          string
	SharedInboxURL    string
	OutboxURL         string
	FollowersURL      string
	FollowingURL      string
	FeaturedURL       string
	FollowersTotal    *int64
	FollowingTotal    *int64
	StatusesTotal     *int64
	// DocumentDigest is the SHA-256 of the fetched document; a fetch with the same digest as
	// the newest snapshot records no new one.
	DocumentDigest []byte
	FetchedAt      time.Time
}

// SameDocument reports whether s was made from the same document as the digest.
func (s RemoteSnapshot) SameDocument(digest []byte) bool {
	return s.ID != "" && bytes.Equal(s.DocumentDigest, digest)
}

// RemoteHead is the newest snapshot of a remote actor (the remote_account_heads projection):
// the endpoints delivery reads and the fetch state of the periodic refresh (data-model §4.2).
type RemoteHead struct {
	AccountID  string
	SnapshotID string
	InboxURL   string
	// SharedInboxURL is the server's shared inbox (endpoints.sharedInbox), "" when the actor
	// declares none.
	SharedInboxURL           string
	LastFetchedAt            time.Time
	ConsecutiveFetchFailures int
}

// Stale reports whether the actor was last fetched RemoteActorTTL or more before now.
func (h RemoteHead) Stale(now time.Time) bool {
	return !now.Before(h.LastFetchedAt.Add(RemoteActorTTL))
}

// HandleOutcome is the result of checking a remote actor's handle with WebFinger.
type HandleOutcome string

// Handle check outcomes (remote_account_handle_verifications.outcome).
const (
	// HandleVerified means that WebFinger of acct:{username}@{domain} names the actor as self.
	HandleVerified HandleOutcome = "verified"
	// HandleMismatch means that WebFinger answered without naming the actor: the account has
	// no handle until a later check verifies one.
	HandleMismatch HandleOutcome = "mismatch"
)

// HandleVerification is one WebFinger check of a remote actor's handle
// (the remote_account_handle_verifications event). Username and Domain are the handle that was
// checked; Domain is a canonical host.
type HandleVerification struct {
	ID        string
	AccountID string
	Username  string
	Domain    string
	Outcome   HandleOutcome
	CheckedAt time.Time
}

// SameResult reports whether v found what o found: the same outcome for the same handle.
func (v HandleVerification) SameResult(o HandleVerification) bool {
	return v.Outcome == o.Outcome && v.Username == o.Username && v.Domain == o.Domain
}

// Handle returns the handle that a verified check gives the account, and false for a mismatch.
func (v HandleVerification) Handle() (Handle, bool) {
	if v.Outcome != HandleVerified {
		return Handle{}, false
	}
	return Handle{
		AccountID: v.AccountID, Username: v.Username,
		UsernameNormalized: NormalizeUsername(v.Username), Domain: NormalizeDomain(v.Domain),
	}, true
}

// RemoteProfileSource is where a remote profile revision came from
// (the remote_profile_revisions subtype): the snapshot that was current when it was made and
// the version of the HTML policy that sanitized its NoteHTML and field values (ADR-0019 §2).
type RemoteProfileSource struct {
	SnapshotID        string
	HTMLPolicyVersion int
}

// KeyRevocation records that a key was replaced or withdrawn (the account_key_revocations
// event). Signatures never verify with a revoked key.
type KeyRevocation struct {
	AccountKeyID string
	RevokedAt    time.Time
}
