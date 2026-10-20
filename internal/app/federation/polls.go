// SPDX-License-Identifier: AGPL-3.0-only

package federation

import (
	"context"
	"fmt"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/federation"
)

// registerPollVoteProjectors records the votes of a local account in a remote poll (FR-112,
// federation.md §7.1): one Create of a Note per choice, named after the option and in reply to
// the Question, to the poll's author alone, as Mastodon sends them. The activity is about the
// poll's status (ObjectID), whose author the fan-out delivers to (phaseStatusAuthor), and its
// document is built here from the event and stored with it. A vote in a local poll sends
// nothing.
func registerPollVoteProjectors(r *uow.Projectors, rec recorder) {
	r.Register(appcontent.EventPollVoted, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
		ev, ok := e.(appcontent.PollVoted)
		if !ok || ev.Status.IsLocal || ev.Status.URI == "" || ev.AuthorURI == "" || len(ev.Votes) == 0 {
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
}
