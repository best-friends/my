// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	appcontent "github.com/mycroblog/my/internal/app/content"
	"github.com/mycroblog/my/internal/domain/content"
	"github.com/mycroblog/my/internal/interface/clientapi"
)

func TestListStatusReactions(t *testing.T) {
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	reactions := []appcontent.ReactedView{
		{
			Reaction: content.Reaction{ID: reactionID(3), AccountID: aliceAccountID, StatusID: sampleStatusID, Emoji: content.ReactionEmoji{Unicode: "👍"}, CreatedAt: at},
			Emoji:    appcontent.EmojiView{Emoji: content.ReactionEmoji{Unicode: "👍"}},
		},
		{
			// An account that no longer exists is left out.
			Reaction: content.Reaction{ID: reactionID(2), AccountID: "01890a5d-ac96-774b-bcce-b302099a8999", StatusID: sampleStatusID, Emoji: content.ReactionEmoji{Unicode: "👍"}, CreatedAt: at},
			Emoji:    appcontent.EmojiView{Emoji: content.ReactionEmoji{Unicode: "👍"}},
		},
		{
			Reaction: content.Reaction{ID: reactionID(1), AccountID: aliceAccountID, StatusID: sampleStatusID, Emoji: content.ReactionEmoji{CustomEmojiID: sampleCustomEmoji.ID}, CreatedAt: at},
			Emoji:    appcontent.EmojiView{Emoji: content.ReactionEmoji{CustomEmojiID: sampleCustomEmoji.ID}, Custom: &sampleCustomEmoji},
		},
	}
	var gotEmoji, gotBefore string
	var gotLimit int
	f := &fakeStatuses{reactions: func(_ appcontent.Viewer, id, emoji, before string, limit int) (appcontent.ReactedPage, error) {
		if id != sampleStatusID {
			return appcontent.ReactedPage{}, errNotFound
		}
		gotEmoji, gotBefore, gotLimit = emoji, before, limit
		return appcontent.ReactedPage{Reactions: reactions[:min(limit, len(reactions))]}, nil
	}}
	e := newStatusEnv(t, f)

	res := e.send(t, http.MethodGet, "/statuses/"+sampleStatusID+"/reactions?limit=2", "", "")
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("GET = %d, Cache-Control %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	var page clientapi.StatusReactionPage
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if gotLimit != 3 || gotEmoji != "" || gotBefore != "" {
		t.Errorf("Reactions(emoji %q, before %q, limit %d), want one more than the limit", gotEmoji, gotBefore, gotLimit)
	}
	if len(page.Items) != 1 || page.Items[0].Account.Username != "alice" || page.Items[0].Emoji.Name != "👍" ||
		!page.Items[0].CreatedAt.Equal(at) || page.NextCursor == nil {
		t.Fatalf("page = %+v, want alice's reaction and a next page", page)
	}

	res = e.send(t, http.MethodGet, "/statuses/"+sampleStatusID+"/reactions?emoji="+url.QueryEscape("blobcat@misskey.example")+"&cursor="+*page.NextCursor, "read", "")
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("GET with an emoji = %d, %v", res.StatusCode, err)
	}
	if gotEmoji != "blobcat@misskey.example" || gotBefore != reactionID(2) {
		t.Errorf("Reactions(emoji %q, before %q), want the emoji and the cursor", gotEmoji, gotBefore)
	}
	if c := page.Items[len(page.Items)-1].Emoji; c.Name != "blobcat@misskey.example" || *c.Shortcode != "blobcat" || *c.Domain != "misskey.example" || c.Emoji == nil || c.Emoji.StaticUrl != sampleCustomEmoji.Image.StaticURL {
		t.Errorf("custom emoji = %+v", c)
	}

	res = e.send(t, http.MethodGet, "/statuses/01890a5d-ac96-774b-bcce-b302099a9999/reactions", "", "")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("reactions of a status that cannot be seen = %d, want 404", res.StatusCode)
	}
	if res := e.send(t, http.MethodGet, "/statuses/"+sampleStatusID+"/reactions?limit=0", "", ""); res.StatusCode != http.StatusBadRequest {
		t.Errorf("limit 0 = %d, want 400", res.StatusCode)
	}
}

// Reacting sets the viewer's reaction with the emoji of the body and answers with the status;
// an emoji that a reaction cannot have is a 422.
func TestReactToStatus(t *testing.T) {
	var ops []string
	f := &fakeStatuses{
		interact: func(op, accountID, id string) error {
			if accountID != aliceAccountID || id != sampleStatusID {
				return fmt.Errorf("unexpected %s by %s of %s", op, accountID, id)
			}
			if op == "react:blobcat@misskey.example" || op == "react:" {
				return fmt.Errorf("fake: %w", content.ErrInvalidReaction)
			}
			ops = append(ops, op)
			return nil
		},
		view: func(v appcontent.Viewer, _ string) (appcontent.StatusView, error) { return sampleView(v), nil },
	}
	e := newStatusEnv(t, f)
	path := "/statuses/" + sampleStatusID + "/reaction"
	if res := e.send(t, http.MethodPut, path, "write", `{"emoji":"👍"}`); res.StatusCode != http.StatusOK || decodeStatus(t, res).Id != sampleStatusID {
		t.Errorf("PUT = %d", res.StatusCode)
	}
	if res := e.send(t, http.MethodDelete, path, "write", ""); res.StatusCode != http.StatusOK {
		t.Errorf("DELETE = %d", res.StatusCode)
	}
	if want := "react:👍 unreact"; strings.Join(ops, " ") != want {
		t.Errorf("operations = %v, want %s", ops, want)
	}
	if res := e.send(t, http.MethodPut, path, "write", `{"emoji":"blobcat@misskey.example"}`); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("PUT with a remote custom emoji = %d, want 422", res.StatusCode)
	}
	if res := e.send(t, http.MethodPut, path, "write", `{"emoji":""}`); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("PUT without an emoji = %d, want 422", res.StatusCode)
	}
	if res := e.send(t, http.MethodPut, path, "", `{"emoji":"👍"}`); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("PUT anonymously = %d, want 401", res.StatusCode)
	}
}

// reactionID returns the canonical ID of the n-th reaction of the tests.
func reactionID(n int) string { return fmt.Sprintf("01890a5d-ac96-774b-bcce-b302099a%04d", 9400+n) }
