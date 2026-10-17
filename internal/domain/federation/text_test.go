// SPDX-License-Identifier: AGPL-3.0-only

package federation

import "testing"

func TestCanonicalLanguage(t *testing.T) {
	cases := map[string]string{
		"en": "en", "EN-us": "en-US", "zh_hant_tw": "zh-Hant-TW", "es-419": "es-419", "ja-JP-u-ca-japanese": "ja-JP-u-ca-japanese",
		"": "und", " ": "und", "e": "und", "1a": "und", "en--us": "und", "en-toolongsubtag": "und", "日本語": "und",
	}
	for in, want := range cases {
		if got := canonicalLanguage(in); got != want {
			t.Errorf("canonicalLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlainText(t *testing.T) {
	cases := map[string]string{
		"plain":                                "plain",
		"<p>a</p><p>b</p>":                     "a\nb",
		"a<br/>b<BR>c":                         "a\nb\nc",
		`<a href="x">link</a> &amp; &lt;b&gt;`: "link & <b>",
		"cut <span":                            "cut",
		"  spaced  ":                           "spaced",
	}
	for in, want := range cases {
		if got := plainText(in); got != want {
			t.Errorf("plainText(%q) = %q, want %q", in, got, want)
		}
	}
}
