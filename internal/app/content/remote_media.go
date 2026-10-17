// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// RemoteMediaInput is a remote media file as a received document declares it.
type RemoteMediaInput struct {
	OwnerAccountID      string
	RemoteURL           string
	DeclaredContentType string
	DeclaredMediaType   content.MediaType
	Width, Height       int
	Blurhash            string
	Focus               *content.MediaFocus
	Description         string
}

// RecordRemoteMedia records a remote media file inside tx: the media file (origin remote) and
// its remote subtype with the declared values (docs/architecture/federation.md §9). The file
// is not fetched. Recording the same owner and URL again returns the file recorded first.
func (s *MediaService) RecordRemoteMedia(ctx context.Context, tx uow.Tx, in RemoteMediaInput) (content.RemoteMedia, error) {
	existing, err := s.d.Repository.RemoteMediaByURL(ctx, tx, in.OwnerAccountID, in.RemoteURL)
	switch {
	case err == nil:
		return existing, nil
	case !errors.Is(err, ErrNotFound):
		return content.RemoteMedia{}, err
	}
	kind := in.DeclaredMediaType
	if kind == "" {
		kind = content.MediaTypeUnknown
	}
	m := content.RemoteMedia{
		MediaID: s.d.IDs.NewID(), OwnerAccountID: in.OwnerAccountID, RemoteURL: in.RemoteURL,
		DeclaredContentType: in.DeclaredContentType, DeclaredMediaType: kind,
		Width: in.Width, Height: in.Height, Blurhash: in.Blurhash, Description: in.Description,
	}
	if in.Focus != nil {
		m.Focus = &content.MediaFocus{MediaID: m.MediaID, X: in.Focus.X, Y: in.Focus.Y}
	}
	file := content.MediaFile{ID: m.MediaID, OwnerAccountID: in.OwnerAccountID, Origin: content.MediaOriginRemote, CreatedAt: s.d.Clock.Now()}
	if err := s.d.Repository.InsertMediaFile(ctx, tx, file); err != nil {
		return content.RemoteMedia{}, err
	}
	inserted, err := s.d.Repository.InsertRemoteMedia(ctx, tx, m)
	if err != nil {
		return content.RemoteMedia{}, err
	}
	if !inserted {
		// A concurrent transaction recorded the same URL after the lookup; the retry finds it.
		return content.RemoteMedia{}, fmt.Errorf("remote media %s: %w", in.RemoteURL, uow.ErrRetryable)
	}
	return m, nil
}

// ProxyPathPrefix is the URL path under which the media proxy serves remote media files:
// ProxyPathPrefix + "{media_id}/{variant}?sig=...".
const ProxyPathPrefix = "/media/proxy/"

// ErrProxyUpstream reports a remote server that did not answer the media proxy's fetch with
// the file: a network error, a timeout, or a 5xx status. A later request may succeed.
var ErrProxyUpstream = errors.New("content: the remote server did not return the media file")

// AccountDomains tells the domain of an account. The account module implements it.
type AccountDomains interface {
	// AccountDomain returns the domain of an account ("" for a local one).
	AccountDomain(ctx context.Context, accountID string) (string, error)
}

// MediaFetchPolicy is the port through which the media proxy reads the domain restrictions on
// fetching media (the media_fetch restriction of a reject_media domain action, data-model §5.2).
// *moderation.FederationPolicy implements it.
type MediaFetchPolicy interface {
	// RejectsMedia reports whether the media of domain, or of a parent domain, are not fetched.
	RejectsMedia(ctx context.Context, domain string) (bool, error)
}

// ProxyDeps are the ports a MediaProxy uses.
type ProxyDeps struct {
	Repository MediaRepository
	HTTP       kernel.HTTPClient
	Accounts   AccountDomains
	Policy     MediaFetchPolicy
	// CacheTTL returns the current remote_media_cache_ttl instance setting: the max-age of the
	// proxy's responses.
	CacheTTL func(ctx context.Context) (time.Duration, error)
	// Keys are the HMAC-SHA256 keys of the proxy URLs' signatures. The first signs; every one
	// verifies, so that URLs signed before a key rotation keep working while the old key is kept.
	Keys [][]byte
	// BaseURL is the origin of the proxy URLs ("https://example.com").
	BaseURL string
	// Logger logs unexpected errors; nil means slog.Default().
	Logger *slog.Logger
}

// MediaProxy relays remote media files from the instance's origin (ADR-0005,
// docs/architecture/federation.md §9), so that clients never contact remote servers: their IP
// addresses stay private and the web client's CSP needs no remote origins. URLs are signed
// with an HMAC of the media ID and the variant, so the proxy relays only the files that the
// instance has recorded, never an arbitrary URL. The file is fetched through the SSRF-guarded
// HTTPClient, its format is detected from its content (ADR-0019 §8: SVG and anything outside
// the allow list is refused), and its size is bounded: 16 MiB for images and 100 MiB for video
// and audio (ADR-0019 §1). The instance keeps no copy; a CDN in front of it caches the
// responses for remote_media_cache_ttl.
type MediaProxy struct {
	d ProxyDeps
}

// NewMediaProxy returns a MediaProxy on d. It needs at least one key.
func NewMediaProxy(d ProxyDeps) (*MediaProxy, error) {
	if len(d.Keys) == 0 {
		return nil, errors.New("content: the media proxy needs a signing key")
	}
	for _, k := range d.Keys {
		if len(k) < sha256.Size {
			return nil, errors.New("content: a media proxy key must be at least 32 bytes")
		}
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	d.BaseURL = strings.TrimSuffix(d.BaseURL, "/")
	return &MediaProxy{d: d}, nil
}

// proxyVariants are the variants the proxy serves: the remote file itself.
var proxyVariants = map[content.MediaVariantKind]bool{content.VariantFull: true}

// URL returns the signed proxy URL of a remote media file's variant.
func (p *MediaProxy) URL(mediaID string, variant content.MediaVariantKind) string {
	return p.d.BaseURL + ProxyPathPrefix + url.PathEscape(mediaID) + "/" + url.PathEscape(string(variant)) +
		"?sig=" + p.sign(p.d.Keys[0], mediaID, string(variant))
}

func (p *MediaProxy) sign(key []byte, mediaID, variant string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(kernel.AAD("media-proxy", mediaID, variant))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verify reports whether sig is the signature of mediaID and variant under one of the keys.
func (p *MediaProxy) verify(mediaID, variant, sig string) bool {
	ok := false
	for _, k := range p.d.Keys {
		// Compare under every key, so that the time does not tell which key matched.
		if hmac.Equal([]byte(p.sign(k, mediaID, variant)), []byte(sig)) {
			ok = true
		}
	}
	return ok
}

// ProxiedMedia is a remote media file on its way to the client.
type ProxiedMedia struct {
	// ContentType is the media type detected from the content.
	ContentType string
	// Length is the length of Body, or -1 when the remote server did not tell it.
	Length int64
	// MaxAge is how long the response may be cached.
	MaxAge time.Duration
	// Body reads the file. A read past the size limit fails with an error wrapping
	// kernel.ErrResponseTooLarge; the caller then aborts the response. The caller closes it.
	Body io.ReadCloser
}

// Open fetches the variant of a remote media file for a proxy request. It fails with an error
// wrapping ErrNotFound when the signature does not match, when the file is not a recorded
// remote media file, when its domain's media are rejected, when the HTTPClient refuses its
// destination, when the remote server answers with a 4xx status, and when the content is not
// in an allowed format or is larger than the limit; it fails with ErrProxyUpstream when the
// remote server cannot be reached or answers with a 5xx status.
func (p *MediaProxy) Open(ctx context.Context, mediaID, variant, sig string) (*ProxiedMedia, error) {
	notFound := func(why string) error {
		return fmt.Errorf("proxy media %s/%s: %s: %w", mediaID, variant, why, ErrNotFound)
	}
	if !p.verify(mediaID, variant, sig) {
		return nil, notFound("bad signature")
	}
	if !proxyVariants[content.MediaVariantKind(variant)] {
		return nil, notFound("unknown variant")
	}
	m, err := p.d.Repository.FindRemoteMedia(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(m.RemoteURL)
	if err != nil || u.Hostname() == "" {
		return nil, notFound("malformed remote URL")
	}
	owner, err := p.d.Accounts.AccountDomain(ctx, m.OwnerAccountID)
	if err != nil {
		return nil, fmt.Errorf("proxy media %s: owner: %w", mediaID, err)
	}
	for _, domain := range []string{owner, strings.ToLower(u.Hostname())} {
		if domain == "" {
			continue
		}
		rejected, err := p.d.Policy.RejectsMedia(ctx, domain)
		if err != nil {
			return nil, err
		}
		if rejected {
			return nil, notFound("media of " + domain + " are rejected")
		}
	}
	maxAge, err := p.d.CacheTTL(ctx)
	if err != nil {
		return nil, err
	}
	return p.fetch(ctx, m, maxAge)
}

func (p *MediaProxy) fetch(ctx context.Context, m content.RemoteMedia, maxAge time.Duration) (*ProxiedMedia, error) {
	purpose := kernel.HTTPPurposeVideo
	if m.DeclaredMediaType == content.MediaTypeImage {
		purpose = kernel.HTTPPurposeImage
	}
	resp, err := p.d.HTTP.Do(ctx, &kernel.HTTPRequest{
		URL: m.RemoteURL, Purpose: purpose,
		Header: http.Header{"Accept": {"image/*, video/*, audio/*;q=0.9, */*;q=0.1"}},
	})
	switch {
	case errors.Is(err, kernel.ErrForbiddenDestination), errors.Is(err, kernel.ErrResponseTooLarge),
		errors.Is(err, kernel.ErrInvalidHTTPRequest), errors.Is(err, kernel.ErrTooManyRedirects):
		return nil, fmt.Errorf("proxy media %s: %w: %w", m.MediaID, ErrNotFound, err)
	case err != nil:
		return nil, fmt.Errorf("proxy media %s: %w: %w", m.MediaID, ErrProxyUpstream, err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = resp.Body.Close()
		}
	}()
	switch {
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("proxy media %s: status %d: %w", m.MediaID, resp.StatusCode, ErrProxyUpstream)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("proxy media %s: status %d: %w", m.MediaID, resp.StatusCode, ErrNotFound)
	}

	head := make([]byte, content.SniffHeadLength)
	n, err := io.ReadFull(resp.Body, head)
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
	case errors.Is(err, kernel.ErrResponseTooLarge):
		return nil, fmt.Errorf("proxy media %s: %w: %w", m.MediaID, ErrNotFound, err)
	case err != nil:
		return nil, fmt.Errorf("proxy media %s: read: %w: %w", m.MediaID, ErrProxyUpstream, err)
	}
	head = head[:n]
	contentType, kind := content.SniffRemoteMedia(head)
	if contentType == "" {
		return nil, fmt.Errorf("proxy media %s: unsupported format: %w", m.MediaID, ErrNotFound)
	}
	limitPurpose := kernel.HTTPPurposeVideo
	if kind == content.MediaTypeImage {
		limitPurpose = kernel.HTTPPurposeImage
	}
	limits, _ := kernel.DefaultHTTPLimits(limitPurpose)

	length := int64(-1)
	if cl := resp.Header.Get("Content-Length"); cl != "" && resp.Header.Get("Content-Encoding") == "" {
		if v, err := strconv.ParseInt(cl, 10, 64); err == nil && v >= int64(n) {
			length = v
		}
	}
	if length > limits.MaxBodyBytes {
		return nil, fmt.Errorf("proxy media %s: %d bytes: %w: %w", m.MediaID, length, ErrNotFound, kernel.ErrResponseTooLarge)
	}
	body := &limitedBody{r: io.MultiReader(bytes.NewReader(head), resp.Body), c: resp.Body, left: limits.MaxBodyBytes}
	if length < 0 && kind == content.MediaTypeImage {
		// An image without a length is read whole (at most 16 MiB), so that the response has
		// one and a file over the limit is refused before anything is sent.
		data, err := io.ReadAll(body)
		_ = resp.Body.Close()
		switch {
		case errors.Is(err, kernel.ErrResponseTooLarge):
			return nil, fmt.Errorf("proxy media %s: %w: %w", m.MediaID, ErrNotFound, err)
		case err != nil:
			return nil, fmt.Errorf("proxy media %s: read: %w: %w", m.MediaID, ErrProxyUpstream, err)
		}
		ok = true
		return &ProxiedMedia{ContentType: contentType, Length: int64(len(data)), MaxAge: maxAge, Body: io.NopCloser(bytes.NewReader(data))}, nil
	}
	ok = true
	return &ProxiedMedia{ContentType: contentType, Length: length, MaxAge: maxAge, Body: body}, nil
}

// limitedBody reads at most left bytes and fails with kernel.ErrResponseTooLarge past them.
type limitedBody struct {
	r    io.Reader
	c    io.Closer
	left int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		// Probe for one more byte: a body of exactly the limit is not too large.
		var one [1]byte
		n, err := b.r.Read(one[:])
		if n > 0 {
			return 0, fmt.Errorf("media proxy: %w", kernel.ErrResponseTooLarge)
		}
		return 0, err
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.r.Read(p)
	b.left -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error { return b.c.Close() }
