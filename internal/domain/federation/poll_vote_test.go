// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/best-friends/my/internal/domain/federation"
)

// A vote is the Create of a Note named after the option, in reply to the Question and addressed
// to the poll's author, as Mastodon sends it.
func TestNewPollVote(t *testing.T) {
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	b, err := federation.NewPollVote("https://my.example/ap/objects/v1", "https://my.example/ap/actors/a1",
		"https://remote.example/users/bob", "https://remote.example/notes/q1", "Yes", at).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["type"] != "Create" || got["id"] != "https://my.example/ap/objects/v1/activity" || got["actor"] != "https://my.example/ap/actors/a1" {
		t.Errorf("Create = %s", b)
	}
	if to, _ := got["to"].([]any); len(to) != 1 || to[0] != "https://remote.example/users/bob" {
		t.Errorf("to = %v", got["to"])
	}
	note, _ := got["object"].(map[string]any)
	if note["type"] != "Note" || note["id"] != "https://my.example/ap/objects/v1" || note["name"] != "Yes" ||
		note["inReplyTo"] != "https://remote.example/notes/q1" || note["attributedTo"] != "https://my.example/ap/actors/a1" {
		t.Errorf("Note = %v", note)
	}
	if to, _ := note["to"].([]any); len(to) != 1 || to[0] != "https://remote.example/users/bob" {
		t.Errorf("Note to = %v", note["to"])
	}
}
