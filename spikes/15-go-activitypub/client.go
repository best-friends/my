// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	cavage "code.superseriousbusiness.org/httpsig"
	rfc9421 "github.com/dadrus/httpsig"
	vocab "github.com/go-ap/activitypub"
)

const acceptActivity = `application/activity+json, application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

// SigningClient signs every outbound request. It implements sender-side
// double-knocking: try RFC 9421 first; on 400/401/403 retry once with
// draft-cavage-12 and remember the per-host preference.
type SigningClient struct {
	HTTP  *http.Client
	KeyID string
	Key   *rsa.PrivateKey

	mu   sync.Mutex
	pref map[string]string // host -> scheme that worked
}

func (c *SigningClient) preferred(host string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.pref[host]; ok {
		return s
	}
	return SchemeRFC9421
}

func (c *SigningClient) remember(host, scheme string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pref == nil {
		c.pref = map[string]string{}
	}
	c.pref[host] = scheme
}

// Preference reports the remembered scheme for host ("" if none).
func (c *SigningClient) Preference(host string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pref[host]
}

// Do sends a signed request built by newReq (called once per attempt).
func (c *SigningClient) Do(ctx context.Context, method, rawURL string, body []byte, contentType string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	first := c.preferred(u.Host)
	resp, err := c.attempt(ctx, method, rawURL, body, contentType, first)
	if err != nil {
		return nil, err
	}
	if first == SchemeRFC9421 && isSignatureRejection(resp.StatusCode) {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		resp, err = c.attempt(ctx, method, rawURL, body, contentType, SchemeCavage)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 300 {
			c.remember(u.Host, SchemeCavage)
		}
		return resp, nil
	}
	if resp.StatusCode < 300 {
		c.remember(u.Host, first)
	}
	return resp, nil
}

func isSignatureRejection(code int) bool {
	return code == http.StatusBadRequest || code == http.StatusUnauthorized || code == http.StatusForbidden
}

func (c *SigningClient) attempt(ctx context.Context, method, rawURL string, body []byte, contentType, scheme string) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if method == http.MethodGet {
		req.Header.Set("Accept", acceptActivity)
	}
	req.Header.Set("User-Agent", "my-spike15/0 (+https://github.com/best-friends/my)")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	switch scheme {
	case SchemeRFC9421:
		err = c.signRFC9421(req)
	default:
		err = c.signCavage(req, body)
	}
	if err != nil {
		return nil, fmt.Errorf("sign %s: %w", scheme, err)
	}
	return c.HTTP.Do(req)
}

func (c *SigningClient) signRFC9421(req *http.Request) error {
	components := []string{"@method", "@target-uri"}
	opts := []rfc9421.SignerOption{rfc9421.WithTTL(5 * time.Minute)}
	if req.Body != nil && req.Body != http.NoBody {
		components = append(components, "content-digest")
		opts = append(opts, rfc9421.WithContentDigestAlgorithm(rfc9421.Sha256))
	}
	opts = append(opts, rfc9421.WithComponents(components...))
	s, err := rfc9421.NewSigner(rfc9421.Key{
		KeyID:     c.KeyID,
		Algorithm: rfc9421.RsaPkcs1v15Sha256,
		Key:       c.Key,
	}, opts...)
	if err != nil {
		return err
	}
	h, err := s.Sign(rfc9421.MessageFromRequest(req))
	if err != nil {
		return err
	}
	req.Header = h
	return nil
}

func (c *SigningClient) signCavage(req *http.Request, body []byte) error {
	headers := []string{cavage.RequestTarget, "host", "date"}
	if req.Method == http.MethodPost {
		headers = append(headers, "digest")
	}
	// Gotcha: the signer reads "host" from req.Header only (client requests
	// carry it in req.Host), so copy it over; net/http ignores Header["Host"].
	req.Header.Set("Host", req.URL.Host)
	// Signers are not goroutine-safe: build one per request.
	s, _, err := cavage.NewSigner([]cavage.Algorithm{cavage.RSA_SHA256}, cavage.DigestSha256, headers, cavage.Signature, 0)
	if err != nil {
		return err
	}
	return s.SignRequest(c.Key, c.KeyID, req, body)
}

// Post delivers an activity and fails on a non-2xx response.
func (c *SigningClient) Post(ctx context.Context, inbox string, body []byte) error {
	resp, err := c.Do(ctx, http.MethodPost, inbox, body, contentTypeActivity)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("deliver %s: %s: %s", inbox, resp.Status, b)
	}
	return nil
}

// HTTPKeyResolver fetches the actor that owns a keyId (signed GET, so it also
// works against Mastodon's authorized-fetch mode). No cache, no SSRF guard:
// both are product-code concerns.
type HTTPKeyResolver struct {
	Client *SigningClient
}

func (h *HTTPKeyResolver) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := h.Client.Do(ctx, http.MethodGet, rawURL, nil, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxInboxBody))
}

func (h *HTTPKeyResolver) ResolveKey(ctx context.Context, keyID string) (*RemoteActor, error) {
	docURL, _, _ := strings.Cut(keyID, "#")
	b, err := h.fetch(ctx, docURL)
	if err != nil {
		return nil, err
	}

	// Some servers (e.g. GoToSocial for non-fragment key ids) return a bare
	// Key object; follow its owner. go-ap does not model "Key", so peek first.
	var peek struct {
		Type         any    `json:"type"`
		Owner        string `json:"owner"`
		PublicKeyPem string `json:"publicKeyPem"`
	}
	if err := json.Unmarshal(b, &peek); err != nil {
		return nil, err
	}
	if peek.PublicKeyPem != "" && peek.Owner != "" {
		if b, err = h.fetch(ctx, peek.Owner); err != nil {
			return nil, err
		}
	}

	item, err := vocab.UnmarshalJSON(b)
	if err != nil {
		return nil, err
	}
	var ra *RemoteActor
	err = vocab.OnActor(item, func(a *vocab.Actor) error {
		if string(a.PublicKey.ID) != keyID || a.PublicKey.Owner.String() != a.ID.String() {
			return errors.New("key is not owned by the fetched actor")
		}
		pub, err := parsePublicKeyPEM(a.PublicKey.PublicKeyPem)
		if err != nil {
			return err
		}
		ra = &RemoteActor{ID: a.ID.String(), KeyID: keyID, PublicKey: pub}
		if a.Inbox != nil {
			ra.Inbox = a.Inbox.GetLink().String()
		}
		if a.Endpoints != nil && a.Endpoints.SharedInbox != nil {
			ra.SharedInbox = a.Endpoints.SharedInbox.GetLink().String()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if ra == nil {
		return nil, errors.New("keyId did not resolve to an actor")
	}
	return ra, nil
}

func parsePublicKeyPEM(s string) (any, error) {
	blk, _ := pem.Decode([]byte(s))
	if blk == nil {
		return nil, errors.New("no PEM block in publicKeyPem")
	}
	if k, err := x509.ParsePKIXPublicKey(blk.Bytes); err == nil {
		return k, nil
	}
	return x509.ParsePKCS1PublicKey(blk.Bytes)
}
