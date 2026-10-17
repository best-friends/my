// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appgraph "github.com/best-friends/my/internal/app/graph"
	apptimeline "github.com/best-friends/my/internal/app/timeline"
	"github.com/best-friends/my/internal/domain/federation"
)

// OutboxPageSize is the number of activities on one page of an outbox.
const OutboxPageSize = 20

// followerScanBatch is how many followers one query checks for a requester's domain.
const followerScanBatch = 1000

// Requester is who fetches an object or a collection (docs/architecture/visibility.md §1): the
// server of a verified signature, or no one for an unsigned request.
type Requester struct {
	// Domain is the canonical host of the signer's actor, "" for an anonymous request.
	Domain string
	// AccountID is the signer's account when this server knows it, "" otherwise.
	AccountID string
}

// Anonymous reports whether the request carries no verified signature.
func (r Requester) Anonymous() bool { return r.Domain == "" }

// ObjectStatuses is the part of the content module's public API that object fetches read.
// *appcontent.Service implements it.
type ObjectStatuses interface {
	ObjectStatus(ctx context.Context, id string) (appcontent.Status, appcontent.Revision, bool, error)
	AuthorRestrictions(ctx context.Context, accountID string) (appcontent.AuthorRestrictions, error)
	PostCount(ctx context.Context, accountID string) (int64, error)
}

// ObjectAccounts is the part of the account module's public API that object fetches read.
// *appaccount.Service implements it.
type ObjectAccounts interface {
	FindByID(ctx context.Context, id string) (appaccount.Actor, error)
	FindByURI(ctx context.Context, uri string) (appaccount.Actor, error)
	AnyOnDomain(ctx context.Context, domain string, ids []string) (bool, error)
}

// ObjectGraph is the part of the graph module's public API that object fetches read: the
// author's active followers, to find one on the requester's domain, and the signer's
// relationship with the author. *appgraph.Service implements it.
type ObjectGraph interface {
	ActiveFollowerIDs(ctx context.Context, accountID, before string, limit int) (ids []string, next string, err error)
	Relationship(ctx context.Context, accountID, targetID string) (appgraph.Relationship, error)
}

// ProfileTimeline lists an author's posts as a viewer may see them; *apptimeline.Service
// implements it.
type ProfileTimeline interface {
	Profile(ctx context.Context, viewer appcontent.Viewer, authorAccountID string, f apptimeline.ProfileFilter, q apptimeline.Query) (apptimeline.Page, error)
}

// StatusReader presents statuses to a viewer; *appcontent.Reader implements it.
type StatusReader interface {
	Views(ctx context.Context, viewer appcontent.Viewer, ids []string) ([]appcontent.StatusView, error)
	Pinned(ctx context.Context, viewer appcontent.Viewer, accountID string) ([]appcontent.StatusView, error)
}

// FetchSettings reads the instance setting that restricts GETs to signed requests;
// *appinstance.Service implements it.
type FetchSettings interface {
	// AuthorizedFetch reports whether authorized fetch (FR-408) is on.
	AuthorizedFetch(ctx context.Context) (bool, error)
}

// AcceptSignatureGET is the Accept-Signature field value of a 401 answer to a GET: the
// RFC 9421 signature that the profile of ADR-0032 requires.
const AcceptSignatureGET = acceptSignatureGET

// ObjectsDeps are the ports and settings of Objects.
type ObjectsDeps struct {
	Statuses ObjectStatuses
	Accounts ObjectAccounts
	Graph    ObjectGraph
	Timeline ProfileTimeline
	Reader   StatusReader
	// Signatures verifies the signatures of GET requests; nil treats every request as unsigned.
	Signatures *SignatureVerifier
	// Domains are the domain blocks; nil means NoDomainRestrictions.
	Domains DomainPolicy
	// Settings decide authorized fetch; nil keeps it off.
	Settings FetchSettings
	// Host is the serving domain of this server (ADR-0018).
	Host string
	// Logger logs the signatures that do not verify; nil means slog.Default().
	Logger *slog.Logger
}

// Objects serves the ActivityPub documents of local statuses and the collections made of them
// (federation.md §10). Who may see a status is decided by appcontent.CanView in
// ContextObjectFetch alone: the requester is the signer's server, and its relation to the author
// is aggregated over that domain (an active follower or a mentioned account on it counts).
type Objects struct {
	d ObjectsDeps
}

// NewObjects returns Objects on d.
func NewObjects(d ObjectsDeps) *Objects {
	if d.Domains == nil {
		d.Domains = NoDomainRestrictions{}
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Objects{d: d}
}

// AuthorizedFetch reports whether authorized fetch (FR-408, federation.md §10) is on: GETs of
// objects, collections, and full actor documents need a verified signature, and no answer that
// depends on the signature may be cached publicly. It is read per request, so that a change of
// the setting applies at once.
func (o *Objects) AuthorizedFetch(ctx context.Context) (bool, error) {
	if o.d.Settings == nil {
		return false, nil
	}
	return o.d.Settings.AuthorizedFetch(ctx)
}

// Authenticate returns the requester of a GET. A request without a signature, or with one that
// does not verify, is anonymous: such a request may still read what anyone may read, unless
// AuthorizedFetch is on and the caller refuses it. A signer on a suspended domain
// (federation.md §11) or on this server is anonymous too. Only an unexpected failure is an
// error.
func (o *Objects) Authenticate(ctx context.Context, r *SignedRequest) (Requester, error) {
	if o.d.Signatures == nil {
		return Requester{}, nil
	}
	sig, err := o.d.Signatures.Parse(r)
	if err == nil {
		err = o.d.Signatures.Check(r, sig)
	}
	var v *VerifiedSignature
	if err == nil {
		v, err = o.d.Signatures.Verify(ctx, r, sig)
	}
	switch {
	case errors.Is(err, ErrSignatureRejected):
		if !errors.Is(err, ErrNoSignature) {
			o.d.Logger.DebugContext(ctx, "GET signature not accepted", slog.String("path", r.URL.Path), slog.Any("reason", err))
		}
		return Requester{}, nil
	case err != nil:
		return Requester{}, err
	}
	owner, ok := signer(v)
	domain := owner.Host()
	if !ok || strings.EqualFold(domain, o.d.Host) {
		return Requester{}, nil
	}
	suspended, err := o.d.Domains.FederationSuspended(ctx, domain)
	if err != nil {
		return Requester{}, err
	}
	if suspended {
		return Requester{}, nil
	}
	req := Requester{Domain: domain}
	switch a, err := o.d.Accounts.FindByURI(ctx, string(owner)); {
	case err == nil:
		req.AccountID = a.Account.ID
	case !errors.Is(err, appaccount.ErrNotFound):
		return Requester{}, err
	}
	return req, nil
}

// signer returns the canonical IRI of the key's owner, false unless it is an https IRI on the
// host of the key.
func signer(v *VerifiedSignature) (federation.IRI, bool) {
	owner, err := CanonicalIRI(v.Owner)
	if err != nil || !strings.HasPrefix(string(owner), "https://") || owner.Host() == "" {
		return "", false
	}
	key, err := url.Parse(v.KeyID)
	if err != nil {
		return "", false
	}
	keyHost, err := CanonicalHost(key.Hostname())
	return owner, err == nil && keyHost == owner.Host()
}

// Object is the answer to a GET of an object IRI.
type Object struct {
	// Document is the JSON body: the object, or a Tombstone when Gone.
	Document []byte
	// Gone reports a deleted object, served with 410 Gone.
	Gone bool
	// Public reports that an anonymous request gets the same answer, so that it may be cached
	// publicly (ADR-0018).
	Public bool
	// URL is the HTML page of the status.
	URL string
}

// Status returns the Note of a local status for req (GET /ap/objects/{id}). A status that is
// missing, remote, or hidden from req fails with an error wrapping ErrNotFound alike, so that
// its existence does not leak (ADR-0018). A deleted status that req could see is Gone.
func (o *Objects) Status(ctx context.Context, req Requester, statusID string) (Object, error) {
	return o.object(ctx, req, statusID, false)
}

// StatusActivity returns the Create of a local status for req (GET /ap/objects/{id}/activity),
// as Status decides.
func (o *Objects) StatusActivity(ctx context.Context, req Requester, statusID string) (Object, error) {
	return o.object(ctx, req, statusID, true)
}

func (o *Objects) object(ctx context.Context, req Requester, statusID string, activity bool) (Object, error) {
	st, rev, deleted, err := o.d.Statuses.ObjectStatus(ctx, statusID)
	if errors.Is(err, appcontent.ErrNotFound) {
		return Object{}, fmt.Errorf("status %s: %w", statusID, ErrNotFound)
	}
	if err != nil {
		return Object{}, err
	}
	if !st.IsLocal {
		return Object{}, fmt.Errorf("remote status %s: %w", statusID, ErrNotFound)
	}
	restrictions, err := o.d.Statuses.AuthorRestrictions(ctx, st.AuthorAccountID)
	if err != nil {
		return Object{}, err
	}
	ok, err := o.canView(ctx, req, st, rev, restrictions)
	if err != nil {
		return Object{}, err
	}
	if !ok {
		return Object{}, fmt.Errorf("status %s: %w", statusID, ErrNotFound)
	}
	out := Object{Gone: deleted, URL: st.URL, Public: req.Anonymous()}
	if !out.Public {
		if out.Public, err = o.canView(ctx, Requester{}, st, rev, restrictions); err != nil {
			return Object{}, err
		}
	}
	id := federation.IRI(st.URI)
	if activity {
		id = createURI(st)
	}
	if deleted {
		out.Document, err = json.Marshal(federation.TombstoneOf(id).TopLevel())
		return out, err
	}
	author, err := o.d.Accounts.FindByID(ctx, st.AuthorAccountID)
	if err != nil {
		return Object{}, fmt.Errorf("author of status %s: %w", statusID, err)
	}
	mentioned, err := o.mentionedIRIs(ctx, rev.MentionedAccountIDs)
	if err != nil {
		return Object{}, err
	}
	actor := federation.IRI(author.Account.URI)
	vis := appcontent.EffectiveVisibility(st.Visibility, restrictions)
	note := revisionNote(st, rev, actor, vis, mentioned)
	if activity {
		out.Document, err = createOf(st, actor, note).Marshal()
	} else {
		out.Document, err = json.Marshal(note.TopLevel())
	}
	return out, err
}

// canView decides with appcontent.CanView whether req may see st. The relation is aggregated
// over the requester's domain: the signer's own blocks with the author, then, only when they
// are needed, whether an account on the domain is mentioned or actively follows the author.
// Facts only ever widen what CanView grants, so reading them lazily decides the same.
func (o *Objects) canView(ctx context.Context, req Requester, st appcontent.Status, rev appcontent.Revision, restrictions appcontent.AuthorRestrictions) (bool, error) {
	facts := appcontent.StatusFacts{AuthorAccountID: st.AuthorAccountID, AuthorIsLocal: st.IsLocal, Visibility: st.Visibility}
	viewer := appcontent.Viewer{Kind: appcontent.ViewerAnonymous}
	var rel appcontent.Relation
	if !req.Anonymous() {
		viewer = appcontent.Viewer{Kind: appcontent.ViewerRemote, AccountID: req.AccountID}
		if req.AccountID != "" {
			r, err := o.d.Graph.Relationship(ctx, req.AccountID, st.AuthorAccountID)
			if err != nil {
				return false, err
			}
			rel.Blocking, rel.BlockedBy = r.Blocking, r.BlockedBy
		}
	}
	if ok := appcontent.CanView(appcontent.ContextObjectFetch, viewer, facts, restrictions, rel); ok || req.Anonymous() {
		return ok, nil
	}
	var err error
	if rel.Mentioned, err = o.d.Accounts.AnyOnDomain(ctx, req.Domain, rev.MentionedAccountIDs); err != nil {
		return false, err
	}
	if appcontent.CanView(appcontent.ContextObjectFetch, viewer, facts, restrictions, rel) {
		return true, nil
	}
	follows, err := o.followerOnDomain(ctx, st.AuthorAccountID, req.Domain)
	if err != nil {
		return false, err
	}
	if follows {
		rel.Follow = appcontent.FollowActive
	}
	return appcontent.CanView(appcontent.ContextObjectFetch, viewer, facts, restrictions, rel), nil
}

// followerOnDomain reports whether an account on domain actively follows the author.
func (o *Objects) followerOnDomain(ctx context.Context, authorID, domain string) (bool, error) {
	before := ""
	for {
		ids, next, err := o.d.Graph.ActiveFollowerIDs(ctx, authorID, before, followerScanBatch)
		if err != nil {
			return false, err
		}
		found, err := o.d.Accounts.AnyOnDomain(ctx, domain, ids)
		if err != nil || found || next == "" {
			return found, err
		}
		before = next
	}
}

// mentionedIRIs returns the ids of the mentioned accounts that exist, in order.
func (o *Objects) mentionedIRIs(ctx context.Context, ids []string) ([]federation.IRI, error) {
	var out []federation.IRI
	for _, id := range ids {
		a, err := o.d.Accounts.FindByID(ctx, id)
		if errors.Is(err, appaccount.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, federation.IRI(a.Account.URI))
	}
	return out, nil
}

// OutboxTotal returns the totalItems of an outbox: the author's statuses that are not deleted,
// as Mastodon reports it.
func (o *Objects) OutboxTotal(ctx context.Context, accountID string) (int64, error) {
	return o.d.Statuses.PostCount(ctx, accountID)
}

// OutboxPage is one page of an outbox.
type OutboxPage struct {
	// Items are the Create activities of the page, newest first, without a context.
	Items []federation.ActivityDocument
	// Next is the cursor of the next page, "" on the last page.
	Next string
}

// Outbox returns a page of the outbox of a local account, starting after the cursor before (""
// for the newest): the Create of each status that an anonymous visitor may see on the profile,
// which lists only public and unlisted statuses (ADR-0018, federation.md §8). Boosts are not
// listed.
func (o *Objects) Outbox(ctx context.Context, accountID, before string) (OutboxPage, error) {
	anon := appcontent.Viewer{Kind: appcontent.ViewerAnonymous}
	page, err := o.d.Timeline.Profile(ctx, anon, accountID, apptimeline.ProfileFilter{ExcludeBoosts: true},
		apptimeline.Query{Before: before, Limit: OutboxPageSize})
	if err != nil {
		return OutboxPage{}, err
	}
	ids := make([]string, 0, len(page.Entries))
	for _, e := range page.Entries {
		ids = append(ids, e.StatusID)
	}
	views, err := o.d.Reader.Views(ctx, anon, ids)
	if err != nil {
		return OutboxPage{}, err
	}
	out := OutboxPage{Items: []federation.ActivityDocument{}, Next: page.Next}
	for _, v := range views {
		actor := federation.IRI(v.Author.Account.URI)
		out.Items = append(out.Items, createOf(v.Status, actor, viewNote(v)).Embedded())
	}
	return out, nil
}

// Featured returns the Notes of the statuses a local account pinned, newest pin first, that an
// anonymous visitor may see: the public and unlisted ones.
func (o *Objects) Featured(ctx context.Context, accountID string) ([]federation.ObjectDocument, error) {
	views, err := o.d.Reader.Pinned(ctx, appcontent.Viewer{Kind: appcontent.ViewerAnonymous}, accountID)
	if err != nil {
		return nil, err
	}
	out := []federation.ObjectDocument{}
	for _, v := range views {
		out = append(out, viewNote(v))
	}
	return out, nil
}

// viewNote returns the Note of a status as an anonymous visitor sees it.
func viewNote(v appcontent.StatusView) federation.ObjectDocument {
	mentioned := make([]federation.IRI, 0, len(v.Mentions))
	for _, m := range v.Mentions {
		mentioned = append(mentioned, federation.IRI(m.Account.URI))
	}
	// The reader showed the status to an anonymous visitor, so its effective visibility is its
	// own: a ceiling would have made it followers-only.
	return revisionNote(v.Status, v.Revision, federation.IRI(v.Author.Account.URI), v.Status.Visibility, mentioned)
}

// revisionNote returns the Note of st as of rev, addressed by the effective visibility vis.
func revisionNote(st appcontent.Status, rev appcontent.Revision, actor federation.IRI, vis appcontent.Visibility, mentioned []federation.IRI) federation.ObjectDocument {
	aud := federation.Address(federation.Visibility(vis), federation.FollowersOf(actor), mentioned)
	var updated time.Time
	if rev.Seq > 1 {
		updated = rev.RevisedAt
	}
	return statusNote(st, actor, aud, updated)
}

// createURI returns the IRI of the Create of a local status (ADR-0018: {object}/activity).
func createURI(st appcontent.Status) federation.IRI { return federation.IRI(st.URI + "/activity") }

// createOf returns the Create of a local status around its Note, addressed as the Note is.
func createOf(st appcontent.Status, actor federation.IRI, note federation.ObjectDocument) federation.ActivityDocument {
	aud := federation.Audience{To: note.To, Cc: note.Cc}
	return federation.NewActivityDocument(createURI(st), federation.ActivityCreate, actor, st.PublishedAt, aud, note)
}
