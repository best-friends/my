// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	cavage "code.superseriousbusiness.org/httpsig"
	rfc9421 "github.com/dadrus/httpsig"
)

// Signature schemes.
const (
	SchemeRFC9421 = "rfc9421"
	SchemeCavage  = "draft-cavage-12"
)

var (
	errNoSignature = errors.New("no HTTP signature")
	errPolicy      = errors.New("signature policy violation")
)

// RemoteActor is what the inbox needs to know about a signer.
type RemoteActor struct {
	ID          string
	Inbox       string
	SharedInbox string
	KeyID       string
	PublicKey   crypto.PublicKey
}

// KeyResolver maps a keyId to the actor that owns it. It must check that the
// key really belongs to the actor (publicKey.id == keyId, owner == actor.id).
type KeyResolver interface {
	ResolveKey(ctx context.Context, keyID string) (*RemoteActor, error)
}

// Verified is the result of a successful verification.
type Verified struct {
	ActorID string
	Actor   *RemoteActor
	Scheme  string
}

// Verifier accepts both draft-cavage-12 and RFC 9421 signatures
// ("double-knocking" on the receiving side means: accept either).
type Verifier struct {
	Resolver KeyResolver
	// PublicBase is this server's public origin. It is used to rebuild
	// @target-uri / (request-target) instead of trusting X-Forwarded-*.
	PublicBase *url.URL
	// MaxAge / MaxSkew mirror Mastodon (12h expiration window, 1h clock skew).
	MaxAge  time.Duration
	MaxSkew time.Duration
	Now     func() time.Time
}

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

func (v *Verifier) maxAge() time.Duration {
	if v.MaxAge > 0 {
		return v.MaxAge
	}
	return 12 * time.Hour
}

func (v *Verifier) maxSkew() time.Duration {
	if v.MaxSkew > 0 {
		return v.MaxSkew
	}
	return time.Hour
}

// Verify checks the signature of r whose body has already been read into body.
func (v *Verifier) Verify(r *http.Request, body []byte) (*Verified, error) {
	switch {
	case r.Header.Get("Signature-Input") != "" && r.Header.Get("Signature") != "":
		return v.verifyRFC9421(r, body)
	case r.Header.Get("Signature") != "":
		return v.verifyCavage(r, body)
	default:
		return nil, errNoSignature
	}
}

// --- RFC 9421 (github.com/dadrus/httpsig) ---

type resolverFunc func(ctx context.Context, keyID string) (rfc9421.Key, error)

func (f resolverFunc) ResolveKey(ctx context.Context, keyID string) (rfc9421.Key, error) {
	return f(ctx, keyID)
}

func rfcAlgorithm(pub crypto.PublicKey) (rfc9421.SignatureAlgorithm, error) {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		// Fediverse keys are RSASSA-PKCS1-v1_5; PSS would need the alg param.
		return rfc9421.RsaPkcs1v15Sha256, nil
	case ed25519.PublicKey:
		return rfc9421.Ed25519, nil
	case *ecdsa.PublicKey:
		switch k.Curve.Params().BitSize {
		case 256:
			return rfc9421.EcdsaP256Sha256, nil
		case 384:
			return rfc9421.EcdsaP384Sha384, nil
		}
	}
	return "", fmt.Errorf("unsupported key type %T", pub)
}

func (v *Verifier) verifyRFC9421(r *http.Request, body []byte) (*Verified, error) {
	var signer *RemoteActor
	resolver := resolverFunc(func(ctx context.Context, keyID string) (rfc9421.Key, error) {
		ra, err := v.Resolver.ResolveKey(ctx, keyID)
		if err != nil {
			return rfc9421.Key{}, err
		}
		alg, err := rfcAlgorithm(ra.PublicKey)
		if err != nil {
			return rfc9421.Key{}, err
		}
		signer = ra
		return rfc9421.Key{KeyID: keyID, Algorithm: alg, Key: ra.PublicKey}, nil
	})

	required := []string{"@method", "@target-uri"}
	if len(body) > 0 {
		// The library recomputes and compares Content-Digest when it is covered.
		required = append(required, "content-digest")
	}
	ver, err := rfc9421.NewVerifier(resolver,
		rfc9421.WithValidateAllSignatures(),
		rfc9421.WithRequiredComponents(required...),
		rfc9421.WithCreatedTimestampRequired(true),
		rfc9421.WithExpiredTimestampRequired(false),
		rfc9421.WithMaxAge(v.maxAge()),
		rfc9421.WithValidityTolerance(v.maxSkew()),
	)
	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	msg := rfc9421.MessageFromRequest(r)
	// Server-side requests carry only the path; rebuild the public URI.
	target := *v.PublicBase
	target.Path = r.URL.Path
	target.RawPath = r.URL.RawPath
	target.RawQuery = r.URL.RawQuery
	msg.URL = &target
	msg.Authority = v.PublicBase.Host

	if err := ver.Verify(msg); err != nil {
		return nil, fmt.Errorf("rfc9421: %w", err)
	}
	if signer == nil {
		return nil, errors.New("rfc9421: no key resolved")
	}
	return &Verified{ActorID: signer.ID, Actor: signer, Scheme: SchemeRFC9421}, nil
}

// --- draft-cavage-12 (code.superseriousbusiness.org/httpsig) ---

// cavageParams parses the Signature header. The library verifies the
// cryptography but does not expose the covered headers, nor check Date or
// Digest, so that policy is ours.
func cavageParams(h string) map[string]string {
	out := map[string]string{}
	for _, part := range splitOutsideQuotes(h) {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[strings.ToLower(k)] = strings.Trim(val, `"`)
	}
	return out
}

func splitOutsideQuotes(s string) []string {
	var parts []string
	inQ := false
	start := 0
	for i, c := range s {
		switch {
		case c == '"':
			inQ = !inQ
		case c == ',' && !inQ:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func (v *Verifier) verifyCavage(r *http.Request, body []byte) (*Verified, error) {
	params := cavageParams(r.Header.Get("Signature"))
	headers := strings.Fields(strings.ToLower(params["headers"]))
	if len(headers) == 0 {
		headers = []string{"date"} // spec default
	}

	// Policy (matches what Mastodon requires of senders).
	for _, need := range []string{"(request-target)", "host"} {
		if !slices.Contains(headers, need) {
			return nil, fmt.Errorf("%w: %s not signed", errPolicy, need)
		}
	}
	switch {
	case slices.Contains(headers, "date"):
		d, err := http.ParseTime(r.Header.Get("Date"))
		if err != nil {
			return nil, fmt.Errorf("%w: bad Date", errPolicy)
		}
		now := v.now()
		if d.After(now.Add(v.maxSkew())) || d.Before(now.Add(-v.maxAge())) {
			return nil, fmt.Errorf("%w: Date outside window", errPolicy)
		}
	case slices.Contains(headers, "(created)"):
		// checked by the library (future-dated) — expiry window is ours to add.
	default:
		return nil, fmt.Errorf("%w: neither date nor (created) signed", errPolicy)
	}
	if r.Method == http.MethodPost {
		if !slices.Contains(headers, "digest") {
			return nil, fmt.Errorf("%w: digest not signed", errPolicy)
		}
		if err := checkDigest(r.Header.Get("Digest"), body); err != nil {
			return nil, err
		}
	}

	cv, err := cavage.NewVerifier(r)
	if err != nil {
		return nil, fmt.Errorf("cavage: %w", err)
	}
	ra, err := v.Resolver.ResolveKey(r.Context(), cv.KeyId())
	if err != nil {
		return nil, fmt.Errorf("cavage: resolve key: %w", err)
	}
	// `algorithm` is deprecated/ignored (hs2019); pick from the key type.
	var algs []cavage.Algorithm
	switch ra.PublicKey.(type) {
	case *rsa.PublicKey:
		algs = []cavage.Algorithm{cavage.RSA_SHA256, cavage.RSA_SHA512}
	case ed25519.PublicKey:
		algs = []cavage.Algorithm{cavage.ED25519}
	default:
		return nil, fmt.Errorf("cavage: unsupported key %T", ra.PublicKey)
	}
	var errs []error
	for _, a := range algs {
		if err := cv.Verify(ra.PublicKey, a); err != nil {
			errs = append(errs, err)
			continue
		}
		return &Verified{ActorID: ra.ID, Actor: ra, Scheme: SchemeCavage}, nil
	}
	return nil, fmt.Errorf("cavage: %w", errors.Join(errs...))
}

func checkDigest(header string, body []byte) error {
	sum := sha256.Sum256(body)
	want := base64.StdEncoding.EncodeToString(sum[:])
	for _, part := range strings.Split(header, ",") {
		alg, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(alg, "SHA-256") &&
			subtle.ConstantTimeCompare([]byte(val), []byte(want)) == 1 {
			return nil
		}
	}
	return fmt.Errorf("%w: Digest mismatch or unsupported", errPolicy)
}
