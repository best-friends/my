// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/best-friends/my/internal/app/media"
)

// FakeMediaProcessor is a media.Processor for tests of the code around processing. It detects
// formats with media.Sniff, reports every file as 4 × 3 pixels, rejects files that contain
// "corrupt", and "processes" a file into PNG variants whose content is derived from the
// input's digest, so that the same input gives the same output.
type FakeMediaProcessor struct{}

var _ media.Processor = FakeMediaProcessor{}

// Inspect implements media.Processor.
func (FakeMediaProcessor) Inspect(ctx context.Context, data []byte, limits media.Limits) (media.Probe, error) {
	if err := ctx.Err(); err != nil {
		return media.Probe{}, err
	}
	limits = limits.WithDefaults()
	if int64(len(data)) > limits.MaxBytes {
		return media.Probe{}, media.ErrTooLarge
	}
	f := media.Sniff(data)
	if f == "" {
		return media.Probe{}, media.ErrUnsupportedFormat
	}
	if bytes.Contains(data, []byte("corrupt")) {
		return media.Probe{}, media.ErrCorrupt
	}
	return media.Probe{Format: f, Width: 4, Height: 3, Frames: 1}, nil
}

// Process implements media.Processor.
func (p FakeMediaProcessor) Process(ctx context.Context, data []byte, limits media.Limits, _ media.Sizes) (media.Output, error) {
	if _, err := p.Inspect(ctx, data, limits); err != nil {
		return media.Output{}, err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:8])
	variant := func(kind string, w, h int) media.Variant {
		return media.Variant{Data: fmt.Appendf(nil, "\x89PNG\r\n\x1A\n%s:%s", kind, digest), ContentType: "image/png", Extension: "png", Width: w, Height: h}
	}
	return media.Output{Width: 4, Height: 3, Full: variant("full", 4, 3), Thumbnail: variant("thumbnail", 2, 1), Blurhash: "LEHV6nWB2yk8pyo0adR*.7kCMdnj"}, nil
}
