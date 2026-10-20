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
// "corrupt", and "processes" a file into still PNG variants, and a GIF also into moving GIF
// variants, whose content is derived from the input's digest and the purpose, so that the same
// input gives the same output.
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

// Preview implements media.Processor: the small still variant of Process.
func (p FakeMediaProcessor) Preview(ctx context.Context, data []byte, limits media.Limits, sizes media.Sizes, req media.Request) (media.Variant, error) {
	out, err := p.Process(ctx, data, limits, sizes, req)
	return out.Still.Small, err
}

// Process implements media.Processor. A GIF also gets moving variants, as an animation would.
func (p FakeMediaProcessor) Process(ctx context.Context, data []byte, limits media.Limits, _ media.Sizes, req media.Request) (media.Output, error) {
	probe, err := p.Inspect(ctx, data, limits)
	if err != nil {
		return media.Output{}, err
	}
	sum := sha256.Sum256(append([]byte(req.Purpose), data...))
	digest := hex.EncodeToString(sum[:8])
	variant := func(kind string, w, h int) media.Variant {
		return media.Variant{Data: fmt.Appendf(nil, "\x89PNG\r\n\x1A\n%s:%s", kind, digest), ContentType: "image/png", Extension: "png", Width: w, Height: h}
	}
	moving := func(kind string, w, h int) media.Variant {
		return media.Variant{Data: fmt.Appendf(nil, "GIF89a%s:%s", kind, digest), ContentType: "image/gif", Extension: "gif", Width: w, Height: h}
	}
	out := media.Output{
		Width: 4, Height: 3, Blurhash: "LEHV6nWB2yk8pyo0adR*.7kCMdnj",
		Still: media.Variants{Small: variant("small", 1, 1), Medium: variant("medium", 2, 1), Large: variant("large", 4, 3)},
	}
	if probe.Format == media.FormatGIF {
		out.Animated = &media.Variants{Small: moving("small", 1, 1), Medium: moving("medium", 2, 1), Large: moving("large", 4, 3)}
	}
	return out, nil
}
