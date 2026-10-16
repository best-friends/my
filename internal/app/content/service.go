// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/i18n"
)

var (
	// ErrAccountUnavailable reports an author that does not exist or is not active.
	ErrAccountUnavailable = errors.New("content: the account does not exist or is not active")
	// ErrRemoteAccount reports a remote author: these use cases cover local statuses only, and
	// remote statuses arrive through the federation module.
	ErrRemoteAccount = errors.New("content: the account is not local")
	// ErrPostingRestricted reports an author whose restrictions forbid posting and editing.
	ErrPostingRestricted = errors.New("content: the account may not post")
	// ErrNotAuthor reports an edit or deletion of another account's status.
	ErrNotAuthor = errors.New("content: the status belongs to another account")
	// ErrReplyTargetNotFound reports a reply to a status that does not exist, is deleted, or
	// that the author may not see.
	ErrReplyTargetNotFound = errors.New("content: the status replied to is not found")
	// ErrEmptyText reports a status without text.
	ErrEmptyText = errors.New("content: the text is empty")
	// ErrTextTooLong reports a text and content warning longer than max_status_length.
	ErrTextTooLong = errors.New("content: the text is too long")
	// ErrInvalidText reports text that is not valid UTF-8 or contains a NUL character.
	ErrInvalidText = errors.New("content: the text is not valid")
	// ErrInvalidLanguage reports a language that is not a well-formed BCP 47 tag.
	ErrInvalidLanguage = errors.New("content: the language is not a valid BCP 47 tag")
)

// UndeterminedLanguage is the language of a status whose language is unknown (BCP 47 "und").
const UndeterminedLanguage = "und"

// Accounts is the part of the account module's public API that the content module reads: the
// author's locality and lifecycle state, and the accounts that a text mentions.
// *appaccount.Service implements it.
type Accounts interface {
	FindByID(ctx context.Context, id string) (appaccount.Actor, error)
	FindByHandle(ctx context.Context, username, domain string) (appaccount.Actor, error)
}

// Follows is the part of the graph module's public API that the content module reads: whether
// an account actively follows another. *appgraph.Service implements it.
type Follows interface {
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
}

// Deps are the ports and settings a Service uses.
type Deps struct {
	UnitOfWork   uow.UnitOfWork
	Repository   Repository
	Accounts     Accounts
	Follows      Follows
	Restrictions RestrictionPolicy
	// MaxStatusLength returns the current max_status_length instance setting (FR-101); it is
	// read per command, so that changes apply at once.
	MaxStatusLength func(ctx context.Context) (int, error)
	// DefaultLanguage returns the default post language of a user (FR-110), used when a post
	// names none; nil or UndeterminedLanguage leaves the language undetermined.
	DefaultLanguage func(ctx context.Context, userID string) (string, error)
	Clock           kernel.Clock
	IDs             kernel.IDGenerator
	// Host is the serving domain of the status URIs and pages (ADR-0018).
	Host string
	// AccountDomain is the domain of local handles; @user@AccountDomain and @user@Host mention a
	// local account like @user. "" means Host.
	AccountDomain string
}

// Service runs the content module's use cases for local statuses: posting, editing, and
// deleting them.
type Service struct {
	d Deps
}

// NewService returns a Service on d.
func NewService(d Deps) *Service {
	if d.Restrictions == nil {
		d.Restrictions = NoRestrictions{}
	}
	if d.AccountDomain == "" {
		d.AccountDomain = d.Host
	}
	return &Service{d: d}
}

// Post is a status with its current revision.
type Post struct {
	Status   content.Status
	Revision content.Revision
}

// TextInput is the content of a post or an edit.
type TextInput struct {
	// Text is the plain text the author typed (FR-103). It must not be blank.
	Text string
	// SpoilerText is the content warning, "" for none (FR-104).
	SpoilerText string
	// Sensitive marks the media as sensitive (FR-104).
	Sensitive bool
	// Language is a BCP 47 tag; "" means the author's default post language (FR-110).
	Language string
}

// CreateInput describes a new local status.
type CreateInput struct {
	TextInput
	AuthorAccountID string
	// Visibility is the requested visibility; the author's visibility ceiling may narrow it.
	Visibility content.Visibility
	// InReplyToID is the ID of the status replied to, "" for none (FR-106).
	InReplyToID string
	// ClientID is the OAuth client that posts, "" when none is recorded.
	ClientID string
	// IdempotencyKey is the client API's Idempotency-Key, "" for none. A key that the author
	// already used returns the status posted with it.
	IdempotencyKey string
}

// Create posts a local status. The text becomes HTML with mentions of local, active accounts,
// hashtags, and URLs linked; the visibility is capped by the author's visibility ceiling
// (data-model §5.4) and stored as capped. A reply must target a status the author may see.
//
// Retrying with an IdempotencyKey that the author already used returns the status posted with it
// and changes nothing; when that status is deleted, it fails with ErrAlreadyDeleted.
func (s *Service) Create(ctx context.Context, in CreateInput) (Post, error) {
	if !in.Visibility.Valid() {
		return Post{}, fmt.Errorf("%w: %q", content.ErrInvalidVisibility, in.Visibility)
	}
	author, err := s.localActive(ctx, in.AuthorAccountID)
	if err != nil {
		return Post{}, err
	}
	if in.IdempotencyKey != "" {
		if post, err := s.replay(ctx, in.AuthorAccountID, in.IdempotencyKey); !errors.Is(err, ErrNotFound) {
			return post, err
		}
	}
	restrictions, err := s.postingRestrictions(ctx, in.AuthorAccountID)
	if err != nil {
		return Post{}, err
	}
	body, err := s.prepareText(ctx, author, in.TextInput)
	if err != nil {
		return Post{}, err
	}

	id := s.d.IDs.NewID()
	now := s.d.Clock.Now()
	st := content.Status{
		ID:              id,
		URI:             content.StatusURI(s.d.Host, id),
		URL:             content.StatusURL(s.d.Host, author.Handle.Username, id),
		AuthorAccountID: in.AuthorAccountID,
		IsLocal:         true,
		Visibility:      content.Narrower(in.Visibility, restrictions.VisibilityCeiling()),
		PublishedAt:     now,
	}
	st.ConversationURI = st.URI
	if in.InReplyToID != "" {
		parent, err := s.replyTarget(ctx, in.AuthorAccountID, in.InReplyToID)
		if err != nil {
			return Post{}, err
		}
		st.InReplyToURI = parent.URI
		st.ConversationURI = cmp.Or(parent.ConversationURI, parent.URI)
	}
	local := content.LocalStatus{
		StatusID: id, AuthorAccountID: in.AuthorAccountID, ClientID: in.ClientID, IdempotencyKey: in.IdempotencyKey,
	}
	rev := body.revision(s.d.IDs.NewID(), id, 1, now)
	tags := body.tags(s.d.IDs, now)

	repo := s.d.Repository
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := repo.InsertStatus(ctx, tx, st); err != nil {
			return err
		}
		if err := repo.InsertLocalStatus(ctx, tx, local); err != nil {
			return err
		}
		var err error
		if rev.TagIDs, err = ensureTags(ctx, tx, repo, tags); err != nil {
			return err
		}
		if err := repo.InsertRevision(ctx, tx, rev); err != nil {
			return err
		}
		return tx.Emit(ctx, StatusCreated{Status: st, Revision: rev})
	})
	if errors.Is(err, ErrIdempotencyKeyUsed) {
		// A concurrent request with the same key won.
		return s.replay(ctx, in.AuthorAccountID, in.IdempotencyKey)
	}
	if err != nil {
		return Post{}, err
	}
	return Post{Status: st, Revision: rev}, nil
}

// replay returns the post that an author posted with an Idempotency-Key.
func (s *Service) replay(ctx context.Context, authorID, key string) (Post, error) {
	ls, err := s.d.Repository.FindLocalStatusByIdempotencyKey(ctx, authorID, key)
	if err != nil {
		return Post{}, err
	}
	post, err := s.Find(ctx, ls.StatusID)
	if errors.Is(err, ErrNotFound) {
		return Post{}, fmt.Errorf("status %s: %w", ls.StatusID, ErrAlreadyDeleted)
	}
	return post, err
}

// replyTarget returns the status that authorID replies to, provided that authorID may see it.
// A status that does not exist, is deleted, or is hidden from the author fails alike with
// ErrReplyTargetNotFound, so that a reply does not reveal it.
func (s *Service) replyTarget(ctx context.Context, authorID, statusID string) (content.Status, error) {
	notFound := fmt.Errorf("status %s: %w", statusID, ErrReplyTargetNotFound)
	parent, err := s.d.Repository.FindStatus(ctx, statusID)
	if errors.Is(err, ErrNotFound) {
		return content.Status{}, notFound
	}
	if err != nil {
		return content.Status{}, err
	}
	rev, err := s.d.Repository.FindCurrentRevision(ctx, statusID)
	if errors.Is(err, ErrNotFound) {
		return content.Status{}, notFound // deleted
	}
	if err != nil {
		return content.Status{}, err
	}
	restrictions, err := s.d.Restrictions.Restrictions(ctx, parent.AuthorAccountID)
	if err != nil {
		return content.Status{}, err
	}
	rel := content.Relation{Mentioned: slices.Contains(rev.MentionedAccountIDs, authorID)}
	if parent.AuthorAccountID != authorID {
		following, err := s.d.Follows.IsFollowing(ctx, authorID, parent.AuthorAccountID)
		if err != nil {
			return content.Status{}, err
		}
		if following {
			rel.Follow = content.FollowActive
		}
	}
	viewer := content.Viewer{Kind: content.ViewerLocal, AccountID: authorID}
	facts := content.StatusFacts{AuthorAccountID: parent.AuthorAccountID, AuthorIsLocal: parent.IsLocal, Visibility: parent.Visibility}
	if !content.CanView(content.ContextStatus, viewer, facts, restrictions.Author, rel) {
		return content.Status{}, notFound
	}
	return parent, nil
}

// EditInput describes an edit of a local status.
type EditInput struct {
	TextInput
	// AccountID is the account editing; it must be the author.
	AccountID string
	StatusID  string
}

// Edit records a new revision of a local status with new content (FR-109). The visibility,
// author, and reply target never change. The new revision becomes the head. A status that does
// not exist or is deleted fails with ErrNotFound; another account's status with ErrNotAuthor.
func (s *Service) Edit(ctx context.Context, in EditInput) (Post, error) {
	author, err := s.localActive(ctx, in.AccountID)
	if err != nil {
		return Post{}, err
	}
	st, err := s.ownStatus(ctx, in.AccountID, in.StatusID)
	if err != nil {
		return Post{}, err
	}
	if _, err := s.postingRestrictions(ctx, in.AccountID); err != nil {
		return Post{}, err
	}
	body, err := s.prepareText(ctx, author, in.TextInput)
	if err != nil {
		return Post{}, err
	}
	now := s.d.Clock.Now()
	rev := body.revision(s.d.IDs.NewID(), st.ID, 0, now)
	tags := body.tags(s.d.IDs, now)

	repo := s.d.Repository
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		deleted, err := repo.IsDeleted(ctx, tx, st.ID)
		if err != nil {
			return err
		}
		if deleted {
			return fmt.Errorf("status %s: %w", st.ID, ErrNotFound)
		}
		head, err := repo.Head(ctx, tx, st.ID)
		if err != nil {
			return err
		}
		rev.Seq = head.Seq + 1
		if rev.TagIDs, err = ensureTags(ctx, tx, repo, tags); err != nil {
			return err
		}
		if err := repo.InsertRevision(ctx, tx, rev); err != nil {
			return err
		}
		return tx.Emit(ctx, StatusRevised{Status: st, Revision: rev})
	})
	if err != nil {
		return Post{}, err
	}
	return Post{Status: st, Revision: rev}, nil
}

// Delete deletes a local status by its author (FR-108). It records the deletion, which stays as
// the tombstone of the URI, and removes the status from every projection at once. A status that
// does not exist or is already deleted fails with ErrNotFound; another account's status with
// ErrNotAuthor.
func (s *Service) Delete(ctx context.Context, accountID, statusID string) (content.Deletion, error) {
	st, err := s.ownStatus(ctx, accountID, statusID)
	if err != nil {
		return content.Deletion{}, err
	}
	d := content.Deletion{
		StatusID: st.ID, URI: st.URI, AuthorAccountID: st.AuthorAccountID, Origin: content.DeletedByAuthor, DeletedAt: s.d.Clock.Now(),
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertDeletion(ctx, tx, d); err != nil {
			return err
		}
		return tx.Emit(ctx, StatusDeleted{Deletion: d, Status: st})
	})
	if errors.Is(err, ErrAlreadyDeleted) {
		return content.Deletion{}, fmt.Errorf("status %s: %w", st.ID, ErrNotFound)
	}
	if err != nil {
		return content.Deletion{}, err
	}
	return d, nil
}

// Find returns a status that is not deleted with its current revision. It does not decide who
// may see the status; callers check content.CanView.
func (s *Service) Find(ctx context.Context, statusID string) (Post, error) {
	st, err := s.d.Repository.FindStatus(ctx, statusID)
	if err != nil {
		return Post{}, err
	}
	rev, err := s.d.Repository.FindCurrentRevision(ctx, statusID)
	if err != nil {
		return Post{}, err
	}
	return Post{Status: st, Revision: rev}, nil
}

// Revisions returns the revisions of a status, oldest first, without their children: the edit
// history (FR-109). Like Find, it does not decide who may see them.
func (s *Service) Revisions(ctx context.Context, statusID string) ([]content.Revision, error) {
	return s.d.Repository.ListRevisions(ctx, statusID)
}

// localActive returns a local, active account with a handle.
func (s *Service) localActive(ctx context.Context, id string) (appaccount.Actor, error) {
	a, err := s.d.Accounts.FindByID(ctx, id)
	if errors.Is(err, appaccount.ErrNotFound) {
		return appaccount.Actor{}, fmt.Errorf("account %s: %w", id, ErrAccountUnavailable)
	}
	if err != nil {
		return appaccount.Actor{}, err
	}
	if !a.Account.IsLocal() {
		return appaccount.Actor{}, fmt.Errorf("account %s: %w", id, ErrRemoteAccount)
	}
	if !a.Lifecycle.Active() || a.Handle == nil || a.Local == nil {
		return appaccount.Actor{}, fmt.Errorf("account %s: %w", id, ErrAccountUnavailable)
	}
	return a, nil
}

// ownStatus returns a local status of accountID.
func (s *Service) ownStatus(ctx context.Context, accountID, statusID string) (content.Status, error) {
	st, err := s.d.Repository.FindStatus(ctx, statusID)
	if err != nil {
		return content.Status{}, err
	}
	if st.AuthorAccountID != accountID || !st.IsLocal {
		return content.Status{}, fmt.Errorf("status %s: %w", statusID, ErrNotAuthor)
	}
	return st, nil
}

func (s *Service) postingRestrictions(ctx context.Context, accountID string) (Restrictions, error) {
	r, err := s.d.Restrictions.Restrictions(ctx, accountID)
	if err != nil {
		return Restrictions{}, err
	}
	if !r.CanPost() {
		return Restrictions{}, fmt.Errorf("account %s: %w", accountID, ErrPostingRestricted)
	}
	return r, nil
}

// preparedText is validated, rendered content, ready to be stored as a revision.
type preparedText struct {
	in       TextInput
	language string
	html     string
	mentions []string // account IDs, sorted
	hashtags []Hashtag
}

// CountCharacters returns the length of a post as max_status_length counts it: the characters
// (Unicode code points) of the text and the content warning.
func CountCharacters(text, spoilerText string) int {
	return utf8.RuneCountInString(text) + utf8.RuneCountInString(spoilerText)
}

// prepareText validates the content against the instance settings, resolves its mentions to
// local, active accounts, and renders it.
func (s *Service) prepareText(ctx context.Context, author appaccount.Actor, in TextInput) (preparedText, error) {
	for _, v := range []string{in.Text, in.SpoilerText} {
		if !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
			return preparedText{}, ErrInvalidText
		}
	}
	if strings.TrimSpace(in.Text) == "" {
		return preparedText{}, ErrEmptyText
	}
	limit, err := s.d.MaxStatusLength(ctx)
	if err != nil {
		return preparedText{}, err
	}
	if n := CountCharacters(in.Text, in.SpoilerText); n > limit {
		return preparedText{}, fmt.Errorf("%w: %d characters, at most %d", ErrTextTooLong, n, limit)
	}
	language, err := s.language(ctx, author, in.Language)
	if err != nil {
		return preparedText{}, err
	}

	parsed := ParseText(in.Text)
	targets := map[MentionHandle]MentionTarget{}
	var mentions []string
	for _, h := range parsed.Mentions() {
		a, ok, err := s.mentioned(ctx, h)
		if err != nil {
			return preparedText{}, err
		}
		if !ok {
			continue
		}
		targets[h] = MentionTarget{Username: a.Handle.Username, URL: profileURL(s.d.Host, a.Handle.Username)}
		if !slices.Contains(mentions, a.Account.ID) {
			mentions = append(mentions, a.Account.ID)
		}
	}
	slices.Sort(mentions)
	return preparedText{
		in:       in,
		language: language,
		html:     parsed.HTML(RenderOptions{Host: s.d.Host, Mentions: targets}),
		mentions: mentions,
		hashtags: parsed.Hashtags(),
	}, nil
}

// mentioned resolves a mentioned handle to a local, active account. Handles of other domains
// are left to the federation module and stay text.
func (s *Service) mentioned(ctx context.Context, h MentionHandle) (appaccount.Actor, bool, error) {
	if h.Domain != "" && h.Domain != strings.ToLower(s.d.AccountDomain) && h.Domain != strings.ToLower(s.d.Host) {
		return appaccount.Actor{}, false, nil
	}
	// "" is account.LocalDomain, the domain of local handles.
	a, err := s.d.Accounts.FindByHandle(ctx, h.Username, "")
	if errors.Is(err, appaccount.ErrNotFound) {
		return appaccount.Actor{}, false, nil
	}
	if err != nil {
		return appaccount.Actor{}, false, err
	}
	ok := a.Account.IsLocal() && a.Lifecycle.Active() && a.Handle != nil
	return a, ok, nil
}

// language canonicalizes the requested language, falling back to the author's default.
func (s *Service) language(ctx context.Context, author appaccount.Actor, requested string) (string, error) {
	if requested == "" && s.d.DefaultLanguage != nil {
		def, err := s.d.DefaultLanguage(ctx, author.Local.UserID)
		if err != nil {
			return "", err
		}
		requested = def
	}
	if requested == "" {
		return UndeterminedLanguage, nil
	}
	tag := i18n.Canonicalize(requested)
	if tag == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidLanguage, requested)
	}
	return tag, nil
}

// revision returns the revision holding the prepared content; its TagIDs are set in the
// transaction.
func (p preparedText) revision(id, statusID string, seq int, at time.Time) content.Revision {
	return content.Revision{
		ID:                  id,
		StatusID:            statusID,
		Seq:                 seq,
		SpoilerText:         p.in.SpoilerText,
		Sensitive:           p.in.Sensitive,
		Language:            p.language,
		RevisedAt:           at,
		Bodies:              []content.Body{{Language: p.language, ContentHTML: p.html}},
		Source:              &content.Source{Text: p.in.Text, Format: content.SourceTextPlain},
		MentionedAccountIDs: p.mentions,
	}
}

// tags returns the tags to ensure, with IDs generated outside the transaction.
func (p preparedText) tags(ids kernel.IDGenerator, at time.Time) []content.Tag {
	out := make([]content.Tag, 0, len(p.hashtags))
	for _, h := range p.hashtags {
		out = append(out, content.Tag{ID: ids.NewID(), Name: h.Name, NameNormalized: h.NameNormalized, CreatedAt: at})
	}
	return out
}

// ensureTags stores the tags that do not exist yet and returns the IDs of all of them, sorted.
func ensureTags(ctx context.Context, tx uow.Tx, repo Repository, tags []content.Tag) ([]string, error) {
	var ids []string
	for _, t := range tags {
		stored, err := repo.EnsureTag(ctx, tx, t)
		if err != nil {
			return nil, err
		}
		ids = append(ids, stored.ID)
	}
	slices.Sort(ids)
	return ids, nil
}

// profileURL returns the HTML profile page of a local account on host (ADR-0018).
func profileURL(host, username string) string { return "https://" + host + "/@" + username }
