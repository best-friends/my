// SPDX-License-Identifier: AGPL-3.0-only

package jsonld_test

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/jsonld"
	appfederation "github.com/best-friends/my/internal/app/federation"
)

const (
	creator = "https://my.example/ap/actors/01a148a1-365b-79f7-929a-781395cc457f#main-key"
	created = "2026-10-17T06:53:04Z"
)

// vector is what jsonld.js 8.3.3 makes of testdata/create-note.json (testdata/sign.mjs): the
// canonical N-Quads of the signature options and of the document, and the RsaSignature2017
// signature of testdata/key.pem.
type vector struct {
	OptionsNQuads  string `json:"options_nquads"`
	DocumentNQuads string `json:"document_nquads"`
	SignatureValue string `json:"signatureValue"`
}

func load(t *testing.T) (*jsonld.Canonicalizer, []byte, vector, *rsa.PrivateKey) {
	t.Helper()
	c, err := jsonld.New()
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	doc, err := os.ReadFile("testdata/create-note.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/create-note.jsonld-js.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vector
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	p, err := os.ReadFile("testdata/key.pem")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(p)
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c, doc, v, k.(*rsa.PrivateKey)
}

// TestCanonicalizeMatchesJSONLDJS checks the URDNA2015 N-Quads against jsonld.js, the processor
// of Misskey, on a Create of a public post as the fan-out builds it: the Mastodon extension
// context, a language map, a list (focalPoint), numbers, and the three kinds of tag.
func TestCanonicalizeMatchesJSONLDJS(t *testing.T) {
	c, doc, v, _ := load(t)
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		t.Fatal(err)
	}
	got, err := c.Canonicalize(m)
	if err != nil {
		t.Fatalf("Canonicalize = %v", err)
	}
	if got != v.DocumentNQuads {
		t.Errorf("Canonicalize(document) =\n%s\nwant\n%s", got, v.DocumentNQuads)
	}
	got, err = c.Canonicalize(map[string]any{"@context": "https://w3id.org/identity/v1", "creator": creator, "created": created})
	if err != nil {
		t.Fatalf("Canonicalize(options) = %v", err)
	}
	if got != v.OptionsNQuads {
		t.Errorf("Canonicalize(options) =\n%s\nwant\n%s", got, v.OptionsNQuads)
	}
}

// TestSignLinkedDataMatchesJSONLDJS signs the document as the delivery does and gets the
// signature that jsonld.js and Node's RSA make, which Mastodon and Misskey verify the same way.
func TestSignLinkedDataMatchesJSONLDJS(t *testing.T) {
	c, doc, v, key := load(t)
	at, _ := time.Parse(time.RFC3339, created)
	signed, err := appfederation.SignLinkedData(doc, c, appfederation.SigningKey{KeyID: creator, PrivateKey: key}, at)
	if err != nil {
		t.Fatalf("SignLinkedData = %v", err)
	}
	var out struct {
		Signature map[string]string `json:"signature"`
	}
	if err := json.Unmarshal(signed, &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"type": "RsaSignature2017", "creator": creator, "created": created, "signatureValue": v.SignatureValue}
	for k, w := range want {
		if out.Signature[k] != w {
			t.Errorf("signature.%s = %q, want %q", k, out.Signature[k], w)
		}
	}
	if err := appfederation.VerifyLinkedData(signed, c, &key.PublicKey); err != nil {
		t.Errorf("VerifyLinkedData = %v", err)
	}
	// The content is not changed, and HTML is not escaped.
	if !strings.Contains(string(signed), `"content":"<p>Hello <a href=\"https://mastodon.example/@bob\"`) {
		t.Errorf("signed document = %s, want the content as it was", signed)
	}
	// Changing what the signature covers breaks it.
	tampered := strings.Replace(string(signed), "こんにちは", "さようなら", 2)
	if err := appfederation.VerifyLinkedData([]byte(tampered), c, &key.PublicKey); !errors.Is(err, appfederation.ErrLinkedDataSignature) {
		t.Errorf("VerifyLinkedData(tampered) = %v, want ErrLinkedDataSignature", err)
	}
	// Signing again gives the same bytes, and replaces an earlier signature.
	again, err := appfederation.SignLinkedData(signed, c, appfederation.SigningKey{KeyID: creator, PrivateKey: key}, at)
	if err != nil || string(again) != string(signed) {
		t.Errorf("SignLinkedData(signed) = %s, %v; want the same bytes", again, err)
	}
	var _ crypto.Signer = key
}

// TestCanonicalizeRefusesUnbundledContexts: a context that is not bundled is never fetched.
func TestCanonicalizeRefusesUnbundledContexts(t *testing.T) {
	c, _, _, _ := load(t)
	_, err := c.Canonicalize(map[string]any{
		"@context": []any{"https://www.w3.org/ns/activitystreams", "https://evil.example/context.jsonld"},
		"id":       "https://my.example/a", "type": "Note",
	})
	if err == nil {
		t.Fatal("Canonicalize with an unbundled context succeeded")
	}
}

// TestCanonicalizeConcurrently: the canonicalizer is shared by the deliveries.
func TestCanonicalizeConcurrently(t *testing.T) {
	c, doc, v, _ := load(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var m map[string]any
			if err := json.Unmarshal(doc, &m); err != nil {
				t.Error(err)
				return
			}
			got, err := c.Canonicalize(m)
			if err != nil || got != v.DocumentNQuads {
				t.Errorf("Canonicalize = %v, or N-Quads that differ", err)
			}
		})
	}
	wg.Wait()
}
