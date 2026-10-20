// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

const (
	// JobPollUpdate records the Update of a local status with a poll, which carries the poll's
	// counts as they are when the job runs (FR-112, federation.md §7.1). Its payload is
	// pollUpdatePayload.
	JobPollUpdate = "federation.poll_update"
	// PollUpdateDelay is how long after a vote in a local poll its Update is sent: the votes of
	// that time share one Update, as Mastodon gathers them.
	PollUpdateDelay = 3 * time.Minute
)

// pollUpdatePayload is the payload of JobPollUpdate.
type pollUpdatePayload struct {
	StatusID string `json:"status_id"`
	PollID   string `json:"poll_id"`
}

// registerPollVoteProjectors records the votes of a local account in a remote poll (FR-112,
// federation.md §7.1): one Create of a Note per choice, named after the option and in reply to
// the Question, to the poll's author alone, as Mastodon sends them. The activity is about the
// poll's status (ObjectID), whose author the fan-out delivers to (phaseStatusAuthor), and its
// document is built here from the event and stored with it.
//
// A vote in a local poll, of a local or a remote account, enqueues JobPollUpdate after
// PollUpdateDelay, and the end of a local poll enqueues it at once: the job's idempotency key is
// the poll's, so the votes that come while it waits share its Update.
func registerPollVoteProjectors(r *uow.Projectors, rec recorder) {
	r.Register(appcontent.EventPollVoted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.PollVoted)
		if !ok || len(ev.Votes) == 0 {
			return nil
		}
		if ev.Status.IsLocal {
			return enqueuePollUpdate(ctx, tx, rec, ev.Status.ID, ev.Poll.ID, ev.Votes[0].VotedAt.Add(PollUpdateDelay))
		}
		if ev.Status.URI == "" || ev.AuthorURI == "" {
			return nil
		}
		actor := federation.IRI(appaccount.ActorURI(rec.d.Host, ev.Votes[0].AccountID))
		for _, v := range ev.Votes {
			option, ok := ev.Poll.Option(v.Position)
			if !ok || v.URI == "" {
				return fmt.Errorf("%w: vote %s for choice %d of poll %s", federation.ErrInvalidOutbound, v.ID, v.Position, ev.Poll.ID)
			}
			doc := federation.NewPollVote(federation.IRI(v.URI), actor, federation.IRI(ev.AuthorURI), federation.IRI(ev.Status.URI), option.Title, v.VotedAt)
			act, err := rec.insert(ctx, tx, federation.OutboundActivity{
				ActivityURI: doc.ID, Type: federation.ActivityCreate, ActorAccountID: v.AccountID,
				ObjectKind: federation.OutboundPollVote, ObjectID: ev.Status.ID, CreatedAt: v.VotedAt,
			})
			if err != nil {
				return err
			}
			b, err := doc.Marshal()
			if err != nil {
				return err
			}
			if _, err := rec.d.Repository.InsertOutboundDocument(ctx, tx, act.ID, b, act.CreatedAt); err != nil {
				return err
			}
			if err := rec.enqueue(ctx, tx, act, fanOutPayload{}); err != nil {
				return err
			}
		}
		return nil
	})
	r.Register(appcontent.EventPollClosed, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.PollClosed)
		if !ok || !ev.Status.IsLocal {
			return nil
		}
		return enqueuePollUpdate(ctx, tx, rec, ev.Status.ID, ev.Poll.ID, time.Time{})
	})
}

func enqueuePollUpdate(ctx context.Context, tx uow.Tx, rec recorder, statusID, pollID string, runAt time.Time) error {
	b, err := json.Marshal(pollUpdatePayload{StatusID: statusID, PollID: pollID})
	if err != nil {
		return err
	}
	return rec.d.Outbox.EnqueueTx(ctx, tx, queue.NewJob{
		Type: JobPollUpdate, Payload: b, IdempotencyKey: "poll_update:" + pollID, RunAt: runAt,
	})
}

// PollStatuses finds a local status with its current revision; *appcontent.Service implements
// it.
type PollStatuses interface {
	Find(ctx context.Context, statusID string) (appcontent.Post, error)
}

// PollUpdatesDeps are the ports of PollUpdates.
type PollUpdatesDeps struct {
	UnitOfWork uow.UnitOfWork
	Statuses   PollStatuses
	// Recorder records the Update and enqueues its fan-out.
	Recorder RecorderDeps
	Clock    kernel.Clock
}

// PollUpdates runs JobPollUpdate.
type PollUpdates struct {
	d   PollUpdatesDeps
	rec recorder
}

// NewPollUpdates returns the PollUpdates on d.
func NewPollUpdates(d PollUpdatesDeps) *PollUpdates {
	return &PollUpdates{d: d, rec: recorder{d: d.Recorder}}
}

// RegisterJobs registers the JobPollUpdate handler.
func (u *PollUpdates) RegisterJobs(reg *queue.JobHandlerRegistry) {
	reg.Register(JobPollUpdate, func(ctx context.Context, j queue.Job) queue.Result {
		var p pollUpdatePayload
		if err := json.Unmarshal(j.Payload, &p); err != nil || p.StatusID == "" || p.PollID == "" {
			return queue.Fail(fmt.Errorf("federation: poll update payload %s: %w", j.Payload, err))
		}
		if err := u.Run(ctx, p.StatusID, p.PollID); err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
}

// Run records the Update of the local status statusID while pollID is the poll of its current
// revision; the fan-out builds its Question with the counts of that time, and sends it to the
// inboxes of the Create, the followers of a public or unlisted status, and the voters
// (FanOut). A status that is deleted, or whose poll is not pollID, sends nothing.
func (u *PollUpdates) Run(ctx context.Context, statusID, pollID string) error {
	post, err := u.d.Statuses.Find(ctx, statusID)
	if errors.Is(err, appcontent.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !post.Status.IsLocal || post.Revision.Poll == nil || post.Revision.Poll.ID != pollID {
		return nil
	}
	now := u.d.Clock.Now()
	return u.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		return u.rec.record(ctx, tx, federation.OutboundActivity{
			Type: federation.ActivityUpdate, ActorAccountID: post.Status.AuthorAccountID,
			ObjectKind: federation.OutboundStatus, ObjectID: post.Status.ID, CreatedAt: now,
		}, post.Revision.ID)
	})
}
