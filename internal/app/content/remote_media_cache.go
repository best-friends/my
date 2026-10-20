// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/best-friends/my/internal/app/blob"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// RemoteMediaPolicy is the remote_media_policy instance setting (FR-409, ADR-0005): which
// remote media files the media proxy keeps a copy of.
type RemoteMediaPolicy string

// Values of RemoteMediaPolicy.
const (
	// RemoteMediaCacheNone keeps no copy: the proxy relays every request.
	RemoteMediaCacheNone RemoteMediaPolicy = "none"
	// RemoteMediaCacheAvatars keeps the avatars, headers, and custom emoji images.
	RemoteMediaCacheAvatars RemoteMediaPolicy = "avatars"
	// RemoteMediaCacheAll keeps every remote media file.
	RemoteMediaCacheAll RemoteMediaPolicy = "all"
)

// ProxySettings are the instance settings of the media proxy.
type ProxySettings struct {
	Policy RemoteMediaPolicy
	// CacheTTL is remote_media_cache_ttl: how long a copy is kept, and the max-age of the
	// proxy's responses.
	CacheTTL time.Duration
}

const (
	// JobCleanupRemoteMedia deletes the copies of the remote media cache past their expiry and
	// those over the size limit (FR-803); no payload.
	JobCleanupRemoteMedia = "media.cleanup_remote"
	// DefaultRemoteMediaCacheMaxSize is the default size limit of the remote media cache.
	DefaultRemoteMediaCacheMaxSize int64 = 1 << 30
	// remoteMediaPrefix starts the private blob keys of the cached copies.
	remoteMediaPrefix = "remote/"
	// cacheTouchInterval is the granularity of last_accessed_at: a copy served again within it
	// is not written again, so that serving a copy rarely writes to the database.
	cacheTouchInterval = time.Hour
)

// RegisterJobs registers the cleanup of the remote media cache on reg and its hourly schedule on
// sched.
func (p *MediaProxy) RegisterJobs(reg *queue.JobHandlerRegistry, sched *queue.Scheduler) {
	reg.Register(JobCleanupRemoteMedia, func(ctx context.Context, _ queue.Job) queue.Result {
		if _, err := p.CleanupCache(ctx); err != nil {
			return queue.Retry(0, err)
		}
		return queue.Done()
	})
	sched.Add(queue.Schedule{Name: JobCleanupRemoteMedia, Every: DefaultCleanupInterval, Job: queue.NewJob{Type: JobCleanupRemoteMedia}})
}

// cacheable reports whether the policy keeps a copy of m: every file under all, and under
// avatars the files that a profile uses as its avatar or header or a custom emoji as its image.
func (p *MediaProxy) cacheable(ctx context.Context, m content.RemoteMedia, policy RemoteMediaPolicy) (bool, error) {
	if p.d.Blobs == nil || p.d.UnitOfWork == nil || p.d.Clock == nil {
		return false, nil
	}
	switch policy {
	case RemoteMediaCacheAll:
		return true, nil
	case RemoteMediaCacheAvatars:
		if p.d.ProfileImages != nil {
			ok, err := p.d.ProfileImages.IsMediaReferenced(ctx, m.MediaID)
			if err != nil || ok {
				return ok, err
			}
		}
		return p.d.Repository.IsCustomEmojiImage(ctx, m.MediaID)
	default:
		return false, nil
	}
}

// remoteMediaKey is the blob key of the copy of a remote URL. It is derived from the URL, so
// that fetching the same URL again replaces the copy instead of leaving one behind.
func remoteMediaKey(remoteURL string) string {
	sum := sha256.Sum256([]byte(remoteURL))
	return remoteMediaPrefix + hex.EncodeToString(sum[:])
}

// openCached serves the copy of m while it has not expired, and otherwise fetches the file,
// keeps a copy for ttl, and serves the copy. The copy is written whole before anything is
// served, so that a file over the size limit or cut short is neither kept nor sent.
func (p *MediaProxy) openCached(ctx context.Context, m content.RemoteMedia, ttl time.Duration) (*ProxiedMedia, error) {
	if pm, err := p.openCopy(ctx, m.RemoteURL, ttl); pm != nil || err != nil {
		if err == nil {
			err = p.checkEmojiCopy(m, pm)
		}
		if err != nil {
			return nil, err
		}
		return pm, nil
	}
	f, err := p.fetch(ctx, m)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.body.Close() }()
	key := remoteMediaKey(m.RemoteURL)
	body := &errReader{r: f.body}
	info, err := p.d.Blobs.Put(ctx, key, body, blob.PutOptions{ContentType: f.contentType})
	if body.err != nil {
		return nil, readError(m, body.err)
	}
	if err != nil {
		return nil, fmt.Errorf("proxy media %s: keep a copy: %w", m.MediaID, err)
	}
	now := p.d.Clock.Now()
	e := content.RemoteMediaCacheEntry{
		URL: m.RemoteURL, BlobKey: key, ContentType: f.contentType, FileSize: info.Size,
		FetchedAt: now, ExpiresAt: now.Add(ttl), LastAccessedAt: now,
	}
	if err := p.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return p.d.Repository.UpsertRemoteMediaCache(ctx, tx, e) }); err != nil {
		// The blob stays without a row; the next fetch of the URL replaces it.
		return nil, fmt.Errorf("proxy media %s: record the copy: %w", m.MediaID, err)
	}
	rc, info, err := p.d.Blobs.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("proxy media %s: read the copy: %w", m.MediaID, err)
	}
	return &ProxiedMedia{ContentType: e.ContentType, Length: info.Size, MaxAge: ttl, Body: rc}, nil
}

// openCopy opens the copy of remoteURL, or returns nil when there is none to serve: no entry,
// an expired one, or one whose blob is gone (the cleanup deleted it, or a write was lost).
func (p *MediaProxy) openCopy(ctx context.Context, remoteURL string, ttl time.Duration) (*ProxiedMedia, error) {
	e, err := p.d.Repository.FindRemoteMediaCache(ctx, remoteURL)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	}
	now := p.d.Clock.Now()
	if !now.Before(e.ExpiresAt) {
		return nil, nil
	}
	rc, info, err := p.d.Blobs.Get(ctx, e.BlobKey)
	switch {
	case errors.Is(err, blob.ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	}
	if now.Sub(e.LastAccessedAt) >= cacheTouchInterval {
		err := p.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return p.d.Repository.TouchRemoteMediaCache(ctx, tx, e.URL, now) })
		if err != nil {
			// Serving the copy matters more than the order of the size limit's cleanup.
			p.d.Logger.WarnContext(ctx, "remote media cache access not recorded", slog.String("url", e.URL), slog.Any("error", err))
		}
	}
	return &ProxiedMedia{ContentType: e.ContentType, Length: info.Size, MaxAge: ttl, Body: rc}, nil
}

// errReader keeps the first error of r other than io.EOF, so that a failed blob write can be
// told apart from a failed read of the remote file whatever error the BlobStore returns.
type errReader struct {
	r   io.Reader
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) && e.err == nil {
		e.err = err
	}
	return n, err
}

// CleanupCache deletes the copies of the remote media cache that have expired, then the least
// recently accessed ones while the cache is over its size limit, and returns how many it
// deleted (FR-803). Each copy's blob is deleted before its row, so that a failure leaves a row
// that the next run deletes again rather than a blob that nothing refers to; a row fetched again
// since it was read is kept. One run looks at a bounded number of copies; the next run continues.
func (p *MediaProxy) CleanupCache(ctx context.Context) (int, error) {
	if p.d.Blobs == nil || p.d.UnitOfWork == nil || p.d.Clock == nil {
		return 0, nil
	}
	deleted := 0
	now := p.d.Clock.Now()
	for range cleanupMaxBatches {
		entries, err := p.d.Repository.ListExpiredRemoteMediaCache(ctx, now, cleanupBatch)
		if err != nil {
			return deleted, err
		}
		for _, e := range entries {
			ok, err := p.evict(ctx, e)
			if err != nil {
				return deleted, err
			}
			if ok {
				deleted++
			}
		}
		if len(entries) < cleanupBatch {
			break
		}
	}

	size, err := p.d.Repository.RemoteMediaCacheSize(ctx)
	if err != nil {
		return deleted, err
	}
	for batch := 0; size > p.d.MaxCacheSize && batch < cleanupMaxBatches; batch++ {
		entries, err := p.d.Repository.ListLeastRecentlyAccessedRemoteMediaCache(ctx, cleanupBatch)
		if err != nil {
			return deleted, err
		}
		if len(entries) == 0 {
			break
		}
		for _, e := range entries {
			if size <= p.d.MaxCacheSize {
				break
			}
			ok, err := p.evict(ctx, e)
			if err != nil {
				return deleted, err
			}
			if ok {
				size -= e.FileSize
				deleted++
			}
		}
	}
	return deleted, nil
}

// evict deletes the blob and then the row of a cache entry, and reports whether it deleted the
// row.
func (p *MediaProxy) evict(ctx context.Context, e content.RemoteMediaCacheEntry) (bool, error) {
	if err := p.d.Blobs.Delete(ctx, e.BlobKey); err != nil {
		return false, err
	}
	var deleted bool
	err := p.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		var err error
		deleted, err = p.d.Repository.DeleteRemoteMediaCache(ctx, tx, e.URL, e.FetchedAt)
		return err
	})
	return deleted, err
}
