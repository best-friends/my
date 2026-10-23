// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"testing"
	"time"

	"github.com/mycroblog/my/internal/domain/content"
)

func TestCountsAsReply(t *testing.T) {
	for v, want := range map[content.Visibility]bool{
		content.VisibilityPublic: true, content.VisibilityLocal: true, content.VisibilityUnlisted: true,
		content.VisibilityPrivate: false, content.VisibilityDirect: false,
	} {
		if got := v.CountsAsReply(); got != want {
			t.Errorf("%s.CountsAsReply() = %v, want %v", v, got, want)
		}
	}
}

func TestRevisionSupersedes(t *testing.T) {
	if !(content.Revision{ID: testID(1), Seq: 1}).Supersedes(content.Head{}) {
		t.Error("a first revision does not supersede an empty head")
	}
	head := content.Head{RevisionID: testID(2), Seq: 2}
	if (content.Revision{ID: testID(1), Seq: 1}).Supersedes(head) || (content.Revision{ID: testID(2), Seq: 2}).Supersedes(head) {
		t.Error("an older or the same revision supersedes the head")
	}
	if !(content.Revision{ID: testID(3), Seq: 3}).Supersedes(head) {
		t.Error("a newer revision does not supersede the head")
	}
}

func TestHeadOf(t *testing.T) {
	at := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	h := content.HeadOf(content.Revision{ID: testID(2), StatusID: testID(1), Seq: 2, RevisedAt: at})
	if h != (content.Head{StatusID: testID(1), RevisionID: testID(2), Seq: 2, LastRevisedAt: at}) {
		t.Errorf("HeadOf = %+v", h)
	}
	if !h.Edited() || (content.Head{Seq: 1}).Edited() {
		t.Error("Edited does not follow seq > 1")
	}
}
