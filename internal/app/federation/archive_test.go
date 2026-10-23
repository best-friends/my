// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	appcontent "github.com/mycroblog/my/internal/app/content"
)

// TestArchiveOutbox checks the outbox of a data export: every status of the author, of every
// visibility, as a Create of its Note, in one OrderedCollection.
func TestArchiveOutbox(t *testing.T) {
	w := newObjWorld()
	var want []string
	for i, v := range []appcontent.Visibility{
		appcontent.VisibilityPublic, appcontent.VisibilityLocal, appcontent.VisibilityUnlisted,
		appcontent.VisibilityPrivate, appcontent.VisibilityDirect,
	} {
		st := w.post("0192f3b0-0000-7000-8000-00000000040"+string(rune('0'+i)), v)
		want = append(want, st.URI+"/activity")
	}
	o := newObjects(w, nil)
	var buf bytes.Buffer
	if err := o.ArchiveOutbox(t.Context(), authorID, &buf); err != nil {
		t.Fatalf("ArchiveOutbox = %v", err)
	}
	var doc struct {
		Context      []any  `json:"@context"`
		ID           string `json:"id"`
		Type         string `json:"type"`
		TotalItems   int    `json:"totalItems"`
		OrderedItems []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Object struct {
				Type    string   `json:"type"`
				Content string   `json:"content"`
				To      []string `json:"to"`
			} `json:"object"`
		} `json:"orderedItems"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("outbox is not JSON: %v\n%s", err, buf.String())
	}
	actor := w.actors[authorID].Account.URI
	if doc.Context == nil || doc.ID != actor+"/outbox" || doc.Type != "OrderedCollection" || doc.TotalItems != len(want) {
		t.Errorf("outbox = %+v, want an OrderedCollection of %d items", doc, len(want))
	}
	var got []string
	for _, it := range doc.OrderedItems {
		if it.Type != "Create" || it.Object.Type != "Note" || len(it.Object.To) == 0 {
			t.Errorf("item = %+v, want the Create of an addressed Note", it)
		}
		got = append(got, it.ID)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}

	buf.Reset()
	if err := o.ArchiveActor(t.Context(), authorID, &buf); err != nil {
		t.Fatalf("ArchiveActor = %v", err)
	}
	var a map[string]any
	if err := json.Unmarshal(buf.Bytes(), &a); err != nil || a["id"] != actor || a["outbox"] != actor+"/outbox" {
		t.Errorf("actor = %v, %v", a, err)
	}
}
