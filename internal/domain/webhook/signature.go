// SPDX-License-Identifier: AGPL-3.0-only

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// The signature of Standard Webhooks v1 (ADR-0060 "署名と秘密",
// https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md).

// Headers of a signed message.
const (
	HeaderID        = "webhook-id"
	HeaderTimestamp = "webhook-timestamp"
	HeaderSignature = "webhook-signature"
)

// SecretSize is the number of random bytes of a secret.
const SecretSize = 32

// SecretPrefix starts the text form of a secret.
const SecretPrefix = "whsec_"

// SecretOverlap is how long a replaced secret still signs after a rotation, beside the new one.
const SecretOverlap = 24 * time.Hour

// ErrInvalidSecret reports a secret that is not SecretSize bytes.
var ErrInvalidSecret = errors.New("webhook: invalid secret")

// FormatSecret returns the text form of a secret that an administrator copies to the receiver:
// whsec_ and the standard base64 of its bytes.
func FormatSecret(secret []byte) string {
	return SecretPrefix + base64.StdEncoding.EncodeToString(secret)
}

// ParseSecret returns the bytes of a secret in its text form.
func ParseSecret(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, SecretPrefix))
	if err != nil || len(b) != SecretSize {
		return nil, ErrInvalidSecret
	}
	return b, nil
}

// Sign returns the v1 signature of a message, "v1," and the standard base64 of the HMAC-SHA256
// under secret of "msgID.timestamp.body".
func Sign(secret []byte, msgID string, timestamp time.Time, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(msgID))
	mac.Write([]byte{'.'})
	mac.Write([]byte(strconv.FormatInt(timestamp.Unix(), 10)))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// SignatureHeader returns the webhook-signature header of a message signed with every secret
// in secrets (the current one first), separated by spaces.
func SignatureHeader(secrets [][]byte, msgID string, timestamp time.Time, body []byte) string {
	sigs := make([]string, len(secrets))
	for i, s := range secrets {
		sigs[i] = Sign(s, msgID, timestamp, body)
	}
	return strings.Join(sigs, " ")
}

// Verify reports whether header carries a valid v1 signature of the message under secret, as a
// receiver checks it. Tests use it.
func Verify(secret []byte, header, msgID string, timestamp time.Time, body []byte) bool {
	want := Sign(secret, msgID, timestamp, body)
	for sig := range strings.FieldsSeq(header) {
		if hmac.Equal([]byte(sig), []byte(want)) {
			return true
		}
	}
	return false
}

// SigningSecrets returns the IDs of the secrets that sign at now, the current one first, from
// the secrets of an endpoint newest first: the newest, and the one before it while the newest is
// younger than SecretOverlap.
func SigningSecrets(secrets []Secret, now time.Time) []Secret {
	switch {
	case len(secrets) == 0:
		return nil
	case len(secrets) == 1 || !now.Before(secrets[0].CreatedAt.Add(SecretOverlap)):
		return secrets[:1]
	}
	return secrets[:2]
}

// Secret is a secret of an endpoint (webhook_secrets), sealed: Sealed opens with the AAD of
// SecretAAD.
type Secret struct {
	ID              string
	EndpointID      string
	Sealed          []byte
	CreatedByUserID string // "" for the CLI and the system
	CreatedAt       time.Time
}
