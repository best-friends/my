// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"context"
	"errors"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/graph"
)

var (
	// ErrAccountUnavailable reports a follower or followee that does not exist or is not active.
	ErrAccountUnavailable = errors.New("graph: the account does not exist or is not active")
	// ErrRemoteAccount reports a remote follower or followee: these use cases cover local
	// accounts only, and federated follows go through the federation module.
	ErrRemoteAccount = errors.New("graph: the account is not local")
)

// Accounts is the part of the account module's public API that the graph module reads: an
// account's locality, lifecycle state, and current profile (whose locked flag decides whether a
// follow needs approval). *appaccount.Service implements it.
type Accounts interface {
	FindByID(ctx context.Context, id string) (appaccount.Actor, error)
}

// Deps are the ports and settings a Service uses.
type Deps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	Accounts   Accounts
	Clock      kernel.Clock
	IDs        kernel.IDGenerator
	// Host is the serving domain of the Follow activity URIs (ADR-0018).
	Host string
}

// Service runs the graph module's follow use cases between local accounts.
type Service struct {
	d Deps
}

// NewService returns a Service on d.
func NewService(d Deps) *Service { return &Service{d: d} }

// Relationship is the follow relationship between an account and a target account, as the
// account sees it.
type Relationship struct {
	AccountID       string
	TargetAccountID string
	// Following is the state of the account's follow of the target; StatePending means a
	// follow request awaits the target's approval.
	Following graph.State
	// FollowedBy is the state of the target's follow of the account.
	FollowedBy graph.State
	// Preferences are those of the account's follow of the target, or the defaults when the
	// account does not follow the target.
	ShowBoosts bool
	Notify     bool
}

// Follows reports whether the account actively follows the target. Other modules read the
// follow state through it and RequestedFollow, without the graph module's domain types.
func (r Relationship) Follows() bool { return r.Following == graph.StateActive }

// RequestedFollow reports whether the account's follow of the target awaits approval.
func (r Relationship) RequestedFollow() bool { return r.Following == graph.StatePending }

// Follow makes followerID follow followeeID. The follow is active at once, unless the followee's
// current profile is locked: then it is a follow request, pending until the followee accepts it
// (FR-302). Following an account that the follower already follows or has requested to follow
// changes nothing. Both accounts must be local and active.
func (s *Service) Follow(ctx context.Context, followerID, followeeID string) (Relationship, error) {
	f := graph.Follow{FollowerAccountID: followerID, FolloweeAccountID: followeeID}
	if err := f.Validate(); err != nil {
		return Relationship{}, err
	}
	if _, err := s.localActive(ctx, followerID); err != nil {
		return Relationship{}, err
	}
	followee, err := s.localActive(ctx, followeeID)
	if err != nil {
		return Relationship{}, err
	}
	locked := followee.Profile != nil && followee.Profile.Locked

	f.ID = s.d.IDs.NewID()
	f.URI = graph.FollowURI(s.d.Host, f.ID)
	f.RequestedAt = s.d.Clock.Now()
	prefs := graph.DefaultPreferences(f.ID)
	acceptance := graph.Acceptance{FollowID: f.ID, AcceptedBy: graph.AcceptedByAuto, AcceptedAt: f.RequestedAt}

	repo := s.d.Repository
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if _, err := repo.Edge(ctx, tx, followerID, followeeID); !errors.Is(err, ErrNotFound) {
			return err // nil: already following or requested
		}
		if err := repo.InsertFollow(ctx, tx, f); err != nil {
			return err
		}
		if err := repo.UpsertPreferences(ctx, tx, prefs); err != nil {
			return err
		}
		if locked {
			return tx.Emit(ctx, FollowRequested{Follow: f, Preferences: prefs})
		}
		if err := repo.InsertAcceptance(ctx, tx, acceptance); err != nil {
			return err
		}
		return tx.Emit(ctx, Followed{Follow: f, Acceptance: acceptance, Preferences: prefs})
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, followerID, followeeID)
}

// localActive returns a local, active account.
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
	if !a.Lifecycle.Active() {
		return appaccount.Actor{}, fmt.Errorf("account %s: %w", id, ErrAccountUnavailable)
	}
	return a, nil
}

// AcceptFollowRequest accepts the pending follow request from followerID to followeeID, made by
// the followee. Accepting a follow that is already active changes nothing; without a follow it
// fails with ErrNotFound.
func (s *Service) AcceptFollowRequest(ctx context.Context, followeeID, followerID string) (Relationship, error) {
	now := s.d.Clock.Now()
	repo := s.d.Repository
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		edge, f, err := s.liveFollow(ctx, tx, followerID, followeeID)
		if err != nil || edge.State == graph.StateActive {
			return err
		}
		a := graph.Acceptance{FollowID: f.ID, AcceptedBy: graph.AcceptedByFollowee, AcceptedAt: now}
		if err := repo.InsertAcceptance(ctx, tx, a); err != nil {
			return err
		}
		prefs, err := repo.Preferences(ctx, tx, f.ID)
		if err != nil {
			return err
		}
		return tx.Emit(ctx, FollowAccepted{Follow: f, Acceptance: a, Preferences: prefs})
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, followeeID, followerID)
}

// RejectFollowRequest declines the pending follow request from followerID to followeeID, made by
// the followee. Without a pending request it fails with ErrNotFound; to end an active follow,
// use RemoveFollower.
func (s *Service) RejectFollowRequest(ctx context.Context, followeeID, followerID string) (Relationship, error) {
	now := s.d.Clock.Now()
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		edge, f, err := s.liveFollow(ctx, tx, followerID, followeeID)
		if err != nil {
			return err
		}
		if edge.State != graph.StatePending {
			return fmt.Errorf("follow request from %s to %s: %w", followerID, followeeID, ErrNotFound)
		}
		r := graph.Rejection{FollowID: f.ID, RejectedBy: graph.RejectedByFollowee, RejectedAt: now}
		if err := s.d.Repository.InsertRejection(ctx, tx, r); err != nil {
			return err
		}
		return tx.Emit(ctx, FollowRejected{Follow: f, Rejection: r})
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, followeeID, followerID)
}

// Unfollow ends followerID's follow of followeeID, or withdraws the pending follow request.
// Without a follow it changes nothing.
func (s *Service) Unfollow(ctx context.Context, followerID, followeeID string) (Relationship, error) {
	if err := s.terminate(ctx, followerID, followeeID, graph.TerminatedUnfollowed, false); err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, followerID, followeeID)
}

// RemoveFollower ends followerID's active follow of followeeID, made by the followee. Without an
// active follow it changes nothing; to decline a pending request, use RejectFollowRequest.
func (s *Service) RemoveFollower(ctx context.Context, followeeID, followerID string) (Relationship, error) {
	if err := s.terminate(ctx, followerID, followeeID, graph.TerminatedRemovedByFollowee, true); err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, followeeID, followerID)
}

func (s *Service) terminate(ctx context.Context, followerID, followeeID string, reason graph.TerminationReason, activeOnly bool) error {
	now := s.d.Clock.Now()
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		edge, f, err := s.liveFollow(ctx, tx, followerID, followeeID)
		if errors.Is(err, ErrNotFound) || (err == nil && activeOnly && edge.State != graph.StateActive) {
			return nil
		}
		if err != nil {
			return err
		}
		t := graph.Termination{FollowID: f.ID, Reason: reason, TerminatedAt: now}
		if err := s.d.Repository.InsertTermination(ctx, tx, t); err != nil {
			return err
		}
		return tx.Emit(ctx, FollowTerminated{Follow: f, Termination: t, PreviousState: edge.State})
	})
}

// liveFollow returns the edge from follower to followee and its follow; ErrNotFound means none.
func (s *Service) liveFollow(ctx context.Context, tx uow.Tx, followerID, followeeID string) (graph.Edge, graph.Follow, error) {
	edge, err := s.d.Repository.Edge(ctx, tx, followerID, followeeID)
	if err != nil {
		return graph.Edge{}, graph.Follow{}, err
	}
	f, err := s.d.Repository.Follow(ctx, tx, edge.FollowID)
	if err != nil {
		return graph.Edge{}, graph.Follow{}, err
	}
	return edge, f, nil
}

// PreferencesInput is the follower's settings of a follow.
type PreferencesInput struct {
	ShowBoosts bool
	Notify     bool
}

// UpdatePreferences changes the preferences of followerID's pending or active follow of
// followeeID. Without a follow it fails with ErrNotFound.
func (s *Service) UpdatePreferences(ctx context.Context, followerID, followeeID string, in PreferencesInput) (Relationship, error) {
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		edge, f, err := s.liveFollow(ctx, tx, followerID, followeeID)
		if err != nil {
			return err
		}
		p := graph.Preferences{FollowID: f.ID, ShowBoosts: in.ShowBoosts, Notify: in.Notify}
		if err := s.d.Repository.UpsertPreferences(ctx, tx, p); err != nil {
			return err
		}
		return tx.Emit(ctx, FollowPreferencesChanged{Follow: f, State: edge.State, Preferences: p})
	})
	if err != nil {
		return Relationship{}, err
	}
	return s.Relationship(ctx, followerID, followeeID)
}

// Relationship returns the follow relationship between accountID and targetID.
func (s *Service) Relationship(ctx context.Context, accountID, targetID string) (Relationship, error) {
	repo := s.d.Repository
	out := Relationship{AccountID: accountID, TargetAccountID: targetID, Following: graph.StateNone, FollowedBy: graph.StateNone}
	defaults := graph.DefaultPreferences("")
	out.ShowBoosts, out.Notify = defaults.ShowBoosts, defaults.Notify

	outgoing, err := repo.FindEdge(ctx, accountID, targetID)
	switch {
	case err == nil:
		out.Following = outgoing.State
		p, err := repo.FindPreferences(ctx, outgoing.FollowID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return Relationship{}, err
		}
		if err == nil {
			out.ShowBoosts, out.Notify = p.ShowBoosts, p.Notify
		}
	case !errors.Is(err, ErrNotFound):
		return Relationship{}, err
	}
	incoming, err := repo.FindEdge(ctx, targetID, accountID)
	switch {
	case err == nil:
		out.FollowedBy = incoming.State
	case !errors.Is(err, ErrNotFound):
		return Relationship{}, err
	}
	return out, nil
}

// Page selects a page of a list ordered newest first.
type Page struct {
	// Before is the follow ID of the last edge of the previous page; "" starts at the newest.
	Before string
	// Limit is the page size; zero or less means DefaultPageLimit, and it is capped at MaxPageLimit.
	Limit int
}

// Page sizes of the follow lists.
const (
	DefaultPageLimit = 40
	MaxPageLimit     = 80
)

func (p Page) limit() int {
	switch {
	case p.Limit <= 0:
		return DefaultPageLimit
	case p.Limit > MaxPageLimit:
		return MaxPageLimit
	}
	return p.Limit
}

// Followers returns accountID's active followers, newest follow first.
func (s *Service) Followers(ctx context.Context, accountID string, p Page) ([]graph.Edge, error) {
	return s.d.Repository.ListFollowers(ctx, accountID, graph.StateActive, p.Before, p.limit())
}

// Following returns the accounts accountID actively follows, newest follow first.
func (s *Service) Following(ctx context.Context, accountID string, p Page) ([]graph.Edge, error) {
	return s.d.Repository.ListFollowing(ctx, accountID, graph.StateActive, p.Before, p.limit())
}

// FollowRequests returns the pending follow requests to accountID, newest first.
func (s *Service) FollowRequests(ctx context.Context, accountID string, p Page) ([]graph.Edge, error) {
	return s.d.Repository.ListFollowers(ctx, accountID, graph.StatePending, p.Before, p.limit())
}

// Counters returns the numbers of accountID's active followers and followees.
func (s *Service) Counters(ctx context.Context, accountID string) (graph.Counters, error) {
	c, err := s.d.Repository.FindCounters(ctx, accountID)
	if errors.Is(err, ErrNotFound) {
		return graph.Counters{AccountID: accountID}, nil
	}
	return c, err
}
