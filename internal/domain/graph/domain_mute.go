// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// DomainMute hides the posts and notifications of a remote domain, and of its subdomains, from
// one local account (FR-304; domain_mutes, a current value: it affects only the muting account
// and keeps no history). Unlike the moderators' domain actions it changes nothing for anyone
// else and nothing that federates: the muted domain is not told.
type DomainMute struct {
	AccountID string
	// Domain is a canonical host (IDNA A-labels in lower case, without a trailing dot).
	Domain    string
	CreatedAt time.Time
}

// ErrInvalidDomain reports a domain mute whose domain is not a canonical remote host.
var ErrInvalidDomain = errors.New("graph: the domain is not a valid remote host")

// MaxDomainMutes is how many domains one account may mute.
const MaxDomainMutes = 1000

// ErrTooManyDomainMutes reports a domain mute beyond MaxDomainMutes.
var ErrTooManyDomainMutes = errors.New("graph: the account mutes too many domains")

// MutedDomains returns the domains whose mutes cover an account on domain: domain followed by
// each of its parent domains, from the nearest ("a.b.example" gives a.b.example, b.example, and
// example). The local domain "" has none.
func MutedDomains(domain string) []string {
	var out []string
	for domain != "" {
		out = append(out, domain)
		_, parent, ok := strings.Cut(domain, ".")
		if !ok {
			break
		}
		domain = parent
	}
	return out
}

// Validate checks the invariants of a domain mute.
func (m DomainMute) Validate() error {
	if m.AccountID == "" {
		return errors.New("graph: a domain mute needs an account")
	}
	if !ValidDomain(m.Domain) {
		return ErrInvalidDomain
	}
	return nil
}

// ValidDomain reports whether d looks like a canonical host: non-empty, lower case, valid
// UTF-8 without control characters, without a scheme, path, port, or empty label. Callers
// canonicalize hosts (IDNA A-labels) before.
func ValidDomain(d string) bool {
	if d == "" || len(d) > 253 || !utf8.ValidString(d) || d != strings.ToLower(d) || strings.ContainsAny(d, "/:@ \t\r\n\x00") {
		return false
	}
	for label := range strings.SplitSeq(d, ".") {
		if label == "" {
			return false
		}
	}
	return true
}
