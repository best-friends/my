// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// The builders below write small video and audio files for the tests of the MediaProcessor
// port: valid containers whose samples are placeholders ("FRAME0", "FRAME1", ...) instead of
// coded media, since a remux never decodes them. Each file carries the metadata a remux must
// drop, marked with AVSecret.

// AVSecret marks the metadata in the files of the builders: titles, tags, locations, and
// dates that a remux must not keep.
const AVSecret = "SECRET-METADATA"

// MP4Options describe an MP4 of MP4File.
type MP4Options struct {
	// VideoCodec is the sample entry of the video track ("avc1", "hvc1"), "" for none.
	VideoCodec string
	// AudioCodec is the sample entry of the sound track ("mp4a"), "" for none.
	AudioCodec    string
	Width, Height int
	// Rotated turns the video a quarter in its track header's matrix, as phones record
	// portrait videos.
	Rotated bool
	// Frames is the number of video samples, over Seconds.
	Frames  int
	Seconds float64
	// QuickTime writes the brand of a QuickTime movie (.mov).
	QuickTime bool
	// Cover is an image in the iTunes item list (covr), nil for none.
	Cover []byte
}

func mp4Box(typ string, parts ...[]byte) []byte {
	n := 8
	for _, p := range parts {
		n += len(p)
	}
	out := binary.BigEndian.AppendUint32(make([]byte, 0, n), uint32(n))
	out = append(out, typ...)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func u32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func u16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }

// u32n, u16n, and b8 write the small sizes and counts of the test files.
func u32n(n int) []byte { return u32(uint32(n)) } //nolint:gosec // G115: small numbers of the test files
func u16n(n int) []byte { return u16(uint16(n)) } //nolint:gosec // G115: small numbers of the test files
func b8(n int) byte     { return byte(n) }        //nolint:gosec // G115: small numbers of the test files

// MP4File writes an MP4 with its moov after its mdat (as cameras write them), user data with
// a location (udta ©xyz), metadata (meta), and a uuid box, and for each track the samples
// "FRAME<i>" (video) or "AUDIO<i>" (sound), one per chunk.
func MP4File(o MP4Options) []byte {
	brand := []byte("isom\x00\x00\x02\x00isomiso2mp41")
	if o.QuickTime {
		brand = []byte("qt  \x00\x00\x02\x00qt  ")
	}
	ftyp := mp4Box("ftyp", brand)
	secret := mp4Box("uuid", []byte("0123456789abcdef"+AVSecret))
	type track struct {
		handler, codec string
		samples        [][]byte
	}
	var tracks []track
	if o.VideoCodec != "" {
		t := track{handler: "vide", codec: o.VideoCodec}
		for i := range max(o.Frames, 1) {
			t.samples = append(t.samples, fmt.Appendf(nil, "FRAME%d", i))
		}
		tracks = append(tracks, t)
	}
	if o.AudioCodec != "" {
		t := track{handler: "soun", codec: o.AudioCodec}
		for i := range 4 {
			t.samples = append(t.samples, fmt.Appendf(nil, "AUDIO%d", i))
		}
		tracks = append(tracks, t)
	}
	var mdatBody []byte
	offsets := make([][]int, len(tracks))
	base := len(ftyp) + len(secret) + 8
	// Interleave the samples of the tracks.
	for i := 0; ; i++ {
		more := false
		for ti, t := range tracks {
			if i < len(t.samples) {
				offsets[ti] = append(offsets[ti], base+len(mdatBody))
				mdatBody = append(mdatBody, t.samples[i]...)
				more = true
			}
		}
		if !more {
			break
		}
	}
	mdat := mp4Box("mdat", mdatBody)
	const timescale = 1000
	duration := uint32(o.Seconds * timescale)
	date := u32(0xDEADBEEF) // a creation time to zero
	mvhd := mp4Box("mvhd", []byte{0, 0, 0, 0}, date, date, u32(timescale), u32(duration), make([]byte, 80))
	var traks [][]byte
	for ti, t := range tracks {
		matrix := make([]byte, 36)
		if o.Rotated && t.handler == "vide" {
			copy(matrix, u32(0))
			copy(matrix[4:], u32(0x10000))
			copy(matrix[12:], u32(0xFFFF0000))
		} else {
			copy(matrix, u32(0x10000))
			copy(matrix[16:], u32(0x10000))
		}
		copy(matrix[32:], u32(0x40000000))
		var w, h int
		if t.handler == "vide" {
			w, h = o.Width<<16, o.Height<<16
		}
		tkhd := mp4Box("tkhd", []byte{0, 0, 0, 3}, date, date, u32(uint32(ti+1)), u32(0), u32(duration), make([]byte, 16), matrix, u32n(w), u32n(h))
		mdhd := mp4Box("mdhd", []byte{0, 0, 0, 0}, date, date, u32(timescale), u32(duration), u32(0))
		hdlr := mp4Box("hdlr", []byte{0, 0, 0, 0}, u32(0), []byte(t.handler), make([]byte, 12), []byte("handler\x00"))
		var entry []byte
		if t.handler == "vide" {
			entry = mp4Box(t.codec, make([]byte, 6), u16(1), make([]byte, 16), u16n(o.Width), u16n(o.Height), make([]byte, 50))
		} else {
			entry = mp4Box(t.codec, make([]byte, 6), u16(1), make([]byte, 20))
		}
		stsd := mp4Box("stsd", []byte{0, 0, 0, 0}, u32(1), entry)
		stts := mp4Box("stts", []byte{0, 0, 0, 0}, u32(1), u32n(len(t.samples)), u32(1))
		stsc := mp4Box("stsc", []byte{0, 0, 0, 0}, u32(1), u32(1), u32(1), u32(1))
		sizes := []byte{0, 0, 0, 0}
		sizes = append(sizes, u32(0)...)
		sizes = append(sizes, u32n(len(t.samples))...)
		for _, s := range t.samples {
			sizes = append(sizes, u32n(len(s))...)
		}
		stsz := mp4Box("stsz", sizes)
		co := []byte{0, 0, 0, 0}
		co = append(co, u32n(len(offsets[ti]))...)
		for _, off := range offsets[ti] {
			co = append(co, u32n(off)...)
		}
		stco := mp4Box("stco", co)
		dinf := mp4Box("dinf", mp4Box("dref", []byte{0, 0, 0, 0}, u32(1), mp4Box("url ", []byte{0, 0, 0, 1})))
		minf := mp4Box("minf", dinf, mp4Box("stbl", stsd, stts, stsc, stsz, stco))
		mdia := mp4Box("mdia", mdhd, hdlr, minf)
		udta := mp4Box("udta", mp4Box("name", []byte(AVSecret)))
		traks = append(traks, mp4Box("trak", tkhd, mdia, udta))
	}
	ilst := []byte{}
	if o.Cover != nil {
		ilst = mp4Box("covr", mp4Box("data", u32(14), u32(0), o.Cover))
	}
	meta := mp4Box("meta", []byte{0, 0, 0, 0}, mp4Box("hdlr", []byte{0, 0, 0, 0}, u32(0), []byte("mdir"), make([]byte, 12), []byte{0}),
		mp4Box("ilst", mp4Box("\xA9nam", mp4Box("data", u32(1), u32(0), []byte(AVSecret))), ilst))
	udta := mp4Box("udta", mp4Box("\xA9xyz", u16(17), u16(0), []byte("+35.6586+139.7454/")), mp4Box("\xA9day", []byte(AVSecret)), meta)
	parts := [][]byte{mvhd}
	parts = append(parts, traks...)
	parts = append(parts, udta, mp4Box("meta", []byte{0, 0, 0, 0}, mp4Box("keys", []byte(AVSecret))))
	moov := mp4Box("moov", parts...)
	return slices.Concat(ftyp, secret, mdat, moov)
}

// EBML element writing for WebMFile.
func ebml(id uint32, body ...[]byte) []byte {
	var out []byte
	for shift := 24; shift >= 0; shift -= 8 {
		if b := byte(id >> shift); b != 0 || len(out) > 0 { //nolint:gosec // G115: one byte of the ID at a time
			out = append(out, b)
		}
	}
	n := 0
	for _, b := range body {
		n += len(b)
	}
	// An 8-byte size, as muxers that write the size afterwards do.
	out = append(out, 0x01, byte(n>>48), byte(n>>40), byte(n>>32), byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	for _, b := range body {
		out = append(out, b...)
	}
	return out
}

func ebmlUint(id uint32, v uint64) []byte { return ebml(id, binary.BigEndian.AppendUint64(nil, v)) }

// WebMOptions describe a WebM of WebMFile.
type WebMOptions struct {
	// VideoCodec is the codec ID of the video track ("V_VP9"), "" for none.
	VideoCodec string
	// AudioCodec is the codec ID of the audio track ("A_OPUS"), "" for none.
	AudioCodec    string
	Width, Height int
	// Frames is the number of video blocks, over Seconds.
	Frames  int
	Seconds float64
	// NoDuration leaves the duration out of the segment information, as browsers' recorders
	// write.
	NoDuration bool
}

// WebM element IDs of WebMFile.
const (
	webmSeekHead = 0x114D9B74
	webmInfo     = 0x1549A966
	webmTracks   = 0x1654AE6B
	webmCluster  = 0x1F43B675
	webmCues     = 0x1C53BB6B
	webmTags     = 0x1254C367
)

// WebMFile writes a WebM with a seek index, segment information with a title and a date, tracks
// with names, a cluster of blocks "FRAME<i>" and "AUDIO<i>", padding, a cue index, and tags.
// The seek index points to the information, the tracks, the cues, and the tags; the cue index
// points to the cluster.
func WebMFile(o WebMOptions) []byte {
	header := ebml(0x1A45DFA3, ebmlUint(0x4286, 1), ebml(0x4282, []byte("webm")))
	infoFields := [][]byte{ebmlUint(0x2AD7B1, 1_000_000), ebml(0x7BA9, []byte(AVSecret)), ebml(0x4461, []byte("\x00\x00\x00\x00\x00\x00\x00\x01")), ebml(0x4D80, []byte("muxer"))}
	if !o.NoDuration {
		infoFields = append(infoFields, ebml(0x4489, binary.BigEndian.AppendUint64(nil, math.Float64bits(o.Seconds*1000))))
	}
	info := ebml(webmInfo, infoFields...)
	var entries [][]byte
	num := uint64(1)
	video, audio := uint64(0), uint64(0)
	if o.VideoCodec != "" {
		video = num
		entries = append(entries, ebml(0xAE, ebmlUint(0xD7, num), ebmlUint(0x83, 1), ebml(0x86, []byte(o.VideoCodec)), ebml(0x536E, []byte(AVSecret)),
			ebml(0xE0, ebml(0xB0, u32n(o.Width)), ebml(0xBA, u32n(o.Height)))))
		num++
	}
	if o.AudioCodec != "" {
		audio = num
		entries = append(entries, ebml(0xAE, ebmlUint(0xD7, num), ebmlUint(0x83, 2), ebml(0x86, []byte(o.AudioCodec)), ebml(0x536E, []byte(AVSecret))))
	}
	tracks := ebml(webmTracks, entries...)
	block := func(track uint64, at int16, payload string) []byte {
		b := []byte{0x80 | b8(int(track))} //nolint:gosec // G115: track numbers are 1 and 2
		b = append(b, u16n(int(at))...)
		return ebml(0xA3, append(append(b, 0x80), payload...))
	}
	blocks := [][]byte{ebmlUint(0xE7, 0)}
	frames := max(o.Frames, 1)
	last := int16(min(o.Seconds*1000, math.MaxInt16))
	for i := range frames {
		at := int16(0)
		if frames > 1 {
			at = int16(int(last) * i / (frames - 1)) //nolint:gosec // G115: at most last
		}
		if video != 0 {
			blocks = append(blocks, block(video, at, fmt.Sprintf("FRAME%d", i)))
		}
		if audio != 0 {
			blocks = append(blocks, block(audio, at, fmt.Sprintf("AUDIO%d", i)))
		}
	}
	cluster := ebml(webmCluster, blocks...)
	void := ebml(0xEC, []byte(AVSecret))
	tags := ebml(webmTags, ebml(0x7373, ebml(0x67C8, ebml(0x45A3, []byte("TITLE")), ebml(0x4487, []byte(AVSecret)))))
	// Positions are relative to the segment's data; the seek index has a fixed size.
	seek := func(id uint32, pos uint64) []byte {
		return ebml(0x4DBB, ebml(0x53AB, u32(id)), ebmlUint(0x53AC, pos))
	}
	seekHeadSize := len(ebml(webmSeekHead, seek(0, 0), seek(0, 0), seek(0, 0), seek(0, 0)))
	posInfo := uint64(seekHeadSize)
	posTracks := posInfo + uint64(len(info))
	posCluster := posTracks + uint64(len(tracks))
	posCues := posCluster + uint64(len(cluster)) + uint64(len(void))
	cues := ebml(webmCues, ebml(0xBB, ebmlUint(0xB3, 0), ebml(0xB7, ebmlUint(0xF7, 1), ebmlUint(0xF1, posCluster))))
	posTags := posCues + uint64(len(cues))
	seekHead := ebml(webmSeekHead, seek(webmInfo, posInfo), seek(webmTracks, posTracks), seek(webmCues, posCues), seek(webmTags, posTags))
	segment := ebml(0x18538067, seekHead, info, tracks, cluster, void, cues, tags)
	return append(header, segment...)
}

// MP3File writes an MP3 of frames MPEG 1 Layer III frames at 128 kbit/s and 44.1 kHz, about
// 26 ms each, between an ID3v2.3 tag with a title and a cover picture (APIC; none when cover is
// nil) and an ID3v1 tag.
func MP3File(frames int, cover []byte) []byte {
	frame := func(id3 string, body []byte) []byte {
		return append(append([]byte(id3), u32n(len(body))...), append([]byte{0, 0}, body...)...)
	}
	body := frame("TIT2", append([]byte{0}, AVSecret...))
	if cover != nil {
		body = append(body, frame("APIC", append([]byte("\x00image/png\x00\x03cover\x00"), cover...))...)
	}
	n := len(body)
	out := append([]byte("ID3\x03\x00\x00"), byte(n>>21&0x7F), byte(n>>14&0x7F), byte(n>>7&0x7F), byte(n&0x7F))
	out = append(out, body...)
	for i := range frames {
		f := make([]byte, 417) // 144 × 128000 / 44100, no padding
		copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})
		copy(f[4:], fmt.Sprintf("FRAME%d", i))
		out = append(out, f...)
	}
	tag := make([]byte, 128)
	copy(tag, "TAG"+AVSecret)
	return append(out, tag...)
}

// OggOpusFile writes an Ogg Opus file of seconds with an OpusTags header that has a title, and
// audio pages of the packets "AUDIO<i>", one every 20 ms.
func OggOpusFile(seconds float64) []byte {
	head := append([]byte("OpusHead\x01\x02"), 0x38, 0x01) // version, channels, pre-skip 312
	head = append(head, 0x80, 0xBB, 0, 0, 0, 0, 0)         // 48 kHz, gain, mapping family
	tags := append([]byte("OpusTags"), 7, 0, 0, 0)
	tags = append(tags, "libopus"...)
	tags = append(tags, 1, 0, 0, 0)
	comment := "TITLE=" + AVSecret
	tags = append(tags, b8(len(comment)), 0, 0, 0)
	tags = append(tags, comment...)
	page := func(flags byte, granule uint64, seq uint32, packets ...[]byte) []byte {
		var lacing, body []byte
		for _, p := range packets {
			n := len(p)
			for n >= 255 {
				lacing, n = append(lacing, 255), n-255
			}
			lacing = append(lacing, b8(n))
			body = append(body, p...)
		}
		out := append([]byte("OggS\x00"), flags)
		out = binary.LittleEndian.AppendUint64(out, granule)
		out = binary.LittleEndian.AppendUint32(out, 0x1234)
		out = binary.LittleEndian.AppendUint32(out, seq)
		out = append(out, 0, 0, 0, 0, b8(len(lacing)))
		out = append(out, lacing...)
		out = append(out, body...)
		binary.LittleEndian.PutUint32(out[22:], OggCRC(out))
		return out
	}
	out := page(0x02, 0, 0, head)
	out = append(out, page(0, 0, 1, tags)...)
	packets := max(int(seconds*50), 1)
	seq := uint32(2)
	for i := 0; i < packets; i += 10 {
		var ps [][]byte
		for j := i; j < min(i+10, packets); j++ {
			ps = append(ps, fmt.Appendf(nil, "AUDIO%d", j))
		}
		var flags byte
		if i+10 >= packets {
			flags = 0x04
		}
		out = append(out, page(flags, uint64(312+min(i+10, packets)*960), seq, ps...)...)
		seq++
	}
	return out
}

// OggCRC returns the checksum of an Ogg page whose checksum field is zero.
func OggCRC(b []byte) uint32 {
	var crc uint32
	for _, v := range b {
		crc ^= uint32(v) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
