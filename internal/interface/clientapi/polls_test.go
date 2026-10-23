// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/domain/content"
)

const (
	samplePollID = "01890a5d-ac96-774b-bcce-b302099a9300"
	otherPollID  = "01890a5d-ac96-774b-bcce-b302099a9301"
)

// pollView is sampleView with a single-choice poll that ends at closesAt; a local viewer voted
// for its second option.
func pollView(viewer appcontent.Viewer, closesAt time.Time, closed bool) appcontent.StatusView {
	v := sampleView(viewer)
	voters := int64(5)
	v.Poll = &appcontent.PollView{
		Poll: content.Poll{ID: samplePollID, ClosesAt: &closesAt, Options: []content.PollOption{
			{Position: 0, Title: "Yes"}, {Position: 1, Title: "No"},
		}},
		Votes: []int64{3, 2}, VotersCount: &voters, Closed: closed,
	}
	if viewer.Kind == appcontent.ViewerLocal {
		v.Poll.OwnVotes = []int{1}
	}
	return v
}

// A status presents its poll with the counts of each option, its end, and, for an
// authenticated user, the user's vote.
func TestStatusPoll(t *testing.T) {
	closesAt := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	f := &fakeStatuses{view: func(v appcontent.Viewer, _ string) (appcontent.StatusView, error) {
		return pollView(v, closesAt, true), nil
	}}
	e := newStatusEnv(t, f)

	s := decodeStatus(t, e.send(t, http.MethodGet, "/statuses/"+sampleStatusID, "", ""))
	p := s.Poll
	if p == nil || p.Id != samplePollID || p.Multiple || !p.Expired || p.ExpiresAt == nil || !p.ExpiresAt.Equal(closesAt) ||
		p.VotesCount != 5 || p.VotersCount == nil || *p.VotersCount != 5 || len(p.Options) != 2 ||
		p.Options[0].Title != "Yes" || p.Options[0].VotesCount == nil || *p.Options[0].VotesCount != 3 ||
		p.Options[1].VotesCount == nil || *p.Options[1].VotesCount != 2 {
		t.Fatalf("anonymous poll = %+v", p)
	}
	if p.Voted != nil || p.OwnVotes != nil {
		t.Errorf("an anonymous poll has the viewer's vote: %+v", p)
	}

	s = decodeStatus(t, e.send(t, http.MethodGet, "/statuses/"+sampleStatusID, "read", ""))
	if p := s.Poll; p == nil || p.Voted == nil || !*p.Voted || p.OwnVotes == nil || !slices.Equal(*p.OwnVotes, []int{1}) {
		t.Errorf("authenticated poll = %+v", s.Poll)
	}

	// A poll that hides its counts until it ends leaves out those of its options.
	f.view = func(v appcontent.Viewer, _ string) (appcontent.StatusView, error) {
		view := pollView(v, closesAt, false)
		view.Poll.Poll.HideTotals, view.Poll.TotalsHidden = true, true
		return view, nil
	}
	s = decodeStatus(t, e.send(t, http.MethodGet, "/statuses/"+sampleStatusID, "", ""))
	if p := s.Poll; p == nil || p.VotersCount == nil || *p.VotersCount != 5 || p.Options[0].VotesCount != nil || p.Options[1].VotesCount != nil {
		t.Errorf("poll that hides its counts = %+v", s.Poll)
	}

	// A status without a poll has none.
	f.view = func(v appcontent.Viewer, _ string) (appcontent.StatusView, error) { return sampleView(v), nil }
	if s := decodeStatus(t, e.send(t, http.MethodGet, "/statuses/"+sampleStatusID, "", "")); s.Poll != nil {
		t.Errorf("poll of a status without one = %+v", s.Poll)
	}
}

// Voting sends the poll and the choices of the body and answers with the status; the failures
// of the vote are 422s, and a poll that an edit replaced a 409.
func TestVoteInPoll(t *testing.T) {
	var ops []string
	fail := map[string]error{
		"vote:" + samplePollID + ":[0 1]": content.ErrInvalidPollChoices,
		"vote:" + samplePollID + ":[1]":   content.ErrAlreadyVoted,
		"vote:" + samplePollID + ":[2]":   content.ErrPollClosed,
		"vote:" + samplePollID + ":[3]":   content.ErrNoPoll,
		"vote:" + otherPollID + ":[0]":    content.ErrPollChanged,
	}
	f := &fakeStatuses{
		interact: func(op, accountID, id string) error {
			if accountID != aliceAccountID || id != sampleStatusID {
				return fmt.Errorf("unexpected %s by %s of %s", op, accountID, id)
			}
			if err := fail[op]; err != nil {
				return fmt.Errorf("fake: %w", err)
			}
			ops = append(ops, op)
			return nil
		},
		view: func(v appcontent.Viewer, _ string) (appcontent.StatusView, error) {
			return pollView(v, time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC), false), nil
		},
	}
	e := newStatusEnv(t, f)
	path := "/statuses/" + sampleStatusID + "/poll/votes"
	body := func(pollID string, choices string) string {
		return `{"poll_id":"` + pollID + `","choices":` + choices + `}`
	}
	res := e.send(t, http.MethodPost, path, "write", body(samplePollID, "[0]"))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST = %d", res.StatusCode)
	}
	if s := decodeStatus(t, res); s.Poll == nil || s.Poll.Expired || s.Poll.Voted == nil || !*s.Poll.Voted {
		t.Errorf("status after the vote = %+v", s.Poll)
	}
	if want := "vote:" + samplePollID + ":[0]"; strings.Join(ops, " ") != want {
		t.Errorf("operations = %v, want %s", ops, want)
	}
	for _, c := range []struct {
		pollID, choices string
		want            int
	}{
		{samplePollID, "[0,1]", http.StatusUnprocessableEntity},
		{samplePollID, "[1]", http.StatusUnprocessableEntity},
		{samplePollID, "[2]", http.StatusUnprocessableEntity},
		{samplePollID, "[3]", http.StatusUnprocessableEntity},
		{otherPollID, "[0]", http.StatusConflict},
	} {
		if res := e.send(t, http.MethodPost, path, "write", body(c.pollID, c.choices)); res.StatusCode != c.want {
			t.Errorf("POST %s %s = %d, want %d", c.pollID, c.choices, res.StatusCode, c.want)
		}
	}
	if res := e.send(t, http.MethodPost, path, "", body(samplePollID, "[0]")); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST anonymously = %d, want 401", res.StatusCode)
	}
}
