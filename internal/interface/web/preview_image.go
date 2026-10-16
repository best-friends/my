// SPDX-License-Identifier: AGPL-3.0-only

package web

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"sync"
	"time"
)

// The built-in preview image is the og:image of profiles and posts without images when the
// instance has no preview_image setting (ADR-0028): a generic picture that names no instance,
// so that one image fits every instance. It is drawn with the standard library when first
// served instead of being committed as a binary file: the code is smaller than the PNG, its
// size is a constant that og:image:width and og:image:height state, and it never goes stale.

// PathDefaultImage is the path of the built-in preview image.
const PathDefaultImage = "/preview-image.png"

// Size of the built-in preview image, the 1.91:1 size that link previews show as a large
// image.
const (
	DefaultImageWidth  = 1200
	DefaultImageHeight = 630
)

// defaultImage is the PNG of the built-in preview image and its ETag.
var defaultImage = sync.OnceValues(func() ([]byte, string) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, drawDefaultImage()); err != nil {
		panic("web: encode the built-in preview image: " + err.Error())
	}
	return buf.Bytes(), etag(buf.Bytes())
})

// Colors of the built-in preview image: a horizontal gradient behind a speech bubble with
// three dots.
var (
	gradientStart = color.NRGBA{R: 0x4f, G: 0x46, B: 0xe5, A: 0xff}
	gradientEnd   = color.NRGBA{R: 0x0e, G: 0xa5, B: 0xe9, A: 0xff}
	bubbleColor   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// drawDefaultImage draws the built-in preview image. Edges are antialiased from their signed
// distance; every row outside the bubble is the same, which PNG compresses to almost nothing.
func drawDefaultImage() *image.NRGBA {
	const (
		w, h           = DefaultImageWidth, DefaultImageHeight
		cx, cy         = w / 2, h / 2
		bubbleHalfW    = 220.0
		bubbleHalfH    = 140.0
		bubbleRadius   = 56.0
		dotRadius      = 24.0
		dotSpacing     = 88.0
		tailHalfWidth  = 36.0
		tailHeight     = 56.0
		tailCenterFrom = -110.0 // the tail's x offset from the center
	)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			bg := mix(gradientStart, gradientEnd, float64(x)/float64(w-1))
			px, py := float64(x)+0.5-cx, float64(y)+0.5-cy
			bubble := math.Min(
				roundedRectDistance(px, py, bubbleHalfW, bubbleHalfH, bubbleRadius),
				tailDistance(px-tailCenterFrom, py-bubbleHalfH+1, tailHalfWidth, tailHeight),
			)
			c := mix(bg, bubbleColor, coverage(bubble))
			for _, dx := range []float64{-dotSpacing, 0, dotSpacing} {
				c = mix(c, bg, coverage(math.Hypot(px-dx, py)-dotRadius))
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// roundedRectDistance is the signed distance from (x, y) to a rectangle centered at the
// origin with half sizes hw, hh and corner radius r: negative inside.
func roundedRectDistance(x, y, hw, hh, r float64) float64 {
	qx, qy := math.Abs(x)-(hw-r), math.Abs(y)-(hh-r)
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

// tailDistance approximates the signed distance to a downward triangle whose top edge is
// centered at the origin, hw wide on each side and height tall.
func tailDistance(x, y, hw, height float64) float64 {
	if y < 0 {
		return -y
	}
	// The slanted edges go from (±hw, 0) to (0, height).
	edge := (math.Abs(x)*height + y*hw - hw*height) / math.Hypot(height, hw)
	return math.Max(edge, y-height)
}

// coverage turns a signed distance into the share of a pixel inside the shape.
func coverage(d float64) float64 { return math.Max(0, math.Min(1, 0.5-d)) }

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: 0xff}
}

// serveDefaultImage serves GET /preview-image.png.
func (h *Handler) serveDefaultImage(w http.ResponseWriter, r *http.Request) {
	data, tag := defaultImage()
	hd := w.Header()
	hd.Set("Content-Type", "image/png")
	hd.Set("Cache-Control", cacheRevalidate)
	hd.Set("ETag", tag)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// builtInImage is the built-in preview image with alt text alt.
func (h *Handler) builtInImage(alt string) *Image {
	return &Image{URL: "https://" + h.o.Host + PathDefaultImage, Width: DefaultImageWidth, Height: DefaultImageHeight, Alt: alt}
}
