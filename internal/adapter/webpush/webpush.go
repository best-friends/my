// SPDX-License-Identifier: AGPL-3.0-only

// Package webpush implements notification.PushSender: it encrypts a message for a browser's
// subscription with the aes128gcm content coding of RFC 8291 (on RFC 8188), signs a VAPID
// authorization with the server's P-256 key (RFC 8292), and posts the message to the push
// service (RFC 8030) through the SSRF-guarded kernel.HTTPClient (ADR-0019 §1). It uses only the
// standard library's cryptography.
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	appnotification "github.com/best-friends/my/internal/app/notification"
	"github.com/best-friends/my/internal/domain/notification"
)

// recordSize is the record size of the aes128gcm coding (RFC 8188 §2.1). A message is one
// record: the payloads stay far below the 4078 bytes that RFC 8291 §4 lets a push service
// refuse beyond.
const recordSize = 4096

// p256PointBytes is the length of an uncompressed P-256 point, the key ID of the coding.
const p256PointBytes = 65

// MaxPayloadBytes is the largest plaintext that fits one record with its delimiter and the
// 16-byte tag, and that every push service accepts (RFC 8291 §4).
const MaxPayloadBytes = 3993

// vapidLifetime is how long a VAPID token is valid; RFC 8292 §2 allows at most 24 hours.
const vapidLifetime = 12 * time.Hour

// Options configure a Sender.
type Options struct {
	// PrivateKey is the server's VAPID private key (ParsePrivateKey).
	PrivateKey *ecdsa.PrivateKey
	// Subject is the contact of the server operator that push services may use (RFC 8292
	// §2.1): an https URL of the instance or a mailto: URI.
	Subject string
	// HTTP sends the messages.
	HTTP  kernel.HTTPClient
	Clock kernel.Clock
	// Random is the source of the ephemeral keys and salts; nil means crypto/rand.
	Random io.Reader
}

// Sender implements notification.PushSender.
type Sender struct {
	o         Options
	serverKey string
}

var _ appnotification.PushSender = (*Sender)(nil)

// New returns a Sender with o.
func New(o Options) (*Sender, error) {
	if o.PrivateKey == nil || o.HTTP == nil || o.Clock == nil || o.Subject == "" {
		return nil, errors.New("webpush: a sender needs a private key, a subject, an HTTP client, and a clock")
	}
	pub, err := o.PrivateKey.PublicKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("webpush: the VAPID public key: %w", err)
	}
	return &Sender{o: o, serverKey: base64.RawURLEncoding.EncodeToString(pub)}, nil
}

// ParsePrivateKey parses a VAPID private key: the 32-byte P-256 scalar in base64url, padded
// or not, as GenerateKey returns it and web-push libraries print it.
func ParsePrivateKey(s string) (*ecdsa.PrivateKey, error) {
	b, err := base64.RawURLEncoding.DecodeString(trimPadding(s))
	if err != nil || len(b) != 32 {
		return nil, errors.New("webpush: a VAPID private key is 32 bytes in base64url")
	}
	k, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), b)
	if err != nil {
		return nil, fmt.Errorf("webpush: the VAPID private key is not a P-256 key: %w", err)
	}
	return k, nil
}

// GenerateKey returns a new VAPID key pair: the private key as ParsePrivateKey reads it and the
// public key that browsers subscribe with, both in unpadded base64url.
func GenerateKey() (privateKey, publicKey string, err error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	priv, err := k.Bytes()
	if err != nil {
		return "", "", err
	}
	pub, err := k.PublicKey.Bytes()
	if err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(priv), base64.RawURLEncoding.EncodeToString(pub), nil
}

func trimPadding(s string) string {
	for len(s) > 0 && s[len(s)-1] == '=' {
		s = s[:len(s)-1]
	}
	return s
}

// ServerKey implements notification.PushSender.
func (s *Sender) ServerKey() string { return s.serverKey }

// Send implements notification.PushSender.
func (s *Sender) Send(ctx context.Context, sub notification.PushSubscription, m appnotification.PushMessage) (appnotification.PushResult, error) {
	if len(m.Payload) > MaxPayloadBytes {
		return appnotification.PushResult{}, fmt.Errorf("webpush: a payload of %d bytes exceeds %d", len(m.Payload), MaxPayloadBytes)
	}
	uaPublic, authSecret, err := notification.DecodePushKeys(sub.KeyP256DH, sub.KeyAuth)
	if err != nil {
		return appnotification.PushResult{}, err
	}
	random := s.o.Random
	if random == nil {
		random = rand.Reader
	}
	asPrivate, err := ecdh.P256().GenerateKey(random)
	if err != nil {
		return appnotification.PushResult{}, err
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(random, salt); err != nil {
		return appnotification.PushResult{}, err
	}
	body, err := Encrypt(m.Payload, uaPublic, authSecret, asPrivate, salt)
	if err != nil {
		return appnotification.PushResult{}, err
	}
	auth, err := s.vapid(sub.Endpoint)
	if err != nil {
		return appnotification.PushResult{}, err
	}
	h := http.Header{}
	h.Set("Authorization", auth)
	h.Set("Content-Encoding", "aes128gcm")
	h.Set("Content-Type", "application/octet-stream")
	h.Set("TTL", strconv.Itoa(int(m.TTL/time.Second)))
	h.Set("Urgency", "normal")
	if m.Topic != "" {
		h.Set("Topic", m.Topic)
	}
	resp, err := s.o.HTTP.Do(ctx, &kernel.HTTPRequest{
		Method: http.MethodPost, URL: sub.Endpoint, Header: h, Body: body, Purpose: kernel.HTTPPurposeWebPush,
	})
	switch {
	case errors.Is(err, kernel.ErrForbiddenDestination), errors.Is(err, kernel.ErrInvalidHTTPRequest):
		return appnotification.PushResult{Outcome: appnotification.PushGone}, nil
	case err != nil:
		return appnotification.PushResult{Outcome: appnotification.PushRetry}, nil
	}
	_, _ = resp.ReadAll() // the answer's body tells nothing the status does not
	return classify(resp, s.o.Clock.Now()), nil
}

// classify turns a push service's answer into a result (RFC 8030 §5, §8.4).
func classify(resp *kernel.HTTPResponse, now time.Time) appnotification.PushResult {
	r := appnotification.PushResult{StatusCode: resp.StatusCode}
	switch c := resp.StatusCode; {
	case c >= 200 && c < 300:
		r.Outcome = appnotification.PushDelivered
	case c == http.StatusNotFound || c == http.StatusGone:
		r.Outcome = appnotification.PushGone
	case c == http.StatusTooManyRequests || c >= 500:
		r.Outcome = appnotification.PushRetry
		r.RetryAfter = retryAfter(resp.Header.Get("Retry-After"), now)
	default:
		r.Outcome = appnotification.PushRejected
	}
	return r
}

// retryAfter parses a Retry-After field: seconds or an HTTP date. It returns zero for none.
func retryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

// vapid returns the Authorization field of a message to endpoint (RFC 8292 §3): a JWT signed
// with ES256 whose audience is the endpoint's origin.
func (s *Sender) vapid(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("webpush: the endpoint: %w", err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, err := json.Marshal(map[string]any{
		"aud": u.Scheme + "://" + u.Host,
		"exp": s.o.Clock.Now().Add(vapidLifetime).Unix(),
		"sub": s.o.Subject,
	})
	if err != nil {
		return "", err
	}
	input := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(input))
	random := s.o.Random
	if random == nil {
		random = rand.Reader
	}
	r, sig, err := ecdsa.Sign(random, s.o.PrivateKey, digest[:])
	if err != nil {
		return "", err
	}
	raw := make([]byte, 64)
	r.FillBytes(raw[:32])
	sig.FillBytes(raw[32:])
	token := input + "." + base64.RawURLEncoding.EncodeToString(raw)
	return "vapid t=" + token + ", k=" + s.serverKey, nil
}

// Encrypt encrypts plaintext for the user agent's public key and authentication secret with
// the aes128gcm content coding (RFC 8291 §3.4), using asPrivate as the application server's
// ephemeral key and salt as the coding's salt. It returns the message body: the coding's
// header (salt, record size, and the ephemeral public key as the key ID) and a single record.
func Encrypt(plaintext []byte, uaPublic *ecdh.PublicKey, authSecret []byte, asPrivate *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	if len(salt) != 16 {
		return nil, errors.New("webpush: the salt is 16 bytes")
	}
	ecdhSecret, err := asPrivate.ECDH(uaPublic)
	if err != nil {
		return nil, err
	}
	uaBytes, asBytes := uaPublic.Bytes(), asPrivate.PublicKey().Bytes()
	keyInfo := append(append([]byte("WebPush: info\x00"), uaBytes...), asBytes...)
	ikm, err := hkdf.Key(sha256.New, ecdhSecret, authSecret, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(salt)
	_ = binary.Write(&b, binary.BigEndian, uint32(recordSize))
	if len(asBytes) != p256PointBytes {
		return nil, errors.New("webpush: the ephemeral key is not an uncompressed P-256 point")
	}
	b.WriteByte(p256PointBytes)
	b.Write(asBytes)
	record := append(append([]byte{}, plaintext...), 0x02) // the last record's delimiter, no padding
	return gcm.Seal(b.Bytes(), nonce, record, nil), nil
}
