// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// This file holds the votes of accounts in polls (FR-112, data-model §4.4). A vote with several
// choices is one PollVote per choice, and votes are never taken back.

var (
	// ErrNoPoll reports a vote on a status whose current revision has no poll.
	ErrNoPoll = errors.New("content: the status has no poll")
	// ErrPollClosed reports a vote in a poll whose end has passed.
	ErrPollClosed = errors.New("content: the poll is closed")
	// ErrAlreadyVoted reports a vote of an account that voted in the poll already.
	ErrAlreadyVoted = errors.New("content: the account voted in the poll already")
	// ErrInvalidPollChoices reports choices that the poll does not take: none, an unknown
	// position, the same position twice, or several in a single-choice poll.
	ErrInvalidPollChoices = errors.New("content: invalid poll choices")
	// ErrPollChanged reports a vote in a poll that is no longer the status's: an edit replaced it.
	ErrPollChanged = errors.New("content: the poll of the status changed")
)

// PollVote is one choice an account voted for in a poll.
type PollVote struct {
	ID        string
	PollID    string
	AccountID string
	Position  int
	// URI is the id of the Note that carries the choice to the poll's server, for a local
	// account's vote in a remote poll (PollVoteURI), or that a remote voter sent; "" otherwise.
	URI     string
	VotedAt time.Time
}

// PollVoteURI returns the id of the Note that carries a local account's vote with the ID id to a
// remote poll's server (ADR-0018: an object of this instance, /ap/objects/{uuid}). Its Create is
// at PollVoteURI + "/activity".
func PollVoteURI(host, id string) string {
	return "https://" + host + "/ap/objects/" + id
}

// Closed reports whether the end of p is at or before now; a poll without an end never closes.
func (p Poll) Closed(now time.Time) bool {
	return p.ClosesAt != nil && !p.ClosesAt.After(now)
}

// Choices checks choices, the positions an account votes for, against p and returns them in
// order. It fails with ErrInvalidPollChoices on none, a position that p does not have, a
// position given twice, or more than one in a single-choice poll.
func (p Poll) Choices(choices []int) ([]int, error) {
	if len(choices) == 0 {
		return nil, fmt.Errorf("%w: no choice", ErrInvalidPollChoices)
	}
	if !p.Multiple && len(choices) > 1 {
		return nil, fmt.Errorf("%w: %d choices in a single-choice poll", ErrInvalidPollChoices, len(choices))
	}
	out := slices.Clone(choices)
	slices.Sort(out)
	for i, c := range out {
		if c < 0 || c >= len(p.Options) {
			return nil, fmt.Errorf("%w: no choice %d", ErrInvalidPollChoices, c)
		}
		if i > 0 && out[i-1] == c {
			return nil, fmt.Errorf("%w: choice %d twice", ErrInvalidPollChoices, c)
		}
	}
	return out, nil
}

// Option returns the choice of p at position, false when p has none there.
func (p Poll) Option(position int) (PollOption, bool) {
	for _, o := range p.Options {
		if o.Position == position {
			return o, true
		}
	}
	return PollOption{}, false
}
