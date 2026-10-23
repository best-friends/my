// SPDX-License-Identifier: AGPL-3.0-only

package content

import "testing"

func TestNormalizeUnicodeEmoji(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"👍", "👍", true},
		{"🧡", "🧡", true},
		// Unqualified and fully qualified forms count together.
		{"❤", "❤\uFE0F", true},
		{"❤\uFE0F", "❤\uFE0F", true},
		{"☺", "☺\uFE0F", true},
		// Minimally qualified: a ZWJ sequence without its selector.
		{"\U0001F636\u200D\U0001F32B", "\U0001F636\u200D\U0001F32B\uFE0F", true},
		// Skin tones and ZWJ sequences.
		{"👍🏽", "👍🏽", true},
		{"👩\u200D👩\u200D👧", "👩\u200D👩\u200D👧", true},
		// Flags.
		{"🇯🇵", "🇯🇵", true},
		{"", "", false},
		{"a", "", false},
		{"👍👍", "", false},
		{"👍 ", "", false},
		{":blobcat:", "", false},
		// A skin tone alone is a component, not an emoji.
		{"🏽", "", false},
	} {
		got, ok := NormalizeUnicodeEmoji(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NormalizeUnicodeEmoji(%q) = %q, %t; want %q, %t", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseReactionShortcode(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{":blobcat:", "blobcat", true},
		{":blob_cat-2+1.x:", "blob_cat-2+1.x", true},
		{":blobcat@misskey.example:", "blobcat", true},
		{"::", "", false},
		{":", "", false},
		{"blobcat", "", false},
		{":blob cat:", "", false},
		{":<b>:", "", false},
		{"👍", "", false},
	} {
		got, ok := ParseReactionShortcode(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseReactionShortcode(%q) = %q, %t; want %q, %t", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	if got := TrimShortcode("hug"); got != "hug" {
		t.Errorf("TrimShortcode(hug) = %q", got)
	}
}

func TestReactionEmojiKey(t *testing.T) {
	u := ReactionEmoji{Unicode: "👍"}
	c := ReactionEmoji{CustomEmojiID: "0192f0c1-0000-7000-8000-000000000001"}
	if u.Key() != "👍" || c.Key() != c.CustomEmojiID || !u.Valid() || !c.Valid() {
		t.Errorf("keys = %q, %q", u.Key(), c.Key())
	}
	if (ReactionEmoji{}).Valid() || (ReactionEmoji{Unicode: "👍", CustomEmojiID: testID(1)}).Valid() {
		t.Error("an emoji with none or both set is valid")
	}
}
