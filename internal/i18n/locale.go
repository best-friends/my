// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// SourceLocale is the locale every catalog is written in and the last fallback.
const SourceLocale = "en"

// Pseudo-locales generated from the source locale by tools/i18n (ADR-0009).
const (
	PseudoLocaleLong = "en-XA" // accented and about 40% longer
	PseudoLocaleRTL  = "ar-XB" // mirrored with bidi overrides
)

func isPseudo(locale string) bool { return locale == PseudoLocaleLong || locale == PseudoLocaleRTL }

// formattingLocale returns the locale whose CLDR plural and number rules apply to
// messages of locale. Pseudo-locales keep the source locale's plural categories, so
// they are formatted with its rules; tools/i18n does the same on the FormatJS side.
func formattingLocale(locale string) string {
	if isPseudo(locale) {
		return SourceLocale
	}
	return locale
}

// parentLocales overrides the default parent (drop the last subtag) where that would
// cross scripts, following CLDR parentLocales: Traditional Chinese never falls back to
// "zh", which is Simplified.
var parentLocales = map[string]string{
	"zh-Hant": "",
	"zh-HK":   "zh-Hant",
	"zh-MO":   "zh-Hant",
	"zh-TW":   "zh-Hant",
}

// FallbackChain returns the locales consulted for a message, most specific first,
// always ending with SourceLocale: zh-Hant-TW -> zh-Hant -> en, ja-JP -> ja -> en.
func FallbackChain(locale string) []string {
	var chain []string
	for l := Canonicalize(locale); l != ""; {
		chain = append(chain, l)
		if p, ok := parentLocales[l]; ok {
			l = p
			continue
		}
		i := strings.LastIndexByte(l, '-')
		if i < 0 {
			break
		}
		l = l[:i]
	}
	if !slices.Contains(chain, SourceLocale) {
		chain = append(chain, SourceLocale)
	}
	return chain
}

// Canonicalize normalizes the case and separators of a BCP 47 tag: "zh_hant_tw" -> "zh-Hant-TW".
// It returns "" for an empty or malformed tag.
func Canonicalize(tag string) string {
	tag = strings.TrimSpace(strings.ReplaceAll(tag, "_", "-"))
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, "-")
	for i, p := range parts {
		if p == "" || len(p) > 8 || strings.IndexFunc(p, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
		}) >= 0 {
			return ""
		}
		switch {
		case i == 0:
			parts[i] = strings.ToLower(p)
		case len(p) == 4 && !isDigit(p[0]):
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		case len(p) == 2 || (len(p) == 3 && isDigit(p[0])):
			parts[i] = strings.ToUpper(p)
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "-")
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// ParseAcceptLanguage returns the language tags of an Accept-Language header value,
// highest quality first (ties keep header order). Tags with q=0, the wildcard "*",
// and malformed entries are dropped.
func ParseAcceptLanguage(header string) []string {
	type weighted struct {
		tag string
		q   float64
	}
	var tags []weighted
	for entry := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(entry, ";")
		tag = Canonicalize(tag)
		if tag == "" || tag == "*" {
			continue
		}
		q := 1.0
		for p := range strings.SplitSeq(params, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
			if !ok || strings.TrimSpace(k) != "q" {
				continue
			}
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || f < 0 || f > 1 {
				f = 0
			}
			q = f
		}
		if q > 0 {
			tags = append(tags, weighted{tag, q})
		}
	}
	slices.SortStableFunc(tags, func(a, b weighted) int { return cmp.Compare(b.q, a.q) })
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.tag
	}
	return out
}

// rtlLanguages are the languages written right to left by default.
var rtlLanguages = map[string]bool{
	"ar": true, "ckb": true, "dv": true, "fa": true, "he": true, "ps": true,
	"sd": true, "ug": true, "ur": true, "yi": true,
}

// Direction returns "rtl" or "ltr" for locale, for the root element's dir attribute.
func Direction(locale string) string {
	locale = Canonicalize(locale)
	lang, rest, _ := strings.Cut(locale, "-")
	switch {
	case locale == PseudoLocaleRTL:
		return "rtl"
	case strings.HasPrefix(rest, "Arab") || strings.HasPrefix(rest, "Hebr"):
		return "rtl"
	case rtlLanguages[lang] && !strings.HasPrefix(rest, "Latn"):
		return "rtl"
	}
	return "ltr"
}
