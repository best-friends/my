// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/mycroblog/my/internal/domain/federation"
)

// The test vectors of FEP-8b32 (fep-8b32.feature) and FEP-521a (fep-521a.feature), from
// https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32 and .../521a.
const (
	fep8b32Document = `{
		"@context": ["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v2"],
		"id": "https://server.example/activities/1",
		"type": "Create",
		"actor": "https://server.example/users/alice",
		"object": {
			"id": "https://server.example/objects/1",
			"type": "Note",
			"attributedTo": "https://server.example/users/alice",
			"content": "Hello world",
			"location": {"type": "Place", "longitude": -71.184902, "latitude": 25.273962}
		}
	}`
	fep8b32Canonical  = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v2"],"actor":"https://server.example/users/alice","id":"https://server.example/activities/1","object":{"attributedTo":"https://server.example/users/alice","content":"Hello world","id":"https://server.example/objects/1","location":{"latitude":25.273962,"longitude":-71.184902,"type":"Place"},"type":"Note"},"type":"Create"}`
	fep8b32SecretKey  = "z3u2en7t5LR2WtQH5PfFqMqwVHBeXouLzo6haApm8XHqvjxq" //nolint:gosec // the published test key of FEP-8b32
	fep8b32PublicKey  = "z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2"
	fep8b32KeyID      = "https://server.example/users/alice#ed25519-key"
	fep8b32ProofValue = "z42ffGu6AUKPCFcFPiabmUvnGLPJzC7e4DGWC52NUasSSH37UMa9c58tdgVszUcZfytxa4fQ5TYHaJENCxUDe9SdL"
	fep8b32Signed     = `{
		"@context": ["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v2"],
		"id": "https://server.example/activities/1",
		"type": "Create",
		"actor": "https://server.example/users/alice",
		"object": {
			"id": "https://server.example/objects/1",
			"type": "Note",
			"attributedTo": "https://server.example/users/alice",
			"content": "Hello world",
			"location": {"type": "Place", "longitude": -71.184902, "latitude": 25.273962}
		},
		"proof": {
			"@context": ["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v2"],
			"type": "DataIntegrityProof",
			"cryptosuite": "eddsa-jcs-2022",
			"verificationMethod": "https://server.example/users/alice#ed25519-key",
			"proofPurpose": "assertionMethod",
			"proofValue": "z42ffGu6AUKPCFcFPiabmUvnGLPJzC7e4DGWC52NUasSSH37UMa9c58tdgVszUcZfytxa4fQ5TYHaJENCxUDe9SdL",
			"created": "2023-02-24T23:36:38Z"
		}
	}`
)

var fep8b32Created = time.Date(2023, 2, 24, 23, 36, 38, 0, time.UTC)

// fep8b32Key decodes the vector's secret key: base58btc of the ed25519-priv multicodec prefix
// (0x8026) and the 32-byte seed.
func fep8b32Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	b := base58Decode(t, strings.TrimPrefix(fep8b32SecretKey, "z"))
	if len(b) != 34 || b[0] != 0x80 || b[1] != 0x26 {
		t.Fatalf("secret key %x is not an ed25519-priv multikey", b)
	}
	return ed25519.NewKeyFromSeed(b[2:])
}

func base58Decode(t *testing.T, s string) []byte {
	t.Helper()
	// An Ed25519 multikey reuses the package's decoder through ParseEd25519Multikey; a secret
	// key has another prefix, so decode it here. The value has no leading zero bytes.
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	n := new(big.Int)
	for _, c := range []byte(s) {
		i := strings.IndexByte(alphabet, c)
		if i < 0 {
			t.Fatalf("invalid base58 %q", c)
		}
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(i)))
	}
	return n.Bytes()
}

func TestCanonicalJSONFEP8b32(t *testing.T) {
	got, err := federation.CanonicalJSON([]byte(fep8b32Document))
	if err != nil || string(got) != fep8b32Canonical {
		t.Errorf("CanonicalJSON = %s, %v\nwant %s", got, err, fep8b32Canonical)
	}
}

func TestCanonicalJSON(t *testing.T) {
	cases := map[string]string{
		// RFC 8785 §3.2.2 (whitespace, escapes, literals, and numbers).
		`{"numbers": [333333333.33333329, 1E30, 4.50, 2e-3, 0.000000000000000000000000001], "string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/", "literals": [null, true, false]}`: `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`,
		// RFC 8785 §3.2.3 (sorting by UTF-16 code units).
		`{"\u20ac": "Euro Sign", "\r": "Carriage Return", "\ufb33": "Hebrew Letter Dalet With Dagesh", "1": "One", "\ud83d\ude00": "Emoji: Grinning Face", "\u0080": "Control", "\u00f6": "Latin Small Letter O With Diaeresis"}`: "{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\",\"ö\":\"Latin Small Letter O With Diaeresis\",\"€\":\"Euro Sign\",\"😀\":\"Emoji: Grinning Face\",\"\ufb33\":\"Hebrew Letter Dalet With Dagesh\"}",
		`"<a href=\"x\">&amp;</a>\u2028"`: "\"<a href=\\\"x\\\">&amp;</a>\u2028\"",
		// The values as Node.js serializes them.
		`[-0, 1, -1.5, 1e21, 1e20, 1e-6, -1e-7, 123456789012345678901234, 5e-324, 1.7976931348623157e308, 9007199254740993, 295147905179352830000, 0.30000000000000004]`: `[0,1,-1.5,1e+21,100000000000000000000,0.000001,-1e-7,1.2345678901234569e+23,5e-324,1.7976931348623157e+308,9007199254740992,295147905179352830000,0.30000000000000004]`,
		`{}`: `{}`,
	}
	for in, want := range cases {
		got, err := federation.CanonicalJSON([]byte(in))
		if err != nil || string(got) != want {
			t.Errorf("CanonicalJSON(%s) = %+q, %v; want %+q", in, got, err, want)
		}
	}
	for _, bad := range []string{`{"a":1,"a":2}`, `[1e400]`, `{"a":1} {}`, `[`, strings.Repeat("[", 100) + strings.Repeat("]", 100)} {
		if _, err := federation.CanonicalJSON([]byte(bad)); !errors.Is(err, federation.ErrNotCanonicalizable) {
			t.Errorf("CanonicalJSON(%.40s) = %v, want ErrNotCanonicalizable", bad, err)
		}
	}
}

func TestSignEdDSAJCS2022MatchesFEP8b32(t *testing.T) {
	key := fep8b32Key(t)
	if got := federation.EncodeEd25519Multikey(key.Public().(ed25519.PublicKey)); got != fep8b32PublicKey {
		t.Fatalf("public key %s, want %s", got, fep8b32PublicKey)
	}
	signed, err := federation.SignEdDSAJCS2022([]byte(fep8b32Document), key, fep8b32KeyID, fep8b32Created)
	if err != nil {
		t.Fatal(err)
	}
	gotCanonical, err := federation.CanonicalJSON(signed)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical, err := federation.CanonicalJSON([]byte(fep8b32Signed))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotCanonical, wantCanonical) {
		t.Errorf("signed document\n%s\nwant\n%s", gotCanonical, wantCanonical)
	}
}

func TestReadProofVerifiesFEP8b32(t *testing.T) {
	pub, err := federation.ParseEd25519Multikey(fep8b32PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	p, err := federation.ReadProof([]byte(fep8b32Signed), now)
	if err != nil {
		t.Fatal(err)
	}
	if p.VerificationMethod != fep8b32KeyID || p.Created != "2023-02-24T23:36:38Z" {
		t.Errorf("proof = %+v", p)
	}
	if err := p.Verify([]byte(fep8b32Signed), pub); err != nil {
		t.Errorf("Verify = %v", err)
	}

	// A linked data signature added after the proof does not change it (FEP-8b32).
	var doc map[string]any
	if err := json.Unmarshal([]byte(fep8b32Signed), &doc); err != nil {
		t.Fatal(err)
	}
	doc["signature"] = map[string]any{"type": "RsaSignature2017", "signatureValue": "x"}
	withLD, _ := json.Marshal(doc)
	if err := p.Verify(withLD, pub); err != nil {
		t.Errorf("Verify with a signature = %v", err)
	}

	// Any change to the document, or another key, fails.
	tampered := strings.Replace(fep8b32Signed, "Hello world", "Hello world!", 1)
	if err := p.Verify([]byte(tampered), pub); !errors.Is(err, federation.ErrProofMismatch) {
		t.Errorf("Verify of a changed document = %v", err)
	}
	other, _, _ := ed25519.GenerateKey(nil)
	if err := p.Verify([]byte(fep8b32Signed), other); !errors.Is(err, federation.ErrProofMismatch) {
		t.Errorf("Verify with another key = %v", err)
	}
	// The document's @context must start with the proof's.
	reordered := strings.Replace(fep8b32Signed,
		`"@context": ["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v2"],
		"id"`, `"@context": ["https://w3id.org/security/data-integrity/v2", "https://www.w3.org/ns/activitystreams"],
		"id"`, 1)
	if reordered == fep8b32Signed {
		t.Fatal("the test did not change the context")
	}
	if err := p.Verify([]byte(reordered), pub); !errors.Is(err, federation.ErrProofMismatch) {
		t.Errorf("Verify with another @context = %v", err)
	}
}

func TestSignedProofRoundTrips(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	// A document whose context lacks the data integrity context gets it, and HTML stays as is.
	doc := `{"@context":["https://www.w3.org/ns/activitystreams",{"toot":"http://joinmastodon.org/ns#"}],"id":"https://social.example/a/1","type":"Like","actor":"https://social.example/u/a","object":"https://remote.example/n/1","content":"<p>&amp;</p>"}`
	signed, err := federation.SignEdDSAJCS2022([]byte(doc), key, "https://social.example/u/a#ed25519-key", fep8b32Created)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(signed, []byte(`"<p>&amp;</p>"`)) || !bytes.Contains(signed, []byte(federation.DataIntegrityContext)) {
		t.Errorf("signed = %s", signed)
	}
	p, err := federation.ReadProof(signed, fep8b32Created)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Verify(signed, pub); err != nil {
		t.Errorf("Verify = %v", err)
	}
	// Signing again replaces the proof and gives the same bytes (Ed25519 is deterministic).
	again, err := federation.SignEdDSAJCS2022(signed, key, "https://social.example/u/a#ed25519-key", fep8b32Created)
	if err != nil || !bytes.Equal(again, signed) {
		t.Errorf("signing again = %s, %v", again, err)
	}
}

func TestReadProofIgnoresUnusableProofs(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	replace := func(from, to string) string { return strings.Replace(fep8b32Signed, from, to, 1) }
	for name, doc := range map[string]string{
		"no proof":       fep8b32Document,
		"another type":   replace(`"type": "DataIntegrityProof"`, `"type": "Ed25519Signature2020"`),
		"another suite":  replace(`"eddsa-jcs-2022"`, `"eddsa-rdfc-2022"`),
		"another use":    replace(`"proofPurpose": "assertionMethod"`, `"proofPurpose": "authentication"`),
		"no key":         replace(`"verificationMethod": "https://server.example/users/alice#ed25519-key",`, ``),
		"expired":        replace(`"created": "2023-02-24T23:36:38Z"`, `"created": "2023-02-24T23:36:38Z", "expires": "2024-01-01T00:00:00Z"`),
		"several proofs": `{"type":"Create","proof":[{"type":"DataIntegrityProof"}]}`,
		"not an object":  `[]`,
	} {
		if _, err := federation.ReadProof([]byte(doc), now); !errors.Is(err, federation.ErrNoProof) {
			t.Errorf("%s: ReadProof = %v, want ErrNoProof", name, err)
		}
	}
	bad := strings.Replace(fep8b32Signed, fep8b32ProofValue, "zzz", 1)
	if _, err := federation.ReadProof([]byte(bad), now); !errors.Is(err, federation.ErrProofMismatch) {
		t.Errorf("ReadProof of a short proofValue = %v, want ErrProofMismatch", err)
	}
}

func TestEd25519MultikeyFEP521a(t *testing.T) {
	raw, _ := hex.DecodeString("2e6fcce36701dc791488e0d0b1745cc1e33a4c1c9fcc41c63bd343dbbe0970e6")
	const want = "z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
	if got := federation.EncodeEd25519Multikey(raw); got != want {
		t.Errorf("EncodeEd25519Multikey = %s, want %s", got, want)
	}
	pub, err := federation.ParseEd25519Multikey(want)
	if err != nil || !bytes.Equal(pub, raw) {
		t.Errorf("ParseEd25519Multikey = %x, %v", pub, err)
	}
	// Leading zero bytes survive the round trip.
	zeros := make(ed25519.PublicKey, ed25519.PublicKeySize)
	zeros[31] = 7
	if pub, err := federation.ParseEd25519Multikey(federation.EncodeEd25519Multikey(zeros)); err != nil || !bytes.Equal(pub, zeros) {
		t.Errorf("round trip of %x = %x, %v", zeros, pub, err)
	}
	for _, bad := range []string{"", "6Mk", "z0OIl", "z4MXj1wBzi9jUstyPMS4jQqB6KdJ"} {
		if _, err := federation.ParseEd25519Multikey(bad); err == nil {
			t.Errorf("ParseEd25519Multikey(%q) succeeded", bad)
		}
	}
}
