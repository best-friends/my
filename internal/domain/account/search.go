// SPDX-License-Identifier: AGPL-3.0-only

package account

// SearchTermKind says what a search term of an account was made from.
type SearchTermKind string

// Search term kinds (account_search_terms.kind).
const (
	// SearchTermUsername is the normalized username, or username@domain for a remote account.
	SearchTermUsername SearchTermKind = "username"
	// SearchTermDisplayName is the normalized display name, whole or one of its words.
	SearchTermDisplayName SearchTermKind = "display_name"
)

// SearchTerm is one word that the prefix search of accounts matches (the account_search_terms
// projection, ADR-0092 §1). Domain is the account's domain, LocalDomain for a local account;
// the search reads it with the term and does not store it.
type SearchTerm struct {
	AccountID string
	Kind      SearchTermKind
	Term      string
	IsLocal   bool
	Domain    string
}
