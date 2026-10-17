// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/domain/federation"
)

func TestReadEnvelope(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want federation.Envelope
	}{
		{
			"object by IRI",
			`{"id":"https://A.example/a/1","type":"Like","actor":"https://a.example/u/1","object":"https://b.example/n/1"}`,
			federation.Envelope{ID: "https://a.example/a/1", Type: "Like", Actor: "https://a.example/u/1", Object: "https://b.example/n/1"},
		},
		{
			"embedded object, actor as an object, several types",
			`{"id":"https://a.example/a/1","type":["Create","Other"],"actor":{"id":"https://a.example/u/1","type":"Person"},"object":[{"id":"https://a.example/n/1","type":"Note"},"https://a.example/n/2"]}`,
			federation.Envelope{ID: "https://a.example/a/1", Type: "Create", Actor: "https://a.example/u/1", Object: "https://a.example/n/1"},
		},
		{
			"no id, no object",
			`{"type":"Delete","actor":"https://a.example/u/1"}`,
			federation.Envelope{Type: "Delete", Actor: "https://a.example/u/1"},
		},
	}
	for _, tt := range tests {
		got, err := federation.ReadEnvelope([]byte(tt.doc), lowerHost)
		if err != nil || got != tt.want {
			t.Errorf("%s: ReadEnvelope = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}

	long := "https://a.example/" + strings.Repeat("x", federation.MaxEnvelopeIRIBytes)
	for name, doc := range map[string]string{
		"not JSON":      `{"type":`,
		"not an object": `["Create"]`,
		"no type":       `{"actor":"https://a.example/u/1"}`,
		"long type":     `{"type":"` + strings.Repeat("x", federation.MaxEnvelopeTypeBytes+1) + `","actor":"https://a.example/u/1"}`,
		"no actor":      `{"type":"Create"}`,
		"http actor":    `{"type":"Create","actor":"http://a.example/u/1"}`,
		"Public actor":  `{"type":"Create","actor":"https://www.w3.org/ns/activitystreams#Public"}`,
		"invalid id":    `{"id":"not an IRI","type":"Create","actor":"https://a.example/u/1"}`,
		"long id":       `{"id":"` + long + `","type":"Create","actor":"https://a.example/u/1"}`,
		"long object":   `{"type":"Like","actor":"https://a.example/u/1","object":"` + long + `"}`,
	} {
		if _, err := federation.ReadEnvelope([]byte(doc), lowerHost); !errors.Is(err, federation.ErrMalformed) {
			t.Errorf("%s: ReadEnvelope = %v, want ErrMalformed", name, err)
		}
	}
}

func TestEnvelopeActivityURI(t *testing.T) {
	body := []byte(`{"type":"Like"}`)
	e := federation.Envelope{ID: "https://a.example/a/1", Actor: "https://a.example/u/1"}
	if got := e.ActivityURI(body); got != e.ID {
		t.Errorf("ActivityURI with an id = %s, want the id", got)
	}
	e.ID = ""
	sum := sha256.Sum256([]byte("https://a.example/u/1\n" + string(body)))
	if got, want := e.ActivityURI(body), federation.IRI("urn:my:digest:"+hex.EncodeToString(sum[:])); got != want {
		t.Errorf("ActivityURI without an id = %s, want %s", got, want)
	}
}

func TestEnvelopeTrustAndSelfRemoval(t *testing.T) {
	actor := federation.IRI("https://a.example/u/1")
	tests := []struct {
		name   string
		id     federation.IRI
		signer federation.IRI
		want   federation.Trust
	}{
		{"signed by the actor", "https://a.example/a/1", actor, federation.TrustSigned},
		{"no id", "", actor, federation.TrustSigned},
		{"relayed", "https://a.example/a/1", "https://relay.example/actor", federation.TrustRefetch},
		{"no usable signer", "https://a.example/a/1", "", federation.TrustRefetch},
		{"id on another host", "https://b.example/a/1", actor, federation.TrustRefetch},
	}
	for _, tt := range tests {
		e := federation.Envelope{ID: tt.id, Type: "Create", Actor: actor}
		if got := e.TrustFor(tt.signer); got != tt.want {
			t.Errorf("%s: TrustFor = %s, want %s", tt.name, got, tt.want)
		}
	}
	for _, tt := range []struct {
		typ    string
		object federation.IRI
		want   bool
	}{
		{"Delete", actor, true}, {"Update", actor, true}, {"Delete", "https://a.example/n/1", false}, {"Follow", actor, false},
	} {
		e := federation.Envelope{Type: tt.typ, Actor: actor, Object: tt.object}
		if got := e.SelfRemoval(); got != tt.want {
			t.Errorf("SelfRemoval(%s %s) = %t, want %t", tt.typ, tt.object, got, tt.want)
		}
	}
}
