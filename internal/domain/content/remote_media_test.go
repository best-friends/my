// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"encoding/binary"
	"testing"

	"github.com/best-friends/my/internal/domain/content"
)

// ftyp returns the start of an ISO base media file whose ftyp box has the major brand and the
// compatible brands.
func ftyp(major string, compatible ...string) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(16+4*len(compatible))) //nolint:gosec // a handful of brands
	b = append(b, "ftyp"+major+"\x00\x00\x00\x00"...)
	for _, c := range compatible {
		b = append(b, c...)
	}
	return append(b, "\x00\x00\x00\x08free"...)
}

func TestSniffRemoteMedia(t *testing.T) {
	for _, tc := range []struct {
		name string
		head []byte
		ct   string
		kind content.MediaType
	}{
		{"jpeg", []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF"), "image/jpeg", content.MediaTypeImage},
		{"png", []byte("\x89PNG\r\n\x1A\n\x00\x00\x00\x0DIHDR"), "image/png", content.MediaTypeImage},
		{"gif", []byte("GIF89a\x01\x00\x01\x00"), "image/gif", content.MediaTypeImage},
		{"webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), "image/webp", content.MediaTypeImage},
		{"avif", ftyp("avif", "mif1", "miaf"), "image/avif", content.MediaTypeImage},
		{"avif by compatible brand", ftyp("mif1", "avif"), "image/avif", content.MediaTypeImage},
		{"mp4", ftyp("isom", "iso2", "avc1", "mp41"), "video/mp4", content.MediaTypeVideo},
		{"m4a", ftyp("M4A ", "isom"), "audio/mp4", content.MediaTypeAudio},
		{"webm", []byte("\x1A\x45\xDF\xA3\x9F\x42\x86\x81\x01\x42\x82\x84webm"), "video/webm", content.MediaTypeVideo},
		{"ogg", []byte("OggS\x00\x02"), "audio/ogg", content.MediaTypeAudio},
		{"mp3 with ID3", []byte("ID3\x04\x00"), "audio/mpeg", content.MediaTypeAudio},
		{"mp3 frame", []byte("\xFF\xFB\x90\x64"), "audio/mpeg", content.MediaTypeAudio},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "", content.MediaTypeUnknown},
		{"svg with xml declaration", []byte(`<?xml version="1.0"?><svg/>`), "", content.MediaTypeUnknown},
		{"html", []byte("<!DOCTYPE html><html>"), "", content.MediaTypeUnknown},
		{"matroska", []byte("\x1A\x45\xDF\xA3\x9F\x42\x86\x81\x01\x42\x82\x88matroska"), "", content.MediaTypeUnknown},
		{"quicktime", ftyp("qt  ", "qt  "), "", content.MediaTypeUnknown},
		{"heic", ftyp("heic", "mif1", "heic"), "", content.MediaTypeUnknown},
		{"truncated ftyp", []byte("\x00\x00\x00\x18ftyp"), "", content.MediaTypeUnknown},
		{"aac adts", []byte("\xFF\xF1\x50\x80"), "", content.MediaTypeUnknown},
		{"empty", nil, "", content.MediaTypeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ct, kind := content.SniffRemoteMedia(tc.head)
			if ct != tc.ct || kind != tc.kind {
				t.Errorf("SniffRemoteMedia = %q, %s; want %q, %s", ct, kind, tc.ct, tc.kind)
			}
		})
	}
}
