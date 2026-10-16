// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/app/media"
)

// MediaProcessorContract verifies an adapter of the MediaProcessor port (ADR-0019 §8): format
// detection from content alone, rejection of unsupported, spoofed, oversized, and
// decompression-bomb files, removal of metadata (EXIF and GPS, PNG text, GIF comments), the
// EXIF orientation, resizing within the variant sizes, the output formats, a valid blurhash,
// repeatable output, and cancellation.
func MediaProcessorContract(t *testing.T, p media.Processor) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, p media.Processor)
	}{
		{"DetectsFormatsFromContent", mediaDetectsFormats},
		{"RejectsUnsupportedContent", mediaRejectsUnsupported},
		{"RejectsOversizedFiles", mediaRejectsOversized},
		{"RejectsDimensionsFromHeadersBeforeDecoding", mediaRejectsDimensions},
		{"RejectsTooManyFrames", mediaRejectsTooManyFrames},
		{"StripsMetadata", mediaStripsMetadata},
		{"AppliesTheEXIFOrientation", mediaAppliesOrientation},
		{"ResizesWithinTheSizes", mediaResizes},
		{"WritesTheDocumentedFormats", mediaWritesFormats},
		{"ComputesABlurhash", mediaComputesBlurhash},
		{"GivesTheSameOutputForTheSameInput", mediaIsDeterministic},
		{"StopsOnCancelledContext", mediaStopsOnCancel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, p) })
	}
}

func mediaProcess(t *testing.T, p media.Processor, data []byte) media.Output {
	t.Helper()
	out, err := p.Process(t.Context(), data, media.Limits{}, media.Sizes{})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	return out
}

func mediaDetectsFormats(t *testing.T, p media.Processor) {
	for _, tc := range []struct {
		name   string
		data   []byte
		format media.Format
		w, h   int
		frames int
	}{
		{"jpeg", FixtureJPEG(40, 20, 1), media.FormatJPEG, 40, 20, 1},
		{"png", FixturePNG(30, 10, false), media.FormatPNG, 30, 10, 1},
		{"gif", FixtureGIF(16, 8, 3), media.FormatGIF, 16, 8, 3},
		{"webp lossy", FixtureWebPLossy, media.FormatWebP, 1, 1, 1},
		{"webp lossless", FixtureWebPLossless, media.FormatWebP, 1, 1, 1},
		{"webp alpha", FixtureWebPAlpha, media.FormatWebP, 1, 1, 1},
	} {
		probe, err := p.Inspect(t.Context(), tc.data, media.Limits{})
		if err != nil {
			t.Errorf("%s: Inspect: %v", tc.name, err)
			continue
		}
		want := media.Probe{Format: tc.format, Width: tc.w, Height: tc.h, Frames: tc.frames}
		if probe != want {
			t.Errorf("%s: Inspect = %+v, want %+v", tc.name, probe, want)
		}
	}
}

func mediaRejectsUnsupported(t *testing.T, p media.Processor) {
	for name, data := range map[string][]byte{
		"empty":         nil,
		"svg":           []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`),
		"svg with xml":  []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`),
		"html":          []byte("<!DOCTYPE html><script>alert(1)</script>"),
		"pdf":           []byte("%PDF-1.7\n"),
		"riff wave":     []byte("RIFF\x24\x00\x00\x00WAVEfmt "),
		"text":          []byte("just text"),
		"animated webp": FixtureWebPAnimated,
		// HTML after a JPEG signature: detected as JPEG, but it does not decode.
		"spoofed jpeg": []byte("\xFF\xD8\xFF<html><script>alert(1)</script></html>"),
		"spoofed png":  []byte("\x89PNG\r\n\x1A\n<script>alert(1)</script>"),
		"spoofed gif":  []byte("GIF89a<script>alert(1)</script>"),
		"truncated":    FixtureJPEG(40, 20, 1)[:200],
	} {
		if _, err := p.Inspect(t.Context(), data, media.Limits{}); !media.Rejected(err) {
			if _, err := p.Process(t.Context(), data, media.Limits{}, media.Sizes{}); !media.Rejected(err) {
				t.Errorf("%s: Inspect and Process accepted it (Process: %v)", name, err)
			}
		}
		if _, err := p.Process(t.Context(), data, media.Limits{}, media.Sizes{}); !media.Rejected(err) {
			t.Errorf("%s: Process = %v, want a rejection", name, err)
		}
	}
	if _, err := p.Inspect(t.Context(), []byte("<svg/>"), media.Limits{}); !errors.Is(err, media.ErrUnsupportedFormat) {
		t.Errorf("svg: %v, want ErrUnsupportedFormat", err)
	}
}

func mediaRejectsOversized(t *testing.T, p media.Processor) {
	data := FixturePNG(30, 10, false)
	limits := media.Limits{MaxBytes: int64(len(data) - 1)}
	if _, err := p.Inspect(t.Context(), data, limits); !errors.Is(err, media.ErrTooLarge) {
		t.Errorf("Inspect = %v, want ErrTooLarge", err)
	}
	if _, err := p.Process(t.Context(), data, limits, media.Sizes{}); !errors.Is(err, media.ErrTooLarge) {
		t.Errorf("Process = %v, want ErrTooLarge", err)
	}
	if _, err := p.Inspect(t.Context(), data, media.Limits{MaxBytes: int64(len(data))}); err != nil {
		t.Errorf("Inspect at the limit = %v", err)
	}
}

func mediaRejectsDimensions(t *testing.T, p media.Processor) {
	// A file of a few kilobytes whose header claims 4 gigapixels: decoding it would allocate them.
	bomb := FixturePNGHeader(65535, 65535)
	if _, err := p.Inspect(t.Context(), bomb, media.Limits{}); !errors.Is(err, media.ErrDimensions) {
		t.Errorf("bomb: Inspect = %v, want ErrDimensions", err)
	}
	if _, err := p.Process(t.Context(), bomb, media.Limits{}, media.Sizes{}); !errors.Is(err, media.ErrDimensions) {
		t.Errorf("bomb: Process = %v, want ErrDimensions", err)
	}
	// Within the default side limit but over the default pixel limit (8000 × 8000 = 64 MP).
	if _, err := p.Inspect(t.Context(), FixturePNGHeader(8000, 8000), media.Limits{}); !errors.Is(err, media.ErrDimensions) {
		t.Errorf("64 MP: Inspect = %v, want ErrDimensions", err)
	}
	img := FixtureJPEG(40, 20, 1)
	for name, limits := range map[string]media.Limits{
		"side":   {MaxSide: 39},
		"pixels": {MaxPixels: 799},
	} {
		if _, err := p.Inspect(t.Context(), img, limits); !errors.Is(err, media.ErrDimensions) {
			t.Errorf("%s: Inspect = %v, want ErrDimensions", name, err)
		}
	}
	if _, err := p.Inspect(t.Context(), img, media.Limits{MaxSide: 40, MaxPixels: 800}); err != nil {
		t.Errorf("at the limits: Inspect = %v", err)
	}
}

func mediaRejectsTooManyFrames(t *testing.T, p media.Processor) {
	data := FixtureGIF(16, 8, 3)
	if _, err := p.Inspect(t.Context(), data, media.Limits{MaxFrames: 2}); !errors.Is(err, media.ErrDimensions) {
		t.Errorf("frames: Inspect = %v, want ErrDimensions", err)
	}
	// The pixel limit covers all frames together: 3 × 128 pixels.
	if _, err := p.Inspect(t.Context(), data, media.Limits{MaxPixels: 3*128 - 1}); !errors.Is(err, media.ErrDimensions) {
		t.Errorf("frame pixels: Inspect = %v, want ErrDimensions", err)
	}
	if _, err := p.Inspect(t.Context(), data, media.Limits{MaxFrames: 3, MaxPixels: 3 * 128}); err != nil {
		t.Errorf("at the limits: Inspect = %v", err)
	}
}

func mediaStripsMetadata(t *testing.T, p media.Processor) {
	for name, data := range map[string][]byte{
		"jpeg": FixtureJPEG(64, 48, 1),
		"png":  FixturePNG(64, 48, false),
		"gif":  FixtureGIF(64, 48, 2),
	} {
		if !bytes.Contains(data, []byte(MetadataMarker)) {
			t.Fatalf("%s: the fixture has no metadata", name)
		}
		out := mediaProcess(t, p, data)
		for vname, v := range map[string]media.Variant{"full": out.Full, "thumbnail": out.Thumbnail} {
			for _, needle := range []string{MetadataMarker, "Exif\x00", "tEXt", "zTXt", "iTXt", "eXIf", "GPS", "http://ns.adobe.com/xap/"} {
				if bytes.Contains(v.Data, []byte(needle)) {
					t.Errorf("%s %s: contains %q", name, vname, needle)
				}
			}
			if v.ContentType == "image/jpeg" && hasJPEGSegment(v.Data, 0xE1) {
				t.Errorf("%s %s: has an APP1 segment", name, vname)
			}
		}
	}
}

// hasJPEGSegment reports whether a JPEG has a segment with the given marker before its scan.
func hasJPEGSegment(data []byte, marker byte) bool {
	for p := 2; p+4 <= len(data) && data[p] == 0xFF; {
		m := data[p+1]
		if m == 0xDA {
			return false
		}
		if m == marker {
			return true
		}
		p += 2 + int(data[p+2])<<8 + int(data[p+3])
	}
	return false
}

func decodeVariant(t *testing.T, v media.Variant) image.Image {
	t.Helper()
	var (
		img image.Image
		err error
	)
	switch v.ContentType {
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(v.Data))
	case "image/png":
		img, err = png.Decode(bytes.NewReader(v.Data))
	case "image/gif":
		img, err = gif.Decode(bytes.NewReader(v.Data))
	default:
		t.Fatalf("unexpected content type %q", v.ContentType)
	}
	if err != nil {
		t.Fatalf("decode %s: %v", v.ContentType, err)
	}
	if b := img.Bounds(); b.Dx() != v.Width || b.Dy() != v.Height {
		t.Fatalf("%s decodes to %v, variant says %d×%d", v.ContentType, b, v.Width, v.Height)
	}
	return img
}

// near reports whether two colors differ by at most 40 in every channel (lossy codecs).
func near(a color.Color, b color.NRGBA) bool {
	c := color.NRGBAModel.Convert(a).(color.NRGBA)
	d := func(x, y uint8) bool { return max(x, y)-min(x, y) <= 40 }
	return d(c.R, b.R) && d(c.G, b.G) && d(c.B, b.B)
}

func mediaAppliesOrientation(t *testing.T, p media.Processor) {
	// The source is 40 × 20: left half FixtureLeft, right half FixtureRight.
	for _, tc := range []struct {
		orientation    int
		w, h           int
		first, second  color.NRGBA
		firstAt, secAt image.Point
	}{
		{1, 40, 20, FixtureLeft, FixtureRight, image.Pt(5, 10), image.Pt(35, 10)},
		{2, 40, 20, FixtureRight, FixtureLeft, image.Pt(5, 10), image.Pt(35, 10)},
		{3, 40, 20, FixtureRight, FixtureLeft, image.Pt(5, 10), image.Pt(35, 10)},
		{6, 20, 40, FixtureLeft, FixtureRight, image.Pt(10, 5), image.Pt(10, 35)},
		{8, 20, 40, FixtureRight, FixtureLeft, image.Pt(10, 5), image.Pt(10, 35)},
	} {
		out := mediaProcess(t, p, FixtureJPEG(40, 20, tc.orientation))
		if out.Width != tc.w || out.Height != tc.h {
			t.Errorf("orientation %d: %d×%d, want %d×%d", tc.orientation, out.Width, out.Height, tc.w, tc.h)
			continue
		}
		img := decodeVariant(t, out.Full)
		if !near(img.At(tc.firstAt.X, tc.firstAt.Y), tc.first) || !near(img.At(tc.secAt.X, tc.secAt.Y), tc.second) {
			t.Errorf("orientation %d: colors at %v and %v are %v and %v", tc.orientation, tc.firstAt, tc.secAt,
				img.At(tc.firstAt.X, tc.firstAt.Y), img.At(tc.secAt.X, tc.secAt.Y))
		}
	}
}

func mediaResizes(t *testing.T, p media.Processor) {
	sizes := media.Sizes{FullMaxPixels: 100 * 75, ThumbnailMaxPixels: 40 * 30}
	out, err := p.Process(t.Context(), FixturePNG(400, 300, false), media.Limits{}, sizes)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out.Width != 400 || out.Height != 300 {
		t.Errorf("source size %d×%d, want 400×300", out.Width, out.Height)
	}
	check := func(name string, v media.Variant, maxPixels int64) {
		decodeVariant(t, v)
		if int64(v.Width)*int64(v.Height) > maxPixels {
			t.Errorf("%s: %d×%d exceeds %d pixels", name, v.Width, v.Height, maxPixels)
		}
		if ratio := float64(v.Width) / float64(v.Height); ratio < 1.25 || ratio > 1.42 {
			t.Errorf("%s: %d×%d does not keep the 4:3 aspect ratio", name, v.Width, v.Height)
		}
		if int64(v.Width)*int64(v.Height) < maxPixels*8/10 {
			t.Errorf("%s: %d×%d is much smaller than %d pixels", name, v.Width, v.Height, maxPixels)
		}
	}
	check("full", out.Full, sizes.FullMaxPixels)
	check("thumbnail", out.Thumbnail, sizes.ThumbnailMaxPixels)

	small := mediaProcess(t, p, FixturePNG(30, 10, false))
	if small.Full.Width != 30 || small.Full.Height != 10 || small.Thumbnail.Width != 30 || small.Thumbnail.Height != 10 {
		t.Errorf("a small image was resized: full %d×%d, thumbnail %d×%d", small.Full.Width, small.Full.Height, small.Thumbnail.Width, small.Thumbnail.Height)
	}
}

func mediaWritesFormats(t *testing.T, p media.Processor) {
	for _, tc := range []struct {
		name      string
		data      []byte
		full      string
		thumbnail string
	}{
		{"jpeg", FixtureJPEG(40, 20, 1), "image/jpeg", "image/jpeg"},
		{"png", FixturePNG(40, 20, false), "image/png", "image/jpeg"},
		{"png with alpha", FixturePNG(40, 20, true), "image/png", "image/png"},
		{"webp lossy", FixtureWebPLossy, "image/jpeg", "image/jpeg"},
		{"webp alpha", FixtureWebPAlpha, "image/png", "image/png"},
		{"gif", FixtureGIF(16, 8, 3), "image/gif", "image/jpeg"},
	} {
		out := mediaProcess(t, p, tc.data)
		if out.Full.ContentType != tc.full || out.Thumbnail.ContentType != tc.thumbnail {
			t.Errorf("%s: full %s, thumbnail %s; want %s, %s", tc.name, out.Full.ContentType, out.Thumbnail.ContentType, tc.full, tc.thumbnail)
			continue
		}
		ext := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/gif": "gif"}
		if out.Full.Extension != ext[tc.full] || out.Thumbnail.Extension != ext[tc.thumbnail] {
			t.Errorf("%s: extensions %q, %q", tc.name, out.Full.Extension, out.Thumbnail.Extension)
		}
		decodeVariant(t, out.Full)
		decodeVariant(t, out.Thumbnail)
	}
	g, err := gif.DecodeAll(bytes.NewReader(mediaProcess(t, p, FixtureGIF(16, 8, 3)).Full.Data))
	if err != nil || len(g.Image) != 3 {
		t.Errorf("gif: the full variant has %v frames (%v), want 3", len(g.Image), err)
	}
}

func mediaComputesBlurhash(t *testing.T, p media.Processor) {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%*+,-.:;=?@[]^_{|}~"
	h := mediaProcess(t, p, FixtureJPEG(64, 48, 1)).Blurhash
	if len(h) < 6 {
		t.Fatalf("Blurhash = %q", h)
	}
	size := strings.IndexByte(chars, h[0])
	nx, ny := size%9+1, size/9+1
	if want := 4 + 2*nx*ny; len(h) != want {
		t.Errorf("Blurhash %q has %d characters, want %d for %d×%d components", h, len(h), want, nx, ny)
	}
	for _, c := range h {
		if !strings.ContainsRune(chars, c) {
			t.Errorf("Blurhash %q contains %q", h, c)
		}
	}
}

func mediaIsDeterministic(t *testing.T, p media.Processor) {
	for name, data := range map[string][]byte{"jpeg": FixtureJPEG(64, 48, 6), "png": FixturePNG(64, 48, true), "gif": FixtureGIF(16, 8, 2)} {
		a, b := mediaProcess(t, p, data), mediaProcess(t, p, data)
		if !bytes.Equal(a.Full.Data, b.Full.Data) || !bytes.Equal(a.Thumbnail.Data, b.Thumbnail.Data) || a.Blurhash != b.Blurhash {
			t.Errorf("%s: two runs gave different output", name)
		}
	}
}

func mediaStopsOnCancel(t *testing.T, p media.Processor) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := p.Process(ctx, FixtureJPEG(40, 20, 1), media.Limits{}, media.Sizes{})
	if err == nil || media.Rejected(err) {
		t.Errorf("Process with a cancelled context = %v, want the context's error", err)
	}
}
