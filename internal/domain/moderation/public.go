// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
)

// publicDomainActionKinds are the kinds of domain actions that the instance may list publicly
// (FR-718). The others (reject_media, reject_reports, force_sensitive) are the instance's own
// handling of the domain's content and stay private.
var publicDomainActionKinds = []DomainActionKind{DomainLimit, DomainSuspend}

// PublicDomainActionKinds returns the kinds of domain actions that the instance may list
// publicly.
func PublicDomainActionKinds() []DomainActionKind { return slices.Clone(publicDomainActionKinds) }

// PublicDomainAction is a domain action as the instance lists it publicly (FR-718): without
// the private note, and with the domain obfuscated when the action asks for it.
type PublicDomainAction struct {
	// ID is the action's ID, for paging; the public list does not show it.
	ID string
	// Domain is the canonical domain, obfuscated (ObfuscateDomain) when the action asks.
	Domain string
	// Digest is the SHA-256 of the canonical domain in lower-case hexadecimal, so that a
	// reader can check a domain it knows against an obfuscated one.
	Digest  string
	Kind    DomainActionKind
	Comment string // the public comment; "" for none
}

// Public returns a as the instance lists it publicly.
func (a DomainAction) Public() PublicDomainAction {
	sum := sha256.Sum256([]byte(a.Domain))
	p := PublicDomainAction{ID: a.ID, Domain: a.Domain, Digest: hex.EncodeToString(sum[:]), Kind: a.Kind, Comment: a.PublicComment}
	if a.Obfuscate {
		p.Domain = ObfuscateDomain(a.Domain)
	}
	return p
}

// ObfuscateDomain hides the middle half of a canonical domain: every byte other than a dot
// whose index i satisfies n/4 < i < n-n/4, for a domain of n bytes, becomes '*'. The first and
// last quarter stay readable, and so do the dots, which keep the shape of the domain
// ("bad.example" becomes "bad.*****le"). A canonical domain is ASCII (IDNA A-labels).
func ObfuscateDomain(domain string) string {
	n := len(domain)
	visible := n / 4
	b := []byte(domain)
	for i := range b {
		if i > visible && i < n-visible && b[i] != '.' {
			b[i] = '*'
		}
	}
	return string(b)
}
