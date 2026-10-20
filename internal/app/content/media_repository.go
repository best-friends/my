// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"context"
	"time"

	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// MediaRepository is the persistence port of the content module's media files (data-model
// §4.4). The database adapters implement it and pass testkit.MediaRepositoryContract.
//
// Methods that take a uow.Tx run inside that transaction; the others read committed data outside
// any transaction. Lookups that find nothing return an error wrapping ErrNotFound.
type MediaRepository interface {
	// InsertMediaFile inserts a media file.
	InsertMediaFile(ctx context.Context, tx uow.Tx, m content.MediaFile) error
	// InsertUploadedMedia inserts the upload subtype of a media file.
	InsertUploadedMedia(ctx context.Context, tx uow.Tx, u content.UploadedMedia) error
	// InsertProcessingResult inserts a processing result with its variants.
	InsertProcessingResult(ctx context.Context, tx uow.Tx, r content.MediaProcessingResult) error
	// HasProcessingResult reports whether the newest processing result of a media file is for
	// purpose.
	HasProcessingResult(ctx context.Context, tx uow.Tx, mediaID string, purpose content.MediaPurpose) (bool, error)
	// UpsertFocus sets the focal point of a media file.
	UpsertFocus(ctx context.Context, tx uow.Tx, f content.MediaFocus) error
	// UpsertDescription sets the description of a media file.
	UpsertDescription(ctx context.Context, tx uow.Tx, d content.MediaDescription) error
	// DeleteDescription removes the description of a media file; a file without one is not
	// an error.
	DeleteDescription(ctx context.Context, tx uow.Tx, mediaID string) error
	// IsAttached reports whether a status revision attaches a media file.
	IsAttached(ctx context.Context, tx uow.Tx, mediaID string) (bool, error)
	// DeleteUnattachedMedia deletes a media file with its upload, processing results,
	// variants, focus, and description, unless a status revision attaches it. It reports whether it deleted
	// the file; a missing file is not deleted and not an error.
	DeleteUnattachedMedia(ctx context.Context, tx uow.Tx, mediaID string) (bool, error)

	// InsertRemoteMedia inserts the remote subtype of a media file. It reports false, and
	// inserts nothing, when the owner already has a remote media file with the same URL.
	InsertRemoteMedia(ctx context.Context, tx uow.Tx, m content.RemoteMedia) (bool, error)
	// RemoteMediaByURL returns the remote media file of an owner with a URL.
	RemoteMediaByURL(ctx context.Context, tx uow.Tx, ownerAccountID, remoteURL string) (content.RemoteMedia, error)

	// UpsertRemoteMediaCache records a copy of a remote file, replacing the entry of its URL.
	UpsertRemoteMediaCache(ctx context.Context, tx uow.Tx, e content.RemoteMediaCacheEntry) error
	// TouchRemoteMediaCache sets when the entry of url was last accessed; a missing entry is
	// not an error.
	TouchRemoteMediaCache(ctx context.Context, tx uow.Tx, url string, at time.Time) error
	// DeleteRemoteMediaCache deletes the entry of url if it is still the copy fetched at
	// fetchedAt, and reports whether it deleted it: an entry fetched again since is kept.
	DeleteRemoteMediaCache(ctx context.Context, tx uow.Tx, url string, fetchedAt time.Time) (bool, error)
	// FindRemoteMediaCache returns the cache entry of a remote URL.
	FindRemoteMediaCache(ctx context.Context, url string) (content.RemoteMediaCacheEntry, error)
	// ListExpiredRemoteMediaCache returns up to limit entries that expire at or before now,
	// the earliest expiry first.
	ListExpiredRemoteMediaCache(ctx context.Context, now time.Time, limit int) ([]content.RemoteMediaCacheEntry, error)
	// ListLeastRecentlyAccessedRemoteMediaCache returns up to limit entries, the least recently
	// accessed first.
	ListLeastRecentlyAccessedRemoteMediaCache(ctx context.Context, limit int) ([]content.RemoteMediaCacheEntry, error)
	// RemoteMediaCacheSize returns the total file size of the cache entries in bytes.
	RemoteMediaCacheSize(ctx context.Context) (int64, error)
	// LocalMediaSize returns the bytes that the local uploads keep, in bytes: the variants of
	// the current processing result of each upload, and the originals that wait for their
	// first processing.
	LocalMediaSize(ctx context.Context) (int64, error)
	// IsCustomEmojiImage reports whether any revision of a custom emoji uses a media file as
	// its image.
	IsCustomEmojiImage(ctx context.Context, mediaID string) (bool, error)
	// ListCurrentCustomEmojis returns each custom emoji with an ID in ids with its current
	// image, ordered by ID; emoji without an image and unknown IDs are left out. It reads every
	// emoji in one query, for the emoji of a page of statuses and accounts.
	ListCurrentCustomEmojis(ctx context.Context, ids []string) ([]content.CurrentCustomEmoji, error)

	FindMediaFile(ctx context.Context, id string) (content.MediaFile, error)
	// FindRemoteMedia returns the remote subtype of a media file; an upload is not found.
	FindRemoteMedia(ctx context.Context, mediaID string) (content.RemoteMedia, error)
	// ListRemoteMedia returns the remote subtype of each media file with an ID in ids, ordered
	// by ID; uploads and unknown IDs are left out. It reads every file in one query, for the
	// images of a page of accounts.
	ListRemoteMedia(ctx context.Context, ids []string) ([]content.RemoteMedia, error)
	FindUploadedMedia(ctx context.Context, mediaID string) (content.UploadedMedia, error)
	// FindLatestProcessingResult returns the newest processing result of a media file with
	// its variants: the file's current state.
	FindLatestProcessingResult(ctx context.Context, mediaID string) (content.MediaProcessingResult, error)
	// ListProcessingResults returns every processing result of a media file with its
	// variants, oldest first.
	ListProcessingResults(ctx context.Context, mediaID string) ([]content.MediaProcessingResult, error)
	// ListReadyVariants returns the variants of the current processing result of each media file
	// with an ID in ids, with the file, ordered by file ID, then by size and motion. A file that
	// is missing, still being processed, or whose current result failed is left out. It reads
	// every file in one query, for the images of a page of accounts.
	ListReadyVariants(ctx context.Context, ids []string) ([]FileVariant, error)
	FindFocus(ctx context.Context, mediaID string) (content.MediaFocus, error)
	FindDescription(ctx context.Context, mediaID string) (content.MediaDescription, error)
	// ListUnattachedUploads returns the IDs of up to limit uploaded media files after afterID
	// ("" for the first page) that were created before createdBefore and that no status
	// revision attaches, ordered by ID.
	ListUnattachedUploads(ctx context.Context, afterID string, createdBefore time.Time, limit int) ([]string, error)
	// ListOwnerUploads returns up to limit uploaded media files of an owner after afterID (""
	// for the first page), ordered by ID, with whether a status revision attaches each.
	ListOwnerUploads(ctx context.Context, ownerAccountID, afterID string, limit int) ([]OwnedUpload, error)
}

// OwnedUpload is an uploaded media file of an owner, with whether a status revision attaches it.
type OwnedUpload struct {
	MediaID  string
	Attached bool
}

// FileVariant is a variant of a media file's current processing result, with the file.
type FileVariant struct {
	File    content.MediaFile
	Variant content.MediaVariant
}
