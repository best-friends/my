// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // G505: RFC 6238 TOTP with HMAC-SHA-1 is what authenticator apps support; HMAC-SHA-1 is not broken
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"strings"
	"time"
)

// Two-factor authentication with TOTP (RFC 6238, FR-005).

// TOTPEnrollment is a TOTP authenticator that a user enrolled. The newest enrollment of a user
// without a revocation is the active one; a user without one signs in with the password alone.
type TOTPEnrollment struct {
	ID     string
	UserID string
	// SecretEnc is the shared secret sealed with kernel.Sealer (ADR-0019 §6) under
	// TOTPSecretAAD of the enrollment.
	SecretEnc  []byte
	EnrolledAt time.Time
}

// TOTPEnrollmentRevocation ends an enrollment.
type TOTPEnrollmentRevocation struct {
	EnrollmentID string
	RevokedAt    time.Time
}

// TOTPRecoveryCodeUse records that a recovery code of an enrollment was used.
type TOTPRecoveryCodeUse struct {
	EnrollmentID string
	CodeHash     []byte
	UsedAt       time.Time
}

// TOTP parameters: the defaults of RFC 6238 that every authenticator app supports (HMAC-SHA-1,
// six digits, 30-second steps) with a 160-bit secret (RFC 4226 §4 recommends at least that).
const (
	TOTPSecretSize = 20
	TOTPDigits     = 6
	TOTPPeriod     = 30 * time.Second
	// TOTPSkew is how many steps before and after the current one a code is accepted from, for
	// clocks that drift and codes typed near the end of their step (RFC 6238 §5.2).
	TOTPSkew = 1
)

// Recovery codes: RecoveryCodeCount codes of RecoveryCodeLength base32 characters (80 bits)
// each, shown in groups of four.
const (
	RecoveryCodeCount  = 10
	RecoveryCodeLength = 16
)

// TOTPSecretColumn names the column of a sealed secret in its additional authenticated data.
const TOTPSecretColumn = "totp_enrollments.secret_enc"

// totpBase32 is the encoding of secrets in otpauth URIs: base32 without padding (RFC 4648).
var totpBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// EncodeTOTPSecret returns the base32 form of a secret that users type into their app.
func EncodeTOTPSecret(secret []byte) string { return totpBase32.EncodeToString(secret) }

// TOTPStep returns the time step of t.
func TOTPStep(t time.Time) int64 { return t.Unix() / int64(TOTPPeriod/time.Second) }

// TOTPCode returns the code of secret at a time step (RFC 4226 §5.3 dynamic truncation).
func TOTPCode(secret []byte, step int64) string {
	mac := hmac.New(sha1.New, secret)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // G115: a step before 1970 is never asked for
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	const mod = 1_000_000 // 10^TOTPDigits
	code := []byte("000000")
	for i, n := len(code)-1, v%mod; i >= 0; i, n = i-1, n/10 {
		code[i] = byte('0' + n%10)
	}
	return string(code)
}

// MatchTOTP reports whether code is the code of secret at a step within TOTPSkew of now's, and
// which step it matched. It compares every candidate in constant time.
func MatchTOTP(secret []byte, code string, now time.Time) (step int64, ok bool) {
	code = NormalizeTOTPCode(code)
	if !IsTOTPCode(code) {
		return 0, false
	}
	cur := TOTPStep(now)
	for s := cur - TOTPSkew; s <= cur+TOTPSkew; s++ {
		if subtle.ConstantTimeCompare([]byte(TOTPCode(secret, s)), []byte(code)) == 1 && !ok {
			step, ok = s, true
		}
	}
	return step, ok
}

// NormalizeTOTPCode removes the spaces that people type or paste in the middle of a code.
func NormalizeTOTPCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == '-' {
			return -1
		}
		return r
	}, strings.TrimSpace(code))
}

// IsTOTPCode reports whether a normalized code has the form of a TOTP code: TOTPDigits ASCII
// digits.
func IsTOTPCode(code string) bool {
	if len(code) != TOTPDigits {
		return false
	}
	for i := range len(code) {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

// recoveryAlphabet is the lowercase base32 alphabet (RFC 4648), which has no 0, 1, 8, or 9 to
// mistake for letters.
const recoveryAlphabet = "abcdefghijklmnopqrstuvwxyz234567"

// RecoveryCodeFromBytes turns RecoveryCodeLength random bytes into a recovery code, using five
// bits of each, grouped by four with hyphens: "abcd-efgh-ijkl-mnop".
func RecoveryCodeFromBytes(b []byte) string {
	var sb strings.Builder
	for i := range RecoveryCodeLength {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(recoveryAlphabet[b[i]&0x1f])
	}
	return sb.String()
}

// NormalizeRecoveryCode returns the canonical form of a typed recovery code: lowercase, without
// hyphens and spaces, and "" when it cannot be a recovery code.
func NormalizeRecoveryCode(code string) string {
	code = strings.ToLower(NormalizeTOTPCode(code))
	if len(code) != RecoveryCodeLength {
		return ""
	}
	for i := range len(code) {
		if !strings.Contains(recoveryAlphabet, code[i:i+1]) {
			return ""
		}
	}
	return code
}

// HashRecoveryCode returns the stored digest of a normalized recovery code. The code has 80 bits
// of randomness, so a fast hash does not let a leaked database be searched for the codes.
func HashRecoveryCode(normalized string) []byte {
	sum := sha256.Sum256([]byte(normalized))
	return sum[:]
}
