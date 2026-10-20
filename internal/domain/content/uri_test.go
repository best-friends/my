// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"testing"

	"github.com/best-friends/my/internal/domain/content"
)

func TestStatusURIs(t *testing.T) {
	const id = "0192f3b0-2c3d-7e4f-8a00-112233445566"
	if got, want := content.StatusURI("social.example", id), "https://social.example/ap/objects/"+id; got != want {
		t.Errorf("StatusURI = %q, want %q", got, want)
	}
	if got, want := content.StatusURL("social.example", "alice", id), "https://social.example/@alice/status/"+id; got != want {
		t.Errorf("StatusURL = %q, want %q", got, want)
	}
}

func TestStatusPageURL(t *testing.T) {
	const id = "0192f3b0-2c3d-7e4f-8a00-112233445566"
	local := content.Status{ID: id, IsLocal: true, URL: "https://social.example/@old/" + id}
	if got, want := local.PageURL("social.example", "alice"), "https://social.example/@alice/status/"+id; got != want {
		t.Errorf("local PageURL = %q, want %q (derived, never the stored value)", got, want)
	}
	remote := content.Status{ID: id, URL: "https://remote.example/@bob/1"}
	if got := remote.PageURL("social.example", "alice"); got != remote.URL {
		t.Errorf("remote PageURL = %q, want %q", got, remote.URL)
	}
	if got := (content.Status{ID: id}).PageURL("social.example", "bob"); got != "" {
		t.Errorf("remote PageURL without url = %q, want empty", got)
	}
}
