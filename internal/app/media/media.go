// SPDX-License-Identifier: AGPL-3.0-only

// Package media defines the MediaProcessor port (ADR-0005, ADR-0019 §8): format detection
// from content, size and dimension limits, metadata removal by re-encoding, resizing,
// thumbnails, and blurhash. The adapters live in internal/adapter/media/* and pass
// testkit.MediaProcessorContract.
//
// The formats of v0 are still images in JPEG, PNG, and WebP, and GIF (animated or not).
// Outputs are JPEG for JPEG input and for opaque WebP, PNG for PNG input and WebP with
// transparency, and GIF for GIF input (every frame kept, without resizing); thumbnails are
// JPEG when opaque and PNG otherwise, and a GIF's thumbnail shows its first frame. Pure-Go
// encoders cannot write WebP, so WebP is never an output.
package media

import (
	"bytes"
	"context"
	"errors"
)

var (
	// ErrUnsupportedFormat reports content that is not one of the accepted formats, whatever
	// its file name or declared type (SVG and HTML included).
	ErrUnsupportedFormat = errors.New("media: unsupported format")
	// ErrTooLarge reports content longer than Limits.MaxBytes.
	ErrTooLarge = errors.New("media: file too large")
	// ErrDimensions reports an image wider or taller than Limits.MaxSide, with more pixels than
	// Limits.MaxPixels, or with more frames than Limits.MaxFrames.
	ErrDimensions = errors.New("media: image dimensions exceed the limits")
	// ErrCorrupt reports content of an accepted format that does not decode.
	ErrCorrupt = errors.New("media: corrupt image")
)

// Format is a detected file format.
type Format string

// Formats.
const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatGIF  Format = "gif"
	FormatWebP Format = "webp"
)

// ContentType returns the media type of f, or "" for an unknown format.
func (f Format) ContentType() string {
	switch f {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatGIF:
		return "image/gif"
	case FormatWebP:
		return "image/webp"
	}
	return ""
}

// Sniff detects the format of data from its leading bytes alone, or returns "" when data is in
// none of the accepted formats. File names and declared types are never consulted.
func Sniff(data []byte) Format {
	switch {
	case bytes.HasPrefix(data, []byte("\xFF\xD8\xFF")):
		return FormatJPEG
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1A\n")):
		return FormatPNG
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return FormatGIF
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP":
		return FormatWebP
	}
	return ""
}

// Limits bound what a processor accepts. Processors check the dimensions from the file's
// headers before decoding its pixels, so that a small file that expands to a huge image
// (a decompression bomb) is rejected without allocating it.
type Limits struct {
	// MaxBytes is the longest accepted content.
	MaxBytes int64
	// MaxSide is the largest accepted width and height in pixels.
	MaxSide int
	// MaxPixels is the largest accepted width × height. For an animation it also bounds the
	// pixels of all frames together.
	MaxPixels int64
	// MaxFrames is the largest accepted number of frames of an animation.
	MaxFrames int
}

// Default limits (ADR-0019 §8).
const (
	DefaultMaxBytes  = 16 << 20
	DefaultMaxSide   = 8192
	DefaultMaxPixels = 7680 * 4320 // 33 MP, 8K UHD
	DefaultMaxFrames = 500
)

// DefaultLimits returns the limits of ADR-0019 §8.
func DefaultLimits() Limits {
	return Limits{MaxBytes: DefaultMaxBytes, MaxSide: DefaultMaxSide, MaxPixels: DefaultMaxPixels, MaxFrames: DefaultMaxFrames}
}

// WithDefaults returns l with its zero or negative fields set to the defaults.
func (l Limits) WithDefaults() Limits {
	d := DefaultLimits()
	if l.MaxBytes <= 0 {
		l.MaxBytes = d.MaxBytes
	}
	if l.MaxSide <= 0 {
		l.MaxSide = d.MaxSide
	}
	if l.MaxPixels <= 0 {
		l.MaxPixels = d.MaxPixels
	}
	if l.MaxFrames <= 0 {
		l.MaxFrames = d.MaxFrames
	}
	return l
}

// Probe is what Inspect learns from a file's headers.
type Probe struct {
	Format Format
	// Width and Height are the stored dimensions, before any EXIF orientation.
	Width, Height int
	// Frames is the number of frames: 1 for a still image.
	Frames int
}

// Sizes bound the processed variants by their area, keeping the aspect ratio. A source that
// already fits is not enlarged.
type Sizes struct {
	// FullMaxPixels bounds the variant that is served as the media itself.
	FullMaxPixels int64
	// ThumbnailMaxPixels bounds the preview.
	ThumbnailMaxPixels int64
}

// Default variant sizes: 3840 × 2160 for the full image and 640 × 360 for the thumbnail, the
// areas Mastodon uses.
const (
	DefaultFullMaxPixels      = 3840 * 2160
	DefaultThumbnailMaxPixels = 640 * 360
)

// DefaultSizes returns the default variant sizes.
func DefaultSizes() Sizes {
	return Sizes{FullMaxPixels: DefaultFullMaxPixels, ThumbnailMaxPixels: DefaultThumbnailMaxPixels}
}

// WithDefaults returns s with its zero or negative fields set to the defaults.
func (s Sizes) WithDefaults() Sizes {
	if s.FullMaxPixels <= 0 {
		s.FullMaxPixels = DefaultFullMaxPixels
	}
	if s.ThumbnailMaxPixels <= 0 {
		s.ThumbnailMaxPixels = DefaultThumbnailMaxPixels
	}
	return s
}

// Variant is one processed rendition of a file.
type Variant struct {
	Data        []byte
	ContentType string
	// Extension is the file name extension for ContentType, without the dot ("jpg").
	Extension     string
	Width, Height int
}

// Output is the result of processing a file.
type Output struct {
	// Width and Height are the dimensions of the source after its EXIF orientation.
	Width, Height int
	// Full is the re-encoded file without metadata, at most Sizes.FullMaxPixels.
	Full Variant
	// Thumbnail is the preview, at most Sizes.ThumbnailMaxPixels.
	Thumbnail Variant
	// Blurhash is the blurhash of the thumbnail (https://blurha.sh).
	Blurhash string
}

// Processor is the MediaProcessor port. Zero fields of Limits and Sizes take their defaults
// (WithDefaults). Implementations bound the time (through ctx) and the
// number of concurrent decodes, and turn a panic of a decoder into ErrCorrupt.
type Processor interface {
	// Inspect detects the format of data from its content and reads its dimensions from its
	// headers, without decoding the pixels. It fails with an error wrapping
	// ErrUnsupportedFormat, ErrTooLarge, ErrDimensions, or ErrCorrupt when data breaks limits.
	Inspect(ctx context.Context, data []byte, limits Limits) (Probe, error)
	// Process inspects data as Inspect does, then decodes it, applies its EXIF orientation,
	// and re-encodes it without metadata into the variants. The same input gives the same
	// output, so that processing is safe to repeat.
	Process(ctx context.Context, data []byte, limits Limits, sizes Sizes) (Output, error)
}

// Rejected reports whether err means the content is not acceptable (as opposed to a failure
// such as a cancelled context, after which a retry may succeed).
func Rejected(err error) bool {
	return errors.Is(err, ErrUnsupportedFormat) || errors.Is(err, ErrTooLarge) ||
		errors.Is(err, ErrDimensions) || errors.Is(err, ErrCorrupt)
}
