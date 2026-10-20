// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"time"

	// The WebP package sets the frame codecs of its animation package.
	_ "github.com/deepteams/webp"
	"github.com/deepteams/webp/animation"
)

// MetadataMarker is the text that the media fixtures hide in their metadata (EXIF
// ImageDescription and GPS, PNG tEXt, GIF comment). A processed file must not contain it.
const MetadataMarker = "SECRET-LOCATION-35.6812N-139.7671E"

// Colors of the halves of the fixture images: the left half is FixtureLeft and the right half
// FixtureRight, so that tests can tell how an image was turned.
var (
	FixtureLeft  = color.NRGBA{R: 220, G: 20, B: 20, A: 255}
	FixtureRight = color.NRGBA{R: 20, G: 20, B: 220, A: 255}
)

// FixtureImage returns a w × h image whose left half is FixtureLeft and right half
// FixtureRight.
func FixtureImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := FixtureLeft
			if x >= w/2 {
				c = FixtureRight
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// FixtureJPEG returns a w × h JPEG of FixtureImage with an EXIF segment that holds the
// Orientation tag (1 to 8), an ImageDescription, and a GPS position; the latter two contain
// MetadataMarker.
func FixtureJPEG(w, h, orientation int) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, FixtureImage(w, h), &jpeg.Options{Quality: 95}); err != nil {
		panic(err)
	}
	tiff := exifTIFF(orientation)
	app1 := make([]byte, 0, 4+6+len(tiff))
	app1 = append(app1, 0xFF, 0xE1)
	app1 = binary.BigEndian.AppendUint16(app1, uint16(2+6+len(tiff))) //nolint:gosec // the segment is a few hundred bytes
	app1 = append(app1, "Exif\x00\x00"...)
	app1 = append(app1, tiff...)
	data := buf.Bytes()
	out := make([]byte, 0, len(data)+len(app1))
	out = append(out, data[:2]...) // SOI
	out = append(out, app1...)
	return append(out, data[2:]...)
}

// exifTIFF builds a big-endian TIFF structure with IFD0 (ImageDescription, Orientation,
// GPSInfo) and a GPS IFD (GPSLatitudeRef, GPSLatitude, and MetadataMarker as GPSAreaInformation).
func exifTIFF(orientation int) []byte {
	be := binary.BigEndian
	desc := MetadataMarker + "\x00"
	const ifd0 = 8
	const ifd0Size = 2 + 3*12 + 4
	descOff := ifd0 + ifd0Size
	gpsOff := descOff + len(desc)
	const gpsSize = 2 + 3*12 + 4
	latOff := gpsOff + gpsSize
	areaOff := latOff + 24

	t := []byte("MM\x00\x2A")
	t = be.AppendUint32(t, ifd0)
	entry := func(tag, typ uint16, count, value uint32) {
		t = be.AppendUint16(t, tag)
		t = be.AppendUint16(t, typ)
		t = be.AppendUint32(t, count)
		t = be.AppendUint32(t, value)
	}
	t = be.AppendUint16(t, 3)
	entry(0x010E, 2, uint32(len(desc)), uint32(descOff)) //nolint:gosec // small offsets
	entry(0x0112, 3, 1, uint32(orientation)<<16)         //nolint:gosec // 1 to 8, left-justified SHORT
	entry(0x8825, 4, 1, uint32(gpsOff))                  //nolint:gosec // small offsets
	t = be.AppendUint32(t, 0)
	t = append(t, desc...)
	t = be.AppendUint16(t, 3)
	entry(0x0001, 2, 2, uint32('N')<<24)
	entry(0x0002, 5, 3, uint32(latOff))                            //nolint:gosec // small offsets
	entry(0x001C, 7, uint32(len(MetadataMarker)), uint32(areaOff)) //nolint:gosec // small offsets
	t = be.AppendUint32(t, 0)
	for _, r := range [][2]uint32{{35, 1}, {40, 1}, {5232, 100}} {
		t = be.AppendUint32(t, r[0])
		t = be.AppendUint32(t, r[1])
	}
	return append(t, MetadataMarker...)
}

// FixturePNG returns a w × h PNG of FixtureImage with a tEXt chunk that contains
// MetadataMarker. With alpha, the right half is half transparent.
func FixturePNG(w, h int, alpha bool) []byte {
	img := FixtureImage(w, h)
	if alpha {
		for y := range h {
			for x := w / 2; x < w; x++ {
				c := FixtureRight
				c.A = 128
				img.SetNRGBA(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return insertPNGChunk(buf.Bytes(), "tEXt", []byte("Comment\x00"+MetadataMarker))
}

// insertPNGChunk inserts a chunk right after IHDR.
func insertPNGChunk(data []byte, typ string, body []byte) []byte {
	const afterIHDR = 8 + 4 + 4 + 13 + 4
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body))) //nolint:gosec // small chunks
	chunk = append(chunk, typ...)
	chunk = append(chunk, body...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(append([]byte(typ), body...)))
	out := make([]byte, 0, len(data)+len(chunk))
	out = append(out, data[:afterIHDR]...)
	out = append(out, chunk...)
	return append(out, data[afterIHDR:]...)
}

// FixturePNGHeader returns a PNG whose header claims w × h pixels, followed by a small
// compressed stream of zeros and no real image: a decompression bomb when w and h are large.
func FixturePNGHeader(w, h uint32) []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, 8, 0, 0, 0, 0) // 8-bit grayscale
	data := []byte("\x89PNG\r\n\x1A\n")
	chunk := func(typ string, body []byte) {
		data = binary.BigEndian.AppendUint32(data, uint32(len(body))) //nolint:gosec // small chunks
		data = append(data, typ...)
		data = append(data, body...)
		data = binary.BigEndian.AppendUint32(data, crc32.ChecksumIEEE(append([]byte(typ), body...)))
	}
	chunk("IHDR", ihdr)
	// A zlib stream of 1 MiB of zeros, about a kilobyte compressed.
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write(make([]byte, 1<<20))
	_ = zw.Close()
	chunk("IDAT", z.Bytes())
	chunk("IEND", nil)
	return data
}

// FixtureGIF returns a w × h GIF with the given number of frames (the first one is
// FixtureImage, then plain colors) and a comment extension that contains MetadataMarker.
func FixtureGIF(w, h, frames int) []byte {
	g := &gif.GIF{LoopCount: 0}
	for i := range frames {
		p := image.NewPaletted(image.Rect(0, 0, w, h), palette.Plan9)
		if i == 0 {
			draw.Draw(p, p.Rect, FixtureImage(w, h), image.Point{}, draw.Src)
		} else {
			for j := range p.Pix {
				p.Pix[j] = uint8(i * 37) //nolint:gosec // any palette index
			}
		}
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		panic(err)
	}
	data := buf.Bytes()
	p := 13
	if flags := data[10]; flags&0x80 != 0 {
		p += 3 << ((flags & 0x07) + 1)
	}
	comment := []byte{0x21, 0xFE, byte(len(MetadataMarker))}
	comment = append(comment, MetadataMarker...)
	comment = append(comment, 0)
	out := make([]byte, 0, len(data)+len(comment))
	out = append(out, data[:p]...)
	out = append(out, comment...)
	return append(out, data[p:]...)
}

func mustBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// WebP fixtures of 1 × 1 pixel: lossy, lossless, lossy with an alpha channel, and animated.
var (
	FixtureWebPLossy    = mustBase64("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	FixtureWebPLossless = mustBase64("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	FixtureWebPAlpha    = mustBase64("UklGRkoAAABXRUJQVlA4WAoAAAAQAAAAAAAAAAAAQUxQSAwAAAARBxAR/Q9ERP8DAABWUDggGAAAABQBAJ0BKgEAAQAAAP4AAA3AAP7mtQAAAA==")
	FixtureWebPAnimated = mustBase64("UklGRlIAAABXRUJQVlA4WAoAAAASAAAAAAAAAAAAQU5JTQYAAAD/////AABBTk1GJgAAAAAAAAAAAAAAAAAAAGQAAABWUDhMDQAAAC8AAAAQBxAREYiI/gcA")
)

// FixtureWebPAnimation returns a w × h animated WebP with the given number of frames (the first
// one is FixtureImage, then plain colors), played forever, with an EXIF chunk that contains
// MetadataMarker.
func FixtureWebPAnimation(w, h, frames int) []byte {
	var buf bytes.Buffer
	enc := animation.NewEncoder(&buf, w, h, &animation.EncodeOptions{Lossless: true})
	enc.SetEXIF([]byte("Exif\x00\x00" + MetadataMarker))
	for i := range frames {
		img := FixtureImage(w, h)
		if i > 0 {
			c := color.NRGBA{R: uint8(i * 37), G: uint8(i * 91), B: 120, A: 255} //nolint:gosec // any color
			for p := 0; p < len(img.Pix); p += 4 {
				img.Pix[p], img.Pix[p+1], img.Pix[p+2], img.Pix[p+3] = c.R, c.G, c.B, c.A
			}
		}
		if err := enc.AddFrame(img, 100*time.Millisecond); err != nil {
			panic(err)
		}
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
