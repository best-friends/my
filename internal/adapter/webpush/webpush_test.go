// SPDX-License-Identifier: AGPL-3.0-only

package webpush_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/webpush"
	"github.com/best-friends/my/internal/app/kernel"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/domain/notification"
	"github.com/best-friends/my/pkg/testkit"
)

func b64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The example of RFC 8291 Appendix A.
const (
	rfcPlaintext  = "When I grow up, I want to be a watermelon"
	rfcASPrivate  = "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"
	rfcUAPrivate  = "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"
	rfcUAPublic   = "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"
	rfcSalt       = "DGv6ra1nlYgDCS1FRnbzlw"
	rfcAuthSecret = "BTBZMqHH6r4Tts7J_aSIgg" //nolint:gosec // G101: the example secret of RFC 8291
	rfcBody       = "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
)

func TestEncryptMatchesRFC8291(t *testing.T) {
	as, err := ecdh.P256().NewPrivateKey(b64(t, rfcASPrivate))
	if err != nil {
		t.Fatal(err)
	}
	ua, err := ecdh.P256().NewPublicKey(b64(t, rfcUAPublic))
	if err != nil {
		t.Fatal(err)
	}
	got, err := webpush.Encrypt([]byte(rfcPlaintext), ua, b64(t, rfcAuthSecret), as, b64(t, rfcSalt))
	if err != nil {
		t.Fatal(err)
	}
	if enc := base64.RawURLEncoding.EncodeToString(got); enc != rfcBody {
		t.Errorf("Encrypt =\n %s\nwant\n %s", enc, rfcBody)
	}
}

// decrypt is the user agent's side of RFC 8291 §3.4, for checking the messages that Send posts.
func decrypt(t *testing.T, body []byte, uaPrivate *ecdh.PrivateKey, authSecret []byte) []byte {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("body of %d bytes", len(body))
	}
	salt, rs, idLen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	if rs != 4096 || idLen != 65 {
		t.Fatalf("record size %d, key ID of %d bytes", rs, idLen)
	}
	asPublic, err := ecdh.P256().NewPublicKey(body[21 : 21+idLen])
	if err != nil {
		t.Fatal(err)
	}
	secret, err := uaPrivate.ECDH(asPublic)
	if err != nil {
		t.Fatal(err)
	}
	info := append(append([]byte("WebPush: info\x00"), uaPrivate.PublicKey().Bytes()...), asPublic.Bytes()...)
	ikm, _ := hkdf.Key(sha256.New, secret, authSecret, string(info), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, body[21+idLen:], nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain[len(plain)-1] != 0x02 {
		t.Fatalf("record delimiter %#x, want 0x02", plain[len(plain)-1])
	}
	return plain[:len(plain)-1]
}

type fakeHTTP struct {
	reqs []*kernel.HTTPRequest
	resp func(*kernel.HTTPRequest) (*kernel.HTTPResponse, error)
}

func (f *fakeHTTP) Do(_ context.Context, r *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	f.reqs = append(f.reqs, r)
	return f.resp(r)
}

func answer(code int, header http.Header) func(*kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	return func(r *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
		if header == nil {
			header = http.Header{}
		}
		return &kernel.HTTPResponse{StatusCode: code, Header: header, URL: r.URL, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}
}

var now = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func newSender(t *testing.T, h kernel.HTTPClient) (*webpush.Sender, string) {
	t.Helper()
	priv, pub, err := webpush.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, err := webpush.ParsePrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	s, err := webpush.New(webpush.Options{PrivateKey: key, Subject: "https://social.example", HTTP: h, Clock: testkit.NewClock(now)})
	if err != nil {
		t.Fatal(err)
	}
	if s.ServerKey() != pub {
		t.Fatalf("ServerKey = %s, want %s", s.ServerKey(), pub)
	}
	return s, pub
}

func subscription(t *testing.T) notification.PushSubscription {
	t.Helper()
	return notification.PushSubscription{
		ID: "sub", AccountID: "acct", TokenFamilyID: "fam", Endpoint: "https://push.example.net/send/abc",
		KeyP256DH: rfcUAPublic, KeyAuth: rfcAuthSecret, Alerts: []notification.Type{notification.TypeMention},
	}
}

func TestSendPostsAnEncryptedMessageWithVAPID(t *testing.T) {
	h := &fakeHTTP{resp: answer(http.StatusCreated, nil)}
	s, pub := newSender(t, h)
	res, err := s.Send(context.Background(), subscription(t), appnotification.PushMessage{Payload: []byte(`{"title":"hi"}`), TTL: 48 * time.Hour, Topic: "abc"})
	if err != nil || res.Outcome != appnotification.PushDelivered {
		t.Fatalf("Send = %+v, %v", res, err)
	}
	if len(h.reqs) != 1 {
		t.Fatalf("%d requests", len(h.reqs))
	}
	r := h.reqs[0]
	if r.Method != http.MethodPost || r.URL != "https://push.example.net/send/abc" || r.Purpose != kernel.HTTPPurposeWebPush {
		t.Errorf("request %s %s purpose %v", r.Method, r.URL, r.Purpose)
	}
	for k, want := range map[string]string{"Content-Encoding": "aes128gcm", "TTL": "172800", "Topic": "abc", "Urgency": "normal"} {
		if got := r.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	ua, _ := ecdh.P256().NewPrivateKey(b64(t, rfcUAPrivate))
	if got := decrypt(t, r.Body, ua, b64(t, rfcAuthSecret)); string(got) != `{"title":"hi"}` {
		t.Errorf("plaintext = %q", got)
	}

	// Authorization: vapid t=<JWT>, k=<public key> (RFC 8292 §3).
	auth := r.Header.Get("Authorization")
	token, key, ok := strings.Cut(strings.TrimPrefix(auth, "vapid t="), ", k=")
	if !strings.HasPrefix(auth, "vapid t=") || !ok || key != pub {
		t.Fatalf("Authorization = %q", auth)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT %q", token)
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(b64(t, parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != "https://push.example.net" || claims.Sub != "https://social.example" || claims.Exp != now.Add(12*time.Hour).Unix() {
		t.Errorf("claims = %+v", claims)
	}
	pk, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), b64(t, pub))
	if err != nil {
		t.Fatal(err)
	}
	sig := b64(t, parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if len(sig) != 64 || !ecdsa.Verify(pk, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Error("the VAPID signature does not verify")
	}
}

func TestSendClassifiesAnswers(t *testing.T) {
	cases := []struct {
		name  string
		resp  func(*kernel.HTTPRequest) (*kernel.HTTPResponse, error)
		want  appnotification.PushOutcome
		after time.Duration
	}{
		{"accepted", answer(http.StatusAccepted, nil), appnotification.PushDelivered, 0},
		{"gone", answer(http.StatusGone, nil), appnotification.PushGone, 0},
		{"not found", answer(http.StatusNotFound, nil), appnotification.PushGone, 0},
		{"too many", answer(http.StatusTooManyRequests, http.Header{"Retry-After": {"120"}}), appnotification.PushRetry, 2 * time.Minute},
		{"unavailable", answer(http.StatusServiceUnavailable, http.Header{"Retry-After": {now.Add(time.Hour).Format(http.TimeFormat)}}), appnotification.PushRetry, time.Hour},
		{"bad request", answer(http.StatusBadRequest, nil), appnotification.PushRejected, 0},
		{"forbidden destination", func(*kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
			return nil, fmt.Errorf("dial: %w", kernel.ErrForbiddenDestination)
		}, appnotification.PushGone, 0},
		{"network", func(*kernel.HTTPRequest) (*kernel.HTTPResponse, error) { return nil, errors.New("timeout") }, appnotification.PushRetry, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newSender(t, &fakeHTTP{resp: tc.resp})
			res, err := s.Send(context.Background(), subscription(t), appnotification.PushMessage{Payload: []byte("{}"), TTL: time.Hour})
			if err != nil || res.Outcome != tc.want || res.RetryAfter != tc.after {
				t.Errorf("Send = %+v, %v; want outcome %v after %v", res, err, tc.want, tc.after)
			}
		})
	}
}

func TestSendRefusesOversizedPayloads(t *testing.T) {
	s, _ := newSender(t, &fakeHTTP{resp: answer(http.StatusCreated, nil)})
	if _, err := s.Send(context.Background(), subscription(t), appnotification.PushMessage{Payload: bytes.Repeat([]byte("a"), webpush.MaxPayloadBytes+1)}); err == nil {
		t.Error("Send accepted an oversized payload")
	}
}

func TestParsePrivateKey(t *testing.T) {
	for _, bad := range []string{"", "not base64!", base64.RawURLEncoding.EncodeToString(make([]byte, 31)), base64.RawURLEncoding.EncodeToString(make([]byte, 32))} {
		if _, err := webpush.ParsePrivateKey(bad); err == nil {
			t.Errorf("ParsePrivateKey(%q) succeeded", bad)
		}
	}
	if _, err := webpush.ParsePrivateKey(rfcASPrivate + "="); err != nil {
		t.Errorf("ParsePrivateKey with padding: %v", err)
	}
}
