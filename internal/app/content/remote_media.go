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

	"github.com/best-friends/my/internal/app/blob"
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
	// Settings returns the current remote_media_policy and remote_media_cache_ttl instance
	// settings.
	Settings func(ctx context.Context) (ProxySettings, error)
	// UnitOfWork, Blobs, and Clock keep the copies of the remote media cache; without Blobs the
	// proxy never caches, whatever the policy.
	UnitOfWork uow.UnitOfWork
	Blobs      blob.Store
	Clock      kernel.Clock
	// ProfileImages tells the avatars and headers of profiles, which the avatars policy caches
	// with the custom emoji images; nil caches only the custom emoji images under it.
	ProfileImages MediaReferrer
	// MaxCacheSize bounds the total size of the cache in bytes; zero means
	// DefaultRemoteMediaCacheMaxSize. The cleanup deletes the least recently accessed copies
	// over it.
	MaxCacheSize int64
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
// and audio (ADR-0019 §1). A CDN in front of the instance caches the responses for
// remote_media_cache_ttl. Under the remote_media_policy none the instance keeps no copy; under
// avatars and all it also keeps one in blob storage for remote_media_cache_ttl
// (remote_media_cache.go).
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
	if d.MaxCacheSize <= 0 {
		d.MaxCacheSize = DefaultRemoteMediaCacheMaxSize
	}
	d.BaseURL = strings.TrimSuffix(d.BaseURL, "/")
	return &MediaProxy{d: d}, nil
}

// ProxyVariant names what the media proxy serves of a remote media file.
type ProxyVariant string

// Proxy variants.
const (
	// ProxyFull is the remote file itself.
	ProxyFull ProxyVariant = "full"
)

// proxyVariants are the variants the proxy serves.
var proxyVariants = map[ProxyVariant]bool{ProxyFull: true}

// URL returns the signed proxy URL of a remote media file's variant.
func (p *MediaProxy) URL(mediaID string, variant ProxyVariant) string {
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
	if !proxyVariants[ProxyVariant(variant)] {
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
	settings, err := p.d.Settings(ctx)
	if err != nil {
		return nil, err
	}
	cache, err := p.cacheable(ctx, m, settings.Policy)
	if err != nil {
		return nil, err
	}
	if cache {
		return p.openCached(ctx, m, settings.CacheTTL)
	}
	return p.relay(ctx, m, settings.CacheTTL)
}

// relay fetches a remote media file and relays it without keeping a copy.
func (p *MediaProxy) relay(ctx context.Context, m content.RemoteMedia, maxAge time.Duration) (*ProxiedMedia, error) {
	f, err := p.fetch(ctx, m)
	if err != nil {
		return nil, err
	}
	if f.length < 0 && f.kind == content.MediaTypeImage {
		// An image without a length is read whole (at most 16 MiB), so that the response has
		// one and a file over the limit is refused before anything is sent.
		data, err := io.ReadAll(f.body)
		_ = f.body.Close()
		if err != nil {
			return nil, readError(m, err)
		}
		return &ProxiedMedia{ContentType: f.contentType, Length: int64(len(data)), MaxAge: maxAge, Body: io.NopCloser(bytes.NewReader(data))}, nil
	}
	return &ProxiedMedia{ContentType: f.contentType, Length: f.length, MaxAge: maxAge, Body: f.body}, nil
}

// readError classifies an error reading the body of a remote media file: a file over the size
// limit is not found, and anything else is an upstream failure.
func readError(m content.RemoteMedia, err error) error {
	if errors.Is(err, kernel.ErrResponseTooLarge) {
		return fmt.Errorf("proxy media %s: %w: %w", m.MediaID, ErrNotFound, err)
	}
	return fmt.Errorf("proxy media %s: read: %w: %w", m.MediaID, ErrProxyUpstream, err)
}

// fetchedMedia is the response of a remote media file in an allowed format.
type fetchedMedia struct {
	contentType string
	kind        content.MediaType
	// length is the declared length of the body, or -1.
	length int64
	// body reads the whole file and fails past the size limit of its kind; the caller closes
	// it.
	body *limitedBody
}

// fetch requests a remote media file and checks its status, format, and declared length.
func (p *MediaProxy) fetch(ctx context.Context, m content.RemoteMedia) (*fetchedMedia, error) {
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
	ok = true
	return &fetchedMedia{
		contentType: contentType, kind: kind, length: length,
		body: &limitedBody{r: io.MultiReader(bytes.NewReader(head), resp.Body), c: resp.Body, left: limits.MaxBodyBytes},
	}, nil
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
