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
	// HasProcessingResult reports whether a media file has a processing result.
	HasProcessingResult(ctx context.Context, tx uow.Tx, mediaID string) (bool, error)
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

	FindMediaFile(ctx context.Context, id string) (content.MediaFile, error)
	FindUploadedMedia(ctx context.Context, mediaID string) (content.UploadedMedia, error)
	// FindLatestProcessingResult returns the newest processing result of a media file with
	// its variants: the file's current state.
	FindLatestProcessingResult(ctx context.Context, mediaID string) (content.MediaProcessingResult, error)
	// ListProcessingResults returns every processing result of a media file with its
	// variants, oldest first.
	ListProcessingResults(ctx context.Context, mediaID string) ([]content.MediaProcessingResult, error)
	FindFocus(ctx context.Context, mediaID string) (content.MediaFocus, error)
	FindDescription(ctx context.Context, mediaID string) (content.MediaDescription, error)
	// ListUnattachedUploads returns the IDs of up to limit uploaded media files after afterID
	// ("" for the first page) that were created before createdBefore and that no status
	// revision attaches, ordered by ID.
	ListUnattachedUploads(ctx context.Context, afterID string, createdBefore time.Time, limit int) ([]string, error)
}
