// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// This file holds the votes of local accounts in polls (FR-112) and what a viewer sees of a
// poll. A vote in a remote poll is recorded in poll_votes and carried to the poll's server by
// the federation module, one Note per choice (federation.md §7.1); the counts of a remote poll
// stay those its server reports (poll_tallies), which include the vote once the server sends
// its next Update.

// EventPollVoted is emitted when a local account votes in a poll: the federation module delivers
// the votes in a remote poll to its author.
const EventPollVoted = "content.poll_voted"

// PollVoted is emitted when the votes of one account in a poll are inserted, one per choice.
type PollVoted struct {
	Votes []content.PollVote
	Poll  content.Poll
	// Status is the status whose current revision has the poll, and AuthorURI the ActivityPub
	// id of its author, who receives the votes in a remote poll.
	Status    StatusRef
	AuthorURI string
}

// EventType implements uow.Event.
func (PollVoted) EventType() string { return EventPollVoted }

type (
	// Poll is content.Poll.
	Poll = content.Poll
	// PollVote is content.PollVote.
	PollVote = content.PollVote
)

// PollView is the poll of a status as a viewer sees it.
type PollView struct {
	Poll Poll
	// Votes are the votes of each choice, by position; a poll without counts yet has 0 for each.
	Votes []int64
	// VotersCount is the number of accounts that voted, nil when it is unknown (a multiple-choice
	// remote poll whose server did not report it).
	VotersCount *int64
	// OwnVotes are the positions the viewer voted for, in order; nil for an anonymous viewer.
	OwnVotes []int
	// Closed is whether the poll has ended (content.Poll.Closed).
	Closed bool
}

// wallClock is the system's clock, for a Reader without one.
type wallClock struct{}

func (wallClock) Now() time.Time { return kernel.Normalize(time.Now()) }

// VotesCount is the sum of the votes of every choice.
func (v PollView) VotesCount() int64 {
	var n int64
	for _, c := range v.Votes {
		n += c
	}
	return n
}

// VotePoll makes accountID vote for choices, the positions of options, in the poll pollID of
// statusID, and returns the votes, one per choice. The poll must be the one of the status's
// current revision: one that an edit replaced fails with content.ErrPollChanged, and a status
// without a poll with content.ErrNoPoll. A poll whose end has passed fails with
// content.ErrPollClosed, choices that the poll does not take with content.ErrInvalidPollChoices,
// and an account that voted in the poll already with content.ErrAlreadyVoted. Like the other
// interactions, it needs a status the account may see and an account that may interact.
func (s *Interactions) VotePoll(ctx context.Context, accountID, statusID, pollID string, choices []int) ([]content.PollVote, error) {
	if err := s.checkRestrictions(ctx, accountID, false); err != nil {
		return nil, err
	}
	st, err := s.visibleStatus(ctx, accountID, statusID)
	if err != nil {
		return nil, err
	}
	rev, err := s.d.Statuses.FindCurrentRevision(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	poll := rev.Poll
	switch {
	case poll == nil:
		return nil, fmt.Errorf("status %s: %w", statusID, content.ErrNoPoll)
	case poll.ID != pollID:
		return nil, fmt.Errorf("poll %s of status %s: %w", pollID, statusID, content.ErrPollChanged)
	}
	author, err := s.d.Accounts.FindByID(ctx, st.AuthorAccountID)
	if err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	if poll.Closed(now) {
		return nil, fmt.Errorf("poll %s: %w", pollID, content.ErrPollClosed)
	}
	positions, err := poll.Choices(choices)
	if err != nil {
		return nil, err
	}
	votes := make([]content.PollVote, len(positions))
	for i, p := range positions {
		id := s.d.IDs.NewID()
		votes[i] = content.PollVote{ID: id, PollID: poll.ID, AccountID: accountID, Position: p, VotedAt: now}
		if !st.IsLocal {
			votes[i].URI = content.PollVoteURI(s.d.Host, id)
		}
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.ensureLive(ctx, tx, st.ID); err != nil {
			return err
		}
		prior, err := s.d.Statuses.ListPollVotesTx(ctx, tx, poll.ID, accountID)
		if err != nil {
			return err
		}
		if len(prior) > 0 {
			return fmt.Errorf("poll %s: %w", poll.ID, content.ErrAlreadyVoted)
		}
		for _, v := range votes {
			ok, err := s.d.Statuses.InsertPollVote(ctx, tx, v)
			if err != nil {
				return err
			}
			if !ok { // a concurrent vote of the account committed first
				return fmt.Errorf("poll %s: %w", poll.ID, content.ErrAlreadyVoted)
			}
		}
		return tx.Emit(ctx, PollVoted{Votes: votes, Poll: *poll, Status: RefOf(st), AuthorURI: author.Account.URI})
	})
	if err != nil {
		return nil, err
	}
	return votes, nil
}

// poll returns the poll of rev as viewerID sees it ("" for an anonymous viewer), nil when rev
// has none.
func (r *Reader) poll(ctx context.Context, rev content.Revision, viewerID string) (*PollView, error) {
	if rev.Poll == nil {
		return nil, nil
	}
	p := *rev.Poll
	out := &PollView{Poll: p, Votes: make([]int64, len(p.Options)), Closed: p.Closed(r.d.Clock.Now())}
	tallies, err := r.d.Statuses.FindPollTallies(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range tallies {
		if t.Position >= 0 && t.Position < len(out.Votes) {
			out.Votes[t.Position] = t.Votes
		}
		out.VotersCount = t.VotersCount
	}
	if out.VotersCount == nil && !p.Multiple {
		// One vote per voter: the votes are the voters.
		n := out.VotesCount()
		out.VotersCount = &n
	}
	if viewerID != "" {
		own, err := r.d.Statuses.ListPollVotes(ctx, p.ID, viewerID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		out.OwnVotes = make([]int, 0, len(own))
		for _, v := range own {
			out.OwnVotes = append(out.OwnVotes, v.Position)
		}
	}
	return out, nil
}
