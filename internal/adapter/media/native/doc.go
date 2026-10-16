// SPDX-License-Identifier: AGPL-3.0-only

// Package native implements the MediaProcessor port (internal/app/media) in pure Go, so that
// the server keeps building with CGO_ENABLED=0 (ADR-0005). It passes
// testkit.MediaProcessorContract.
//
// JPEG files have their EXIF orientation applied to the pixels before re-encoding. ICC color
// profiles are dropped with the rest of the metadata, so wide-gamut images are shown as sRGB.
package native
