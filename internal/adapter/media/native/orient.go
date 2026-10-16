// SPDX-License-Identifier: AGPL-3.0-only

package native

import (
	"encoding/binary"
	"image"
)

// exifOrientation returns the EXIF Orientation (1 to 8) of a JPEG, or 1 when it has none. It
// reads the APP1 "Exif" segments before the image data and tag 0x0112 of their IFD0.
func exifOrientation(jpeg []byte) int {
	p := 2 // after SOI
	for p+4 <= len(jpeg) {
		if jpeg[p] != 0xFF {
			return 1
		}
		marker := jpeg[p+1]
		if marker == 0xD8 || marker >= 0xD0 && marker <= 0xD7 || marker == 0x01 || marker == 0xFF {
			p++ // markers without a length (and fill bytes)
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan, end of image
			return 1
		}
		n := int(binary.BigEndian.Uint16(jpeg[p+2:]))
		if n < 2 || p+2+n > len(jpeg) {
			return 1
		}
		seg := jpeg[p+4 : p+2+n]
		if marker == 0xE1 && len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00" {
			if o := tiffOrientation(seg[6:]); o != 0 {
				return o
			}
		}
		p += 2 + n
	}
	return 1
}

// tiffOrientation returns the Orientation tag of IFD0 of a TIFF structure, or 0.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(t[2:]) != 42 {
		return 0
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 0
	}
	count := int(order.Uint16(t[ifd:]))
	for i := range count {
		e := ifd + 2 + 12*i
		if e+12 > len(t) {
			return 0
		}
		const tagOrientation, typeShort = 0x0112, 3
		if order.Uint16(t[e:]) == tagOrientation && order.Uint16(t[e+2:]) == typeShort {
			if o := int(order.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 0
		}
	}
	return 0
}

// orient returns img turned upright for an EXIF orientation: mirrored and rotated so that it
// displays as intended without the tag.
func orient(img *image.NRGBA, orientation int) *image.NRGBA {
	if orientation < 2 || orientation > 8 {
		return img
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for dy := range dh {
		for dx := range dw {
			var sx, sy int
			switch orientation {
			case 2: // mirrored horizontally
				sx, sy = w-1-dx, dy
			case 3: // rotated 180°
				sx, sy = w-1-dx, h-1-dy
			case 4: // mirrored vertically
				sx, sy = dx, h-1-dy
			case 5: // transposed
				sx, sy = dy, dx
			case 6: // needs a 90° clockwise rotation
				sx, sy = dy, h-1-dx
			case 7: // transversed
				sx, sy = w-1-dy, h-1-dx
			case 8: // needs a 90° counter-clockwise rotation
				sx, sy = w-1-dy, dx
			}
			si := img.PixOffset(img.Rect.Min.X+sx, img.Rect.Min.Y+sy)
			di := dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], img.Pix[si:si+4])
		}
	}
	return dst
}
