// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"errors"
	"fmt"
	"math"
	"time"
	"unicode/utf8"
)

// MediaOrigin tells where a media file comes from (media_files.origin).
type MediaOrigin string

// Media origins.
const (
	MediaOriginUpload MediaOrigin = "upload"
	MediaOriginRemote MediaOrigin = "remote"
)

// MediaFile is a media file: an upload of a local account or a remote file (data-model §4.4).
type MediaFile struct {
	ID             string
	OwnerAccountID string
	Origin         MediaOrigin
	CreatedAt      time.Time
}

// UploadedMedia is the upload subtype of a media file: the file as received, kept in blob
// storage under BlobKey until processing ends. ContentType is the type detected from the
// content, never the one the client declared.
type UploadedMedia struct {
	MediaID     string
	BlobKey     string
	ContentType string
	FileSize    int64
}

// MediaOutcome is the outcome of processing a media file.
type MediaOutcome string

// Media outcomes.
const (
	MediaReady  MediaOutcome = "ready"
	MediaFailed MediaOutcome = "failed"
)

// MediaType is the kind of a media file, as the client API reports it.
type MediaType string

// Media types.
const (
	MediaTypeImage   MediaType = "image"
	MediaTypeGIFV    MediaType = "gifv"
	MediaTypeVideo   MediaType = "video"
	MediaTypeAudio   MediaType = "audio"
	MediaTypeUnknown MediaType = "unknown"
)

// MediaVariantKind names a processed rendition of a media file.
type MediaVariantKind string

// Media variant kinds.
const (
	// VariantFull is the file served as the media itself: re-encoded without metadata and
	// scaled down to the server's limit.
	VariantFull MediaVariantKind = "full"
	// VariantThumbnail is the preview.
	VariantThumbnail MediaVariantKind = "thumbnail"
)

// MediaProcessingResult records one processing of a media file. The newest result of a file is
// its current state. Width, Height, and Blurhash are zero when unknown (always for a failure);
// DurationMS is zero for still images.
type MediaProcessingResult struct {
	ID          string
	MediaID     string
	Outcome     MediaOutcome
	MediaType   MediaType
	Width       int
	Height      int
	DurationMS  int
	Blurhash    string
	ProcessedAt time.Time
	// Variants are the renditions in blob storage, ordered by Kind; none for a failure.
	Variants []MediaVariant
}

// Variant returns the variant of kind k.
func (r MediaProcessingResult) Variant(k MediaVariantKind) (MediaVariant, bool) {
	for _, v := range r.Variants {
		if v.Kind == k {
			return v, true
		}
	}
	return MediaVariant{}, false
}

// MediaVariant is a processed rendition of a media file in blob storage.
type MediaVariant struct {
	ResultID    string
	Kind        MediaVariantKind
	BlobKey     string
	ContentType string
	FileSize    int64
	Width       int
	Height      int
}

// MediaFocus is the focal point of a media file for cropping previews, from -1.0 (left,
// bottom) to 1.0 (right, top) on each axis; the center is (0, 0).
type MediaFocus struct {
	MediaID string
	X, Y    float64
}

// ErrInvalidFocus reports a focal point outside -1.0 to 1.0.
var ErrInvalidFocus = errors.New("content: focus must be between -1.0 and 1.0")

// Validate checks that the focal point lies within the image.
func (f MediaFocus) Validate() error {
	for _, v := range []float64{f.X, f.Y} {
		if math.IsNaN(v) || v < -1 || v > 1 {
			return fmt.Errorf("%w: (%v, %v)", ErrInvalidFocus, f.X, f.Y)
		}
	}
	return nil
}

// MaxMediaDescriptionLength is the most characters (Unicode code points) of a media file's
// description.
const MaxMediaDescriptionLength = 1500

// ErrDescriptionTooLong reports a media description longer than MaxMediaDescriptionLength.
var ErrDescriptionTooLong = errors.New("content: the media description is too long")

// MediaDescription is the description (alternative text) of an upload that the uploader sets
// before a post attaches it; the post copies it into its revision's attachment. Text is never
// empty: removing the description deletes it.
type MediaDescription struct {
	MediaID string
	Text    string
}

// ValidateMediaDescription checks the length of a media description.
func ValidateMediaDescription(text string) error {
	if n := utf8.RuneCountInString(text); n > MaxMediaDescriptionLength {
		return fmt.Errorf("%w: %d characters", ErrDescriptionTooLong, n)
	}
	return nil
}
