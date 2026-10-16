// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"strings"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	"github.com/best-friends/my/internal/app/blob"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/media"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// Job types of the media files.
const (
	// JobProcessMedia processes one upload: payload {"media_id": "..."}.
	JobProcessMedia = "media.process"
	// JobCleanupMedia deletes the uploads that nothing attached in time (FR-803); no payload.
	JobCleanupMedia = "media.cleanup"
)

// Defaults of MediaDeps.
const (
	// DefaultUnattachedTTL is how long an upload may stay unattached before the cleanup deletes
	// it.
	DefaultUnattachedTTL = 24 * time.Hour
	// DefaultCleanupInterval is the period of the cleanup schedule.
	DefaultCleanupInterval = time.Hour
	// cleanupBatch is the number of uploads one cleanup query looks at; cleanupMaxBatches
	// bounds the batches of one run, and the next run continues.
	cleanupBatch      = 100
	cleanupMaxBatches = 10
)

// ErrContentTypeMismatch reports an upload whose declared media type differs from the type of
// its content (ADR-0019 §8).
var ErrContentTypeMismatch = errors.New("content: the declared content type does not match the content")

// MediaReferrer is a module that refers to media files from outside the content module's
// status revisions, such as the account module's avatars and headers. The cleanup keeps every
// file that a referrer reports as referenced.
type MediaReferrer interface {
	IsMediaReferenced(ctx context.Context, mediaID string) (bool, error)
}

// MediaDeps are the ports a MediaService uses.
type MediaDeps struct {
	UnitOfWork uow.UnitOfWork
	Repository MediaRepository
	Blobs      blob.Store
	Processor  media.Processor
	// Outbox enqueues the processing job in the upload's transaction. When it is nil (the
	// inline queue), Jobs enqueues it after the commit.
	Outbox queue.Outbox
	Jobs   queue.JobQueue
	Clock  kernel.Clock
	IDs    kernel.IDGenerator
	// Limits bound uploads; zero fields take media.DefaultLimits.
	Limits media.Limits
	// Sizes bound the variants; zero fields take media.DefaultSizes.
	Sizes media.Sizes
	// UnattachedTTL is how long an upload may stay unattached; zero means DefaultUnattachedTTL.
	UnattachedTTL time.Duration
	Referrers     []MediaReferrer
	// Logger logs what jobs survive; nil means slog.Default().
	Logger *slog.Logger
}

// MediaService runs the use cases of media files: uploads, their processing, focal points,
// and the cleanup of unattached uploads (FR-105, FR-803, ADR-0005, ADR-0019 §8).
//
// An upload is checked and stored synchronously: the content must be an accepted format
// detected from its bytes, within the size and dimension limits. The file as received stays in
// blob storage under a private key (uploads/<id>) until the media.process job has re-encoded
// it without metadata into public variants (media/<id>/full.<ext> and
// media/<id>/thumbnail.<ext>); then the job deletes it, so that no location data is kept.
type MediaService struct {
	d MediaDeps
}

// NewMediaService returns a MediaService on d.
func NewMediaService(d MediaDeps) *MediaService {
	d.Limits = d.Limits.WithDefaults()
	d.Sizes = d.Sizes.WithDefaults()
	if d.UnattachedTTL <= 0 {
		d.UnattachedTTL = DefaultUnattachedTTL
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &MediaService{d: d}
}

// Limits returns the upload limits in effect.
func (s *MediaService) Limits() media.Limits { return s.d.Limits }

// RegisterJobs registers the media job handlers on reg and the cleanup schedule on sched.
func (s *MediaService) RegisterJobs(reg *queue.JobHandlerRegistry, sched *queue.Scheduler) {
	reg.Register(JobProcessMedia, s.handleProcess)
	reg.Register(JobCleanupMedia, s.handleCleanup)
	sched.Add(queue.Schedule{Name: JobCleanupMedia, Every: DefaultCleanupInterval, Job: queue.NewJob{Type: JobCleanupMedia}})
}

// UploadInput describes an upload.
type UploadInput struct {
	OwnerAccountID string
	Content        io.Reader
	// DeclaredContentType is the media type the client sent, or "". It is never trusted: an
	// upload whose content is of another type fails with ErrContentTypeMismatch.
	DeclaredContentType string
	// Description is the alternative text, or "" for none.
	Description string
}

// Upload checks and stores an upload and enqueues its processing. It fails with an error
// wrapping media.ErrUnsupportedFormat, media.ErrTooLarge, media.ErrDimensions,
// media.ErrCorrupt, or ErrContentTypeMismatch for content it does not accept, and with
// content.ErrDescriptionTooLong for a description that is too long.
func (s *MediaService) Upload(ctx context.Context, in UploadInput) (Media, error) {
	if err := content.ValidateMediaDescription(in.Description); err != nil {
		return Media{}, err
	}
	data, err := io.ReadAll(io.LimitReader(in.Content, s.d.Limits.MaxBytes+1))
	if err != nil {
		return Media{}, fmt.Errorf("content: read upload: %w", err)
	}
	probe, err := s.d.Processor.Inspect(ctx, data, s.d.Limits)
	if err != nil {
		return Media{}, err
	}
	if err := checkDeclaredType(in.DeclaredContentType, probe.Format); err != nil {
		return Media{}, err
	}
	file := content.MediaFile{ID: s.d.IDs.NewID(), OwnerAccountID: in.OwnerAccountID, Origin: content.MediaOriginUpload, CreatedAt: s.d.Clock.Now()}
	up := content.UploadedMedia{
		MediaID: file.ID, BlobKey: uploadKey(file.ID), ContentType: probe.Format.ContentType(), FileSize: int64(len(data)),
	}
	if _, err := s.d.Blobs.Put(ctx, up.BlobKey, bytes.NewReader(data), blob.PutOptions{ContentType: up.ContentType}); err != nil {
		return Media{}, err
	}
	job := processJob(file.ID)
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if err := s.d.Repository.InsertMediaFile(ctx, tx, file); err != nil {
			return err
		}
		if err := s.d.Repository.InsertUploadedMedia(ctx, tx, up); err != nil {
			return err
		}
		if in.Description != "" {
			if err := s.d.Repository.UpsertDescription(ctx, tx, content.MediaDescription{MediaID: file.ID, Text: in.Description}); err != nil {
				return err
			}
		}
		if s.d.Outbox != nil {
			return s.d.Outbox.EnqueueTx(ctx, tx, job)
		}
		return nil
	})
	if err != nil {
		_ = s.d.Blobs.Delete(context.WithoutCancel(ctx), up.BlobKey)
		return Media{}, err
	}
	if s.d.Outbox == nil {
		if err := s.d.Jobs.Enqueue(ctx, job); err != nil {
			s.d.Logger.ErrorContext(ctx, "media processing not enqueued", slog.String("media_id", file.ID), slog.Any("error", err))
		}
	}
	return s.Media(ctx, file.ID)
}

func checkDeclaredType(declared string, detected media.Format) error {
	if declared == "" {
		return nil
	}
	mt, _, err := mime.ParseMediaType(declared)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrContentTypeMismatch, declared)
	}
	switch mt {
	case "application/octet-stream":
		return nil
	case "image/jpg", "image/pjpeg":
		mt = "image/jpeg"
	}
	if mt != detected.ContentType() {
		return fmt.Errorf("%w: declared %s, content is %s", ErrContentTypeMismatch, mt, detected.ContentType())
	}
	return nil
}

func uploadKey(mediaID string) string { return "uploads/" + mediaID }

func variantKey(mediaID string, kind content.MediaVariantKind, ext string) string {
	return blob.PublicPrefix + mediaID + "/" + string(kind) + "." + ext
}

type processPayload struct {
	MediaID string `json:"media_id"`
}

func processJob(mediaID string) queue.NewJob {
	payload, _ := json.Marshal(processPayload{MediaID: mediaID})
	return queue.NewJob{Type: JobProcessMedia, Payload: payload, IdempotencyKey: JobProcessMedia + ":" + mediaID}
}

// Media is a media file with its current state.
type Media struct {
	File content.MediaFile
	// Upload is the upload subtype, nil for a remote file.
	Upload *content.UploadedMedia
	// Result is the newest processing result, nil while the file is being processed.
	Result *content.MediaProcessingResult
	// Focus is the focal point, nil when none is set (the center).
	Focus *content.MediaFocus
	// Description is the alternative text that the uploader set, or "". An attachment of a
	// status revision has its own copy.
	Description string
}

// Ready reports whether the file was processed successfully and can be shown.
func (m Media) Ready() bool { return m.Result != nil && m.Result.Outcome == content.MediaReady }

// Media returns a media file with its current state.
func (s *MediaService) Media(ctx context.Context, id string) (Media, error) {
	file, err := s.d.Repository.FindMediaFile(ctx, id)
	if err != nil {
		return Media{}, err
	}
	m := Media{File: file}
	if up, err := s.d.Repository.FindUploadedMedia(ctx, id); err == nil {
		m.Upload = &up
	} else if !errors.Is(err, ErrNotFound) {
		return Media{}, err
	}
	if r, err := s.d.Repository.FindLatestProcessingResult(ctx, id); err == nil {
		m.Result = &r
	} else if !errors.Is(err, ErrNotFound) {
		return Media{}, err
	}
	if f, err := s.d.Repository.FindFocus(ctx, id); err == nil {
		m.Focus = &f
	} else if !errors.Is(err, ErrNotFound) {
		return Media{}, err
	}
	if d, err := s.d.Repository.FindDescription(ctx, id); err == nil {
		m.Description = d.Text
	} else if !errors.Is(err, ErrNotFound) {
		return Media{}, err
	}
	return m, nil
}

// OwnMedia returns a media file of accountID with its current state. A file of another
// account fails with ErrNotFound like an unknown one, so that nobody learns of the uploads of
// others: other accounts see a file only as an attachment of a status they may see.
func (s *MediaService) OwnMedia(ctx context.Context, accountID, id string) (Media, error) {
	m, err := s.Media(ctx, id)
	if err != nil {
		return Media{}, err
	}
	if m.File.OwnerAccountID != accountID {
		return Media{}, fmt.Errorf("%w: media file %s of another account", ErrNotFound, id)
	}
	return m, nil
}

// ErrMediaAttached reports a change to a media file that a status revision attaches: the
// revision has its own copy of the description, so the file can no longer change on its own.
var ErrMediaAttached = errors.New("content: the media file is attached to a status")

// MediaUpdate changes a media file; nil fields keep their value.
type MediaUpdate struct {
	MediaID string
	// Description is the new alternative text; "" removes it.
	Description *string
	// Focus is the new focal point; its MediaID is ignored.
	Focus *content.MediaFocus
}

// Update changes the description or focal point of an unattached media file of accountID and
// returns the file. It fails with content.ErrDescriptionTooLong or content.ErrInvalidFocus for
// invalid values, ErrNotFound for an unknown file or one of another account, and
// ErrMediaAttached for a file that a status revision attaches.
func (s *MediaService) Update(ctx context.Context, accountID string, u MediaUpdate) (Media, error) {
	if u.Description != nil {
		if err := content.ValidateMediaDescription(*u.Description); err != nil {
			return Media{}, err
		}
	}
	var focus content.MediaFocus
	if u.Focus != nil {
		focus = content.MediaFocus{MediaID: u.MediaID, X: u.Focus.X, Y: u.Focus.Y}
		if err := focus.Validate(); err != nil {
			return Media{}, err
		}
	}
	if _, err := s.OwnMedia(ctx, accountID, u.MediaID); err != nil {
		return Media{}, err
	}
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if attached, err := s.d.Repository.IsAttached(ctx, tx, u.MediaID); err != nil || attached {
			if err == nil {
				err = ErrMediaAttached
			}
			return err
		}
		switch {
		case u.Description == nil:
		case *u.Description == "":
			if err := s.d.Repository.DeleteDescription(ctx, tx, u.MediaID); err != nil {
				return err
			}
		default:
			if err := s.d.Repository.UpsertDescription(ctx, tx, content.MediaDescription{MediaID: u.MediaID, Text: *u.Description}); err != nil {
				return err
			}
		}
		if u.Focus != nil {
			return s.d.Repository.UpsertFocus(ctx, tx, focus)
		}
		return nil
	})
	if err != nil {
		return Media{}, err
	}
	return s.Media(ctx, u.MediaID)
}

// ErrNotAnImage reports a media file that is not a still image, or whose processing failed.
var ErrNotAnImage = errors.New("content: the media file is not a still image")

// CheckImage fails with ErrNotFound for an unknown media file, and with ErrNotAnImage for a
// file that is not a still image: a processed file of another media type, a file whose
// processing failed, or an upload being processed whose format is not an image. It implements
// instance.MediaFiles for the instance's preview image.
func (s *MediaService) CheckImage(ctx context.Context, id string) error {
	m, err := s.Media(ctx, id)
	if err != nil {
		return err
	}
	switch {
	case m.Result != nil && (!m.Ready() || m.Result.MediaType != content.MediaTypeImage):
		return fmt.Errorf("media %s: %w", id, ErrNotAnImage)
	case m.Result == nil && (m.Upload == nil || !strings.HasPrefix(m.Upload.ContentType, "image/")):
		return fmt.Errorf("media %s: %w", id, ErrNotAnImage)
	}
	return nil
}

// URL returns the public URL of a variant.
func (s *MediaService) URL(v content.MediaVariant) string { return s.d.Blobs.PublicURL(v.BlobKey) }

// ErrNotOwner reports a change to a media file of another account.
var ErrNotOwner = errors.New("content: the media file belongs to another account")

// CheckProfileImage reports whether accountID may use a media file as its avatar or header:
// the account uploaded it and its processing succeeded. It implements the account module's
// CheckProfileImage and fails with an error wrapping appaccount.ErrUnusableImage otherwise.
func (s *MediaService) CheckProfileImage(ctx context.Context, accountID, mediaID string) error {
	m, err := s.Media(ctx, mediaID)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("media %s: %w", mediaID, appaccount.ErrUnusableImage)
	}
	if err != nil {
		return err
	}
	if m.Upload == nil || m.File.OwnerAccountID != accountID || !m.Ready() {
		return fmt.Errorf("media %s: %w", mediaID, appaccount.ErrUnusableImage)
	}
	return nil
}

// SetFocus sets the focal point of a media file that accountID owns. It fails with
// content.ErrInvalidFocus for a point outside the image, ErrNotFound for an unknown file, and
// ErrNotOwner for a file of another account.
func (s *MediaService) SetFocus(ctx context.Context, accountID string, f content.MediaFocus) error {
	if err := f.Validate(); err != nil {
		return err
	}
	file, err := s.d.Repository.FindMediaFile(ctx, f.MediaID)
	if err != nil {
		return err
	}
	if file.OwnerAccountID != accountID {
		return ErrNotOwner
	}
	return s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.d.Repository.UpsertFocus(ctx, tx, f) })
}

func (s *MediaService) handleProcess(ctx context.Context, j queue.Job) queue.Result {
	var p processPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.MediaID == "" {
		return queue.Fail(fmt.Errorf("media.process: bad payload %s", j.Payload))
	}
	return s.Process(ctx, p.MediaID)
}

// Process processes an upload: it re-encodes the file into its variants, records the result,
// and deletes the file as received. It is idempotent: a file that has a result only has its
// received file deleted again, and a file that is gone (cleaned up) is done. Content the
// processor rejects records a failed result.
func (s *MediaService) Process(ctx context.Context, mediaID string) queue.Result {
	up, err := s.d.Repository.FindUploadedMedia(ctx, mediaID)
	if errors.Is(err, ErrNotFound) {
		return queue.Done()
	}
	if err != nil {
		return queue.Retry(0, err)
	}
	if _, err := s.d.Repository.FindLatestProcessingResult(ctx, mediaID); err == nil {
		return s.deleteReceived(ctx, up)
	} else if !errors.Is(err, ErrNotFound) {
		return queue.Retry(0, err)
	}

	result := content.MediaProcessingResult{ID: s.d.IDs.NewID(), MediaID: mediaID, MediaType: content.MediaTypeImage, ProcessedAt: s.d.Clock.Now()}
	out, err := s.processReceived(ctx, up)
	switch {
	case err == nil:
		result.Outcome = content.MediaReady
		result.Width, result.Height, result.Blurhash = out.Width, out.Height, out.Blurhash
		for _, v := range []struct {
			kind content.MediaVariantKind
			v    media.Variant
		}{{content.VariantFull, out.Full}, {content.VariantThumbnail, out.Thumbnail}} {
			key := variantKey(mediaID, v.kind, v.v.Extension)
			info, err := s.d.Blobs.Put(ctx, key, bytes.NewReader(v.v.Data), blob.PutOptions{ContentType: v.v.ContentType})
			if err != nil {
				return queue.Retry(0, err)
			}
			result.Variants = append(result.Variants, content.MediaVariant{
				ResultID: result.ID, Kind: v.kind, BlobKey: key, ContentType: v.v.ContentType, FileSize: info.Size,
				Width: v.v.Width, Height: v.v.Height,
			})
		}
	case media.Rejected(err) || errors.Is(err, blob.ErrNotFound):
		s.d.Logger.WarnContext(ctx, "media processing failed", slog.String("media_id", mediaID), slog.Any("error", err))
		result.Outcome, result.MediaType = content.MediaFailed, content.MediaTypeUnknown
	default:
		return queue.Retry(0, err)
	}
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		// A concurrent run of the same job may have recorded its result first.
		if done, err := s.d.Repository.HasProcessingResult(ctx, tx, mediaID); err != nil || done {
			return err
		}
		return s.d.Repository.InsertProcessingResult(ctx, tx, result)
	})
	if err != nil {
		return queue.Retry(0, err)
	}
	return s.deleteReceived(ctx, up)
}

func (s *MediaService) processReceived(ctx context.Context, up content.UploadedMedia) (media.Output, error) {
	r, _, err := s.d.Blobs.Get(ctx, up.BlobKey)
	if err != nil {
		return media.Output{}, err
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(io.LimitReader(r, s.d.Limits.MaxBytes+1))
	if err != nil {
		return media.Output{}, fmt.Errorf("content: read %s: %w", up.BlobKey, err)
	}
	return s.d.Processor.Process(ctx, data, s.d.Limits, s.d.Sizes)
}

// deleteReceived deletes the file as received once it has a result; a failed deletion is
// retried with the job.
func (s *MediaService) deleteReceived(ctx context.Context, up content.UploadedMedia) queue.Result {
	if err := s.d.Blobs.Delete(ctx, up.BlobKey); err != nil {
		return queue.Retry(0, err)
	}
	return queue.Done()
}

func (s *MediaService) handleCleanup(ctx context.Context, _ queue.Job) queue.Result {
	if _, err := s.CleanupUnattached(ctx); err != nil {
		return queue.Retry(0, err)
	}
	return queue.Done()
}

// CleanupUnattached deletes the uploads older than the unattached TTL that no status revision
// attaches and no referrer references, with their blobs, and returns how many it deleted. One
// run looks at a bounded number of uploads; the next run continues. A blob whose deletion
// fails after its rows are gone is logged and left behind.
func (s *MediaService) CleanupUnattached(ctx context.Context) (int, error) {
	before := s.d.Clock.Now().Add(-s.d.UnattachedTTL)
	deleted, after := 0, ""
	for range cleanupMaxBatches {
		ids, err := s.d.Repository.ListUnattachedUploads(ctx, after, before, cleanupBatch)
		if err != nil {
			return deleted, err
		}
		for _, id := range ids {
			ok, err := s.deleteUnattached(ctx, id)
			if err != nil {
				return deleted, err
			}
			if ok {
				deleted++
			}
		}
		if len(ids) < cleanupBatch {
			break
		}
		after = ids[len(ids)-1]
	}
	return deleted, nil
}

func (s *MediaService) deleteUnattached(ctx context.Context, id string) (bool, error) {
	for _, r := range s.d.Referrers {
		referenced, err := r.IsMediaReferenced(ctx, id)
		if err != nil || referenced {
			return false, err
		}
	}
	var keys []string
	if up, err := s.d.Repository.FindUploadedMedia(ctx, id); err == nil {
		keys = append(keys, up.BlobKey)
	} else if !errors.Is(err, ErrNotFound) {
		return false, err
	}
	results, err := s.d.Repository.ListProcessingResults(ctx, id)
	if err != nil {
		return false, err
	}
	for _, r := range results {
		for _, v := range r.Variants {
			keys = append(keys, v.BlobKey)
		}
	}
	var deleted bool
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		var err error
		deleted, err = s.d.Repository.DeleteUnattachedMedia(ctx, tx, id)
		return err
	})
	if err != nil || !deleted {
		return false, err
	}
	for _, k := range keys {
		if err := s.d.Blobs.Delete(ctx, k); err != nil {
			s.d.Logger.WarnContext(ctx, "blob of a deleted upload left behind", slog.String("key", k), slog.Any("error", err))
		}
	}
	return true, nil
}
