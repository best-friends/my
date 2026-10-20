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
	"runtime"

	"golang.org/x/image/draw"
	xwebp "golang.org/x/image/webp"

	"github.com/best-friends/my/internal/app/media"
)

// Options configure a Processor.
type Options struct {
	// Concurrency is the number of files processed at the same time; the default is half the
	// CPUs, at least 1. Decoding a large image takes hundreds of megabytes, so this bounds the
	// memory too.
	Concurrency int
}

// Processor implements media.Processor in pure Go with the standard library's JPEG, PNG,
// and GIF codecs, golang.org/x/image for still WebP decoding and resampling, and
// github.com/deepteams/webp for WebP encoding and animated WebP decoding.
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
func (p *Processor) Process(ctx context.Context, data []byte, limits media.Limits, sizes media.Sizes, req media.Request) (media.Output, error) {
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
		out, err := process(data, probe, sizes, req)
		done <- result{out: out, err: err}
	}()
	select {
	case r := <-done:
		return r.out, r.err
	case <-ctx.Done():
		return media.Output{}, ctx.Err()
	}
}

func process(data []byte, probe media.Probe, sizes media.Sizes, req media.Request) (media.Output, error) {
	boxes := sizes.For(req.Purpose)
	switch {
	case probe.Format == media.FormatGIF:
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return media.Output{}, fmt.Errorf("%w: %w", media.ErrCorrupt, err)
		}
		src := gifSource(g)
		if len(g.Image) > 1 {
			return animate(src, targets(src.width, src.height, boxes, req), g)
		}
		return firstFrameOutput(src, boxes, req)
	case probe.Format == media.FormatWebP && webpAnimated(data):
		src, err := webpSource(data)
		if err != nil {
			return media.Output{}, err
		}
		if probe.Frames > 1 {
			return animate(src, targets(src.width, src.height, boxes, req), nil)
		}
		return firstFrameOutput(src, boxes, req)
	}
	src, err := decodeStill(data, probe.Format)
	if err != nil {
		return media.Output{}, err
	}
	img := toNRGBA(src)
	if probe.Format == media.FormatJPEG {
		img = orient(img, exifOrientation(data))
	}
	return stillOutput(img, boxes, req)
}

// firstFrameOutput makes the still variants of an animation of one frame.
func firstFrameOutput(src animationSource, boxes media.Boxes, req media.Request) (media.Output, error) {
	var first *image.NRGBA
	err := src.each(func(f frame) error {
		if first == nil {
			first = cloneNRGBA(f.canvas)
		}
		return nil
	})
	if err != nil {
		return media.Output{}, err
	}
	if first == nil {
		return media.Output{}, fmt.Errorf("%w: no frames", media.ErrCorrupt)
	}
	return stillOutput(first, boxes, req)
}

// stillOutput makes the variants of a still image.
func stillOutput(img *image.NRGBA, boxes media.Boxes, req media.Request) (media.Output, error) {
	imgs := renderAll(img, targets(img.Rect.Dx(), img.Rect.Dy(), boxes, req), draw.CatmullRom)
	still, err := stillVariants(imgs)
	if err != nil {
		return media.Output{}, err
	}
	return media.Output{Width: img.Rect.Dx(), Height: img.Rect.Dy(), Still: still, Blurhash: blurhash(imgs[0], blurhashX, blurhashY)}, nil
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
		img, err = xwebp.Decode(r)
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
