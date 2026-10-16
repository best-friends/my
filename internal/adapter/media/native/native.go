// SPDX-License-Identifier: AGPL-3.0-only

package native

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"runtime"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"github.com/best-friends/my/internal/app/media"
)

// jpegQuality is the quality of every JPEG the processor writes.
const jpegQuality = 85

// Options configure a Processor.
type Options struct {
	// Concurrency is the number of files processed at the same time; the default is half the
	// CPUs, at least 1. Decoding a large image takes hundreds of megabytes, so this bounds the
	// memory too.
	Concurrency int
}

// Processor implements media.Processor in pure Go with the standard library's JPEG, PNG,
// and GIF codecs and golang.org/x/image for WebP decoding and resampling.
type Processor struct {
	sem chan struct{}
}

var _ media.Processor = (*Processor)(nil)

// New returns a Processor.
func New(o Options) *Processor {
	if o.Concurrency <= 0 {
		o.Concurrency = max(1, runtime.GOMAXPROCS(0)/2)
	}
	return &Processor{sem: make(chan struct{}, o.Concurrency)}
}

// Inspect implements media.Processor.
func (p *Processor) Inspect(ctx context.Context, data []byte, limits media.Limits) (probe media.Probe, err error) {
	if err := ctx.Err(); err != nil {
		return media.Probe{}, err
	}
	defer func() {
		if r := recover(); r != nil {
			probe, err = media.Probe{}, fmt.Errorf("%w: decoder panicked: %v", media.ErrCorrupt, r)
		}
	}()
	return inspect(data, limits.WithDefaults())
}

// Process implements media.Processor. It waits for a free slot of Options.Concurrency, then
// decodes in a goroutine; when ctx ends first, it returns ctx's error at once and the decode
// finishes in the background, still holding its slot.
func (p *Processor) Process(ctx context.Context, data []byte, limits media.Limits, sizes media.Sizes) (media.Output, error) {
	limits, sizes = limits.WithDefaults(), sizes.WithDefaults()
	probe, err := p.Inspect(ctx, data, limits)
	if err != nil {
		return media.Output{}, err
	}
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return media.Output{}, ctx.Err()
	}
	type result struct {
		out media.Output
		err error
	}
	done := make(chan result, 1)
	go func() {
		defer func() { <-p.sem }()
		defer func() {
			if r := recover(); r != nil {
				done <- result{err: fmt.Errorf("%w: decoder panicked: %v", media.ErrCorrupt, r)}
			}
		}()
		out, err := process(data, probe, sizes)
		done <- result{out: out, err: err}
	}()
	select {
	case r := <-done:
		return r.out, r.err
	case <-ctx.Done():
		return media.Output{}, ctx.Err()
	}
}

func process(data []byte, probe media.Probe, sizes media.Sizes) (media.Output, error) {
	var (
		img  *image.NRGBA
		full media.Variant
		err  error
	)
	switch probe.Format {
	case media.FormatGIF:
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return media.Output{}, fmt.Errorf("%w: %w", media.ErrCorrupt, err)
		}
		// Re-encoding keeps the frames, their delays, and the loop count, and drops comments
		// and application extensions (XMP). Animations are not resized.
		var buf bytes.Buffer
		if err := gif.EncodeAll(&buf, g); err != nil {
			return media.Output{}, fmt.Errorf("media/native: encode gif: %w", err)
		}
		full = media.Variant{Data: buf.Bytes(), ContentType: "image/gif", Extension: "gif", Width: g.Config.Width, Height: g.Config.Height}
		img = firstFrame(g)
	default:
		src, err := decodeStill(data, probe.Format)
		if err != nil {
			return media.Output{}, err
		}
		img = toNRGBA(src)
		if probe.Format == media.FormatJPEG {
			img = orient(img, exifOrientation(data))
		}
	}
	out := media.Output{Width: img.Rect.Dx(), Height: img.Rect.Dy()}
	if probe.Format != media.FormatGIF {
		scaled := fit(img, sizes.FullMaxPixels)
		asPNG := probe.Format == media.FormatPNG || probe.Format == media.FormatWebP && !scaled.Opaque()
		if full, err = encode(scaled, asPNG); err != nil {
			return media.Output{}, err
		}
		img = scaled
	}
	thumb := fit(img, sizes.ThumbnailMaxPixels)
	if out.Thumbnail, err = encode(thumb, !thumb.Opaque()); err != nil {
		return media.Output{}, err
	}
	out.Full = full
	out.Blurhash = blurhash(thumb, blurhashX, blurhashY)
	return out, nil
}

func decodeStill(data []byte, f media.Format) (image.Image, error) {
	var (
		img image.Image
		err error
	)
	r := bytes.NewReader(data)
	switch f {
	case media.FormatJPEG:
		img, err = jpeg.Decode(r)
	case media.FormatPNG:
		img, err = png.Decode(r)
	case media.FormatWebP:
		img, err = webp.Decode(r)
	default:
		return nil, media.ErrUnsupportedFormat
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", media.ErrCorrupt, err)
	}
	return img, nil
}

// toNRGBA copies img into a new NRGBA image whose bounds start at the origin.
func toNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, img, b.Min, draw.Src)
	return dst
}

// firstFrame renders the first frame of an animation on its logical screen.
func firstFrame(g *gif.GIF) *image.NRGBA {
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		b := g.Image[0].Bounds()
		w, h = b.Max.X, b.Max.Y
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, g.Image[0].Bounds(), g.Image[0], g.Image[0].Bounds().Min, draw.Over)
	return dst
}

// fit scales img down to at most maxPixels, keeping its aspect ratio; an image that fits is
// returned as it is.
func fit(img *image.NRGBA, maxPixels int64) *image.NRGBA {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if int64(w)*int64(h) <= maxPixels {
		return img
	}
	scale := math.Sqrt(float64(maxPixels) / float64(int64(w)*int64(h)))
	nw := max(1, int(math.Floor(float64(w)*scale)))
	nh := max(1, int(math.Floor(float64(h)*scale)))
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.BiLinear.Scale(dst, dst.Rect, img, img.Rect, draw.Src, nil)
	return dst
}

// encode writes img as PNG or as JPEG. Neither encoder writes metadata.
func encode(img *image.NRGBA, asPNG bool) (media.Variant, error) {
	var buf bytes.Buffer
	v := media.Variant{Width: img.Rect.Dx(), Height: img.Rect.Dy()}
	if asPNG {
		if err := png.Encode(&buf, img); err != nil {
			return media.Variant{}, fmt.Errorf("media/native: encode png: %w", err)
		}
		v.ContentType, v.Extension = "image/png", "png"
	} else {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return media.Variant{}, fmt.Errorf("media/native: encode jpeg: %w", err)
		}
		v.ContentType, v.Extension = "image/jpeg", "jpg"
	}
	v.Data = buf.Bytes()
	return v, nil
}
