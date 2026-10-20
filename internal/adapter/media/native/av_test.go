// SPDX-License-Identifier: AGPL-3.0-only

package native

import (
	"testing"

	"github.com/best-friends/my/internal/app/media"
	"github.com/best-friends/my/pkg/testkit"
)

// remuxAV runs the parser of data's container without the recovery of Processor.Remux, so that
// fuzzing finds panics.
func remuxAV(data []byte) (media.AVOutput, error) {
	limits := media.DefaultAVLimits()
	switch media.SniffAV(data) {
	case media.FormatMP4:
		f, err := parseMP4(data, limits)
		if err != nil {
			return media.AVOutput{}, err
		}
		out, err := f.write(data)
		return media.AVOutput{AVProbe: f.probe, Data: out}, err
	case media.FormatWebM:
		f, err := parseWebM(data, limits)
		if err != nil {
			return media.AVOutput{}, err
		}
		out, err := f.write()
		return media.AVOutput{AVProbe: f.probe, Data: out}, err
	case media.FormatMP3:
		return remuxMP3(data, limits)
	case media.FormatOgg:
		return remuxOgg(data, limits)
	}
	return media.AVOutput{}, media.ErrUnsupportedFormat
}

// FuzzRemux checks that no input panics a parser, that every failure is a rejection, and that
// a remuxed file parses again into the same probe.
func FuzzRemux(f *testing.F) {
	f.Add(testkit.MP4File(testkit.MP4Options{VideoCodec: "avc1", AudioCodec: "mp4a", Width: 64, Height: 48, Frames: 4, Seconds: 1, Cover: []byte("cover")}))
	f.Add(testkit.MP4File(testkit.MP4Options{AudioCodec: "mp4a", Seconds: 1, QuickTime: true}))
	f.Add(testkit.WebMFile(testkit.WebMOptions{VideoCodec: "V_VP9", AudioCodec: "A_OPUS", Width: 64, Height: 48, Frames: 4, Seconds: 1}))
	f.Add(testkit.WebMFile(testkit.WebMOptions{AudioCodec: "A_VORBIS", Frames: 4, Seconds: 1, NoDuration: true}))
	f.Add(testkit.MP3File(4, []byte("cover")))
	f.Add(testkit.OggOpusFile(0.5))
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := remuxAV(data)
		if err != nil {
			if !media.Rejected(err) {
				t.Fatalf("remux failed with %v, not a rejection", err)
			}
			return
		}
		again, err := remuxAV(out.Data)
		if err != nil {
			t.Fatalf("the remuxed file does not parse: %v", err)
		}
		if again.AVProbe != out.AVProbe {
			t.Fatalf("the remuxed file parses into %+v, the source into %+v", again.AVProbe, out.AVProbe)
		}
	})
}
