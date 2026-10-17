// SPDX-License-Identifier: AGPL-3.0-only

package account

import "time"

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
