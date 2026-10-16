// SPDX-License-Identifier: AGPL-3.0-only

// Package activitypub serves the federation endpoints by calling application use cases:
// WebFinger and host-meta (FR-401), NodeInfo 2.1 (FR-402), and the actor documents and
// collections of local accounts and the instance actor (FR-403), with the URIs of ADR-0018.
//
// Every lookup that finds nothing, or finds an account that may not be shown (a remote account,
// or a local account that is not active), answers with the same 404 response (ADR-0018).
// Responses that depend on the Accept header carry Vary: Accept, and only documents that anyone
// may fetch anonymously are cacheable (Cache-Control: public, max-age=60).
//
// Actor documents name the inbox and the shared inbox, which are not served yet. The outbox,
// followers, following, featured, and featured-tags collections are served, and are empty
// until posts and follows exist. Profile pages (/@alice) redirect ActivityPub requests to the
// actor and pass every other request to the web client's HTML handler (Options.HTML).
package activitypub
