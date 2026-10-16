// SPDX-License-Identifier: AGPL-3.0-only

package account

import (
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/account"
)

// Deps are the ports and settings a Service uses.
type Deps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	Clock      kernel.Clock
	IDs        kernel.IDGenerator
	// Sealer encrypts actor private keys for storage.
	Sealer *kernel.Sealer
	// Host is the serving domain that every local actor URI uses (ADR-0018).
	Host string
	// GenerateKey generates actor keys; nil means GenerateKey.
	GenerateKey KeyGenerator
}

// Service runs the account module's use cases.
type Service struct {
	d Deps
}

// NewService returns a Service on d.
func NewService(d Deps) *Service {
	if d.GenerateKey == nil {
		d.GenerateKey = GenerateKey
	}
	return &Service{d: d}
}

// LocalAccountInput describes a new local account.
type LocalAccountInput struct {
	// UserID is the identity module's user that logs in as the account.
	UserID   string
	Username string
	// DisplayName is the display name of the first profile revision; it may be empty.
	DisplayName string
}

// PreparedLocalAccount is a local account with its IDs, time, and sealed keys fixed, ready to
// be written by WriteLocalAccount. Preparing does the slow and external work (key generation,
// sealing through the KeyManager) outside the transaction, so that a retried transaction
// writes the same values.
type PreparedLocalAccount struct {
	Account account.Account
	Local   account.LocalAccount
	Profile account.ProfileRevision
	keys    []preparedKey
}

type preparedKey struct {
	key    account.Key
	sealed []byte
}

// PrepareLocalAccount validates in and generates everything a new local account needs.
// It writes nothing.
func (s *Service) PrepareLocalAccount(ctx context.Context, in LocalAccountInput) (*PreparedLocalAccount, error) {
	if in.UserID == "" {
		return nil, errors.New("account: a local account needs a user")
	}
	if err := account.ValidateUsername(in.Username); err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	a := account.Account{ID: s.d.IDs.NewID(), Domain: account.LocalDomain, ActorType: account.ActorPerson, CreatedAt: now}
	a.URI = account.ActorURI(s.d.Host, a.ID)
	keys, err := s.prepareKeys(ctx, a, now)
	if err != nil {
		return nil, err
	}
	return &PreparedLocalAccount{
		Account: a,
		Local: account.LocalAccount{
			AccountID:          a.ID,
			UserID:             in.UserID,
			Username:           in.Username,
			UsernameNormalized: account.NormalizeUsername(in.Username),
		},
		Profile: account.ProfileRevision{
			ID:          s.d.IDs.NewID(),
			AccountID:   a.ID,
			Seq:         1,
			DisplayName: in.DisplayName,
			RevisedAt:   now,
		},
		keys: keys,
	}, nil
}

func (s *Service) prepareKeys(ctx context.Context, a account.Account, now time.Time) ([]preparedKey, error) {
	out := make([]preparedKey, 0, len(LocalKeyAlgorithms))
	for _, alg := range LocalKeyAlgorithms {
		priv, err := s.d.GenerateKey(alg)
		if err != nil {
			return nil, fmt.Errorf("account: generate %s key: %w", alg, err)
		}
		if err := checkAlgorithm(alg, priv); err != nil {
			return nil, err
		}
		pubPEM, err := encodePublicKey(priv.Public())
		if err != nil {
			return nil, fmt.Errorf("account: encode %s public key: %w", alg, err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return nil, fmt.Errorf("account: encode %s private key: %w", alg, err)
		}
		k := account.Key{
			ID:           s.d.IDs.NewID(),
			AccountID:    a.ID,
			KeyID:        account.KeyIRI(a.URI, alg),
			Algorithm:    alg,
			PublicKeyPEM: pubPEM,
			CreatedAt:    now,
		}
		sealed, err := s.d.Sealer.Seal(ctx, der, privateKeyAAD(k.ID))
		if err != nil {
			return nil, fmt.Errorf("account: seal %s private key: %w", alg, err)
		}
		out = append(out, preparedKey{key: k, sealed: sealed})
	}
	return out, nil
}

// WriteLocalAccount writes a prepared local account inside tx and emits its events, so that
// another module's command (such as user registration) can create the account in its own
// transaction. It fails with ErrUsernameTaken when the username is taken.
func (s *Service) WriteLocalAccount(ctx context.Context, tx uow.Tx, p *PreparedLocalAccount) error {
	repo := s.d.Repository
	if err := repo.InsertAccount(ctx, tx, p.Account); err != nil {
		return err
	}
	if err := repo.InsertLocalAccount(ctx, tx, p.Local); err != nil {
		return err
	}
	if err := writeKeys(ctx, tx, repo, p.keys); err != nil {
		return err
	}
	if err := repo.InsertProfileRevision(ctx, tx, p.Profile); err != nil {
		return err
	}
	if err := tx.Emit(ctx, LocalAccountCreated{Account: p.Account, Local: p.Local}); err != nil {
		return err
	}
	return tx.Emit(ctx, ProfileRevised{Revision: p.Profile})
}

func writeKeys(ctx context.Context, tx uow.Tx, repo Repository, keys []preparedKey) error {
	for _, k := range keys {
		if err := repo.InsertKey(ctx, tx, k.key, k.sealed); err != nil {
			return err
		}
	}
	return nil
}

// CreateLocalAccount creates a local account for an existing user, with RSA and Ed25519 keys
// and an initial profile revision.
func (s *Service) CreateLocalAccount(ctx context.Context, in LocalAccountInput) (account.Account, error) {
	p, err := s.PrepareLocalAccount(ctx, in)
	if err != nil {
		return account.Account{}, err
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.WriteLocalAccount(ctx, tx, p) })
	if err != nil {
		return account.Account{}, err
	}
	return p.Account, nil
}

// EnsureInstanceActor returns the instance actor (the local Application actor that signs the
// instance's own requests), creating it with its keys on first use. Concurrent calls agree on
// one actor.
func (s *Service) EnsureInstanceActor(ctx context.Context) (account.Account, error) {
	a, err := s.d.Repository.FindInstanceActor(ctx)
	if !errors.Is(err, ErrNotFound) {
		return a, err
	}
	now := s.d.Clock.Now()
	a = account.Account{ID: s.d.IDs.NewID(), Domain: account.LocalDomain, ActorType: account.ActorApplication, CreatedAt: now}
	a.URI = account.ActorURI(s.d.Host, a.ID)
	keys, err := s.prepareKeys(ctx, a, now)
	if err != nil {
		return account.Account{}, err
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertAccount(ctx, tx, a); err != nil {
			return err
		}
		return writeKeys(ctx, tx, s.d.Repository, keys)
	})
	if errors.Is(err, ErrInstanceActorExists) {
		return s.d.Repository.FindInstanceActor(ctx)
	}
	if err != nil {
		return account.Account{}, err
	}
	return a, nil
}

// ProfileInput is the content of a new profile revision.
type ProfileInput struct {
	DisplayName   string
	NoteHTML      string
	Locked        bool
	Bot           bool
	Discoverable  bool
	Indexable     bool
	AvatarMediaID string
	HeaderMediaID string
	// Fields are numbered by their order.
	Fields []account.ProfileField
}

// IsMediaReferenced reports whether any profile revision, current or not, uses a media file as
// its avatar or header. It implements content.MediaReferrer, so that the cleanup of unattached
// uploads keeps profile images.
func (s *Service) IsMediaReferenced(ctx context.Context, mediaID string) (bool, error) {
	return s.d.Repository.IsMediaReferenced(ctx, mediaID)
}

// ReviseProfile records a new revision of an account's profile and makes it current.
func (s *Service) ReviseProfile(ctx context.Context, accountID string, in ProfileInput) (account.ProfileRevision, error) {
	base := account.ProfileRevision{
		ID:            s.d.IDs.NewID(),
		AccountID:     accountID,
		DisplayName:   in.DisplayName,
		NoteHTML:      in.NoteHTML,
		Locked:        in.Locked,
		Bot:           in.Bot,
		Discoverable:  in.Discoverable,
		Indexable:     in.Indexable,
		AvatarMediaID: in.AvatarMediaID,
		HeaderMediaID: in.HeaderMediaID,
		RevisedAt:     s.d.Clock.Now(),
	}
	for i, f := range in.Fields {
		f.Position = i
		base.Fields = append(base.Fields, f)
	}
	var rev account.ProfileRevision
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		rev = base
		head, err := s.d.Repository.ProfileHead(ctx, tx, accountID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		rev.Seq = head.Seq + 1
		if err := s.d.Repository.InsertProfileRevision(ctx, tx, rev); err != nil {
			return err
		}
		return tx.Emit(ctx, ProfileRevised{Revision: rev})
	})
	if err != nil {
		return account.ProfileRevision{}, err
	}
	return rev, nil
}

// LifecycleInput describes a lifecycle event to record.
type LifecycleInput struct {
	AccountID          string
	Kind               account.LifecycleKind
	InitiatedBy        account.Initiator
	ModerationActionID string
	// DeletionDueAt is required for account.LifecycleDeletionScheduled and forbidden otherwise.
	DeletionDueAt *time.Time
}

// RecordLifecycleEvent records a lifecycle event of an account and updates its lifecycle state.
func (s *Service) RecordLifecycleEvent(ctx context.Context, in LifecycleInput) (account.LifecycleEvent, error) {
	if (in.Kind == account.LifecycleDeletionScheduled) != (in.DeletionDueAt != nil) {
		return account.LifecycleEvent{}, errors.New("account: a deletion due time is required for, and only for, deletion_scheduled")
	}
	e := account.LifecycleEvent{
		ID:                 s.d.IDs.NewID(),
		AccountID:          in.AccountID,
		Kind:               in.Kind,
		InitiatedBy:        in.InitiatedBy,
		ModerationActionID: in.ModerationActionID,
		OccurredAt:         s.d.Clock.Now(),
	}
	if in.DeletionDueAt != nil {
		due := kernel.Normalize(*in.DeletionDueAt)
		e.DeletionDueAt = &due
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertLifecycleEvent(ctx, tx, e); err != nil {
			return err
		}
		return tx.Emit(ctx, LifecycleEventRecorded{Event: e})
	})
	if err != nil {
		return account.LifecycleEvent{}, err
	}
	return e, nil
}

// Actor is an account with what is needed to present it: its handle and current profile
// (when it has them), public keys, and lifecycle state.
type Actor struct {
	Account   account.Account
	Local     *account.LocalAccount // nil unless a local account (the instance actor has none)
	Handle    *account.Handle
	Profile   *account.ProfileRevision
	Keys      []account.Key
	Lifecycle account.LifecycleState
}

// FindByID returns the actor with an account ID. It fails with ErrNotFound when there is none.
func (s *Service) FindByID(ctx context.Context, id string) (Actor, error) {
	a, err := s.d.Repository.FindAccount(ctx, id)
	if err != nil {
		return Actor{}, err
	}
	return s.actor(ctx, a)
}

// FindByHandle returns the actor with a handle. username compares case-insensitively; domain is
// account.LocalDomain for local accounts. It fails with ErrNotFound when there is none.
func (s *Service) FindByHandle(ctx context.Context, username, domain string) (Actor, error) {
	h, err := s.d.Repository.FindHandle(ctx, account.NormalizeUsername(username), account.NormalizeDomain(domain))
	if err != nil {
		return Actor{}, err
	}
	return s.FindByID(ctx, h.AccountID)
}

// InstanceActor returns the instance actor. It fails with ErrNotFound before EnsureInstanceActor.
func (s *Service) InstanceActor(ctx context.Context) (Actor, error) {
	a, err := s.d.Repository.FindInstanceActor(ctx)
	if err != nil {
		return Actor{}, err
	}
	return s.actor(ctx, a)
}

func (s *Service) actor(ctx context.Context, a account.Account) (Actor, error) {
	repo := s.d.Repository
	out := Actor{Account: a, Lifecycle: account.InitialLifecycleState(a.ID)}
	if a.IsLocal() && a.ActorType != account.ActorApplication {
		la, err := repo.FindLocalAccount(ctx, a.ID)
		if err != nil {
			return Actor{}, err
		}
		out.Local = &la
	}
	h, err := repo.FindHandleOf(ctx, a.ID)
	switch {
	case err == nil:
		out.Handle = &h
	case !errors.Is(err, ErrNotFound):
		return Actor{}, err
	}
	p, err := repo.FindCurrentProfile(ctx, a.ID)
	switch {
	case err == nil:
		out.Profile = &p
	case !errors.Is(err, ErrNotFound):
		return Actor{}, err
	}
	if out.Keys, err = repo.ListKeys(ctx, a.ID); err != nil {
		return Actor{}, err
	}
	st, err := repo.FindLifecycleState(ctx, a.ID)
	switch {
	case err == nil:
		out.Lifecycle = st
	case !errors.Is(err, ErrNotFound):
		return Actor{}, err
	}
	return out, nil
}

// CountLocalUsers returns the number of local accounts of users that are not deleted, for
// NodeInfo's usage statistics. The instance actor is not counted.
func (s *Service) CountLocalUsers(ctx context.Context) (int, error) {
	return s.d.Repository.CountLocalAccounts(ctx)
}

// PrivateKey opens the sealed private key of a local actor's key.
func (s *Service) PrivateKey(ctx context.Context, k account.Key) (crypto.Signer, error) {
	sealed, err := s.d.Repository.FindSealedPrivateKey(ctx, k.ID)
	if err != nil {
		return nil, err
	}
	der, err := s.d.Sealer.Open(ctx, sealed, privateKeyAAD(k.ID))
	if err != nil {
		return nil, fmt.Errorf("account: open private key %s: %w", k.KeyID, err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("account: parse private key %s: %w", k.KeyID, err)
	}
	signer, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("account: private key %s is a %T", k.KeyID, parsed)
	}
	if err := checkAlgorithm(k.Algorithm, signer); err != nil {
		return nil, err
	}
	return signer, nil
}
