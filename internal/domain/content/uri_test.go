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
	if got, want := content.StatusURL("social.example", "alice", id), "https://social.example/@alice/"+id; got != want {
		t.Errorf("StatusURL = %q, want %q", got, want)
	}
}
