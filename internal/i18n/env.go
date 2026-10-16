// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import "strings"

// EnvLocales returns the message locale candidates of a command-line process from its
// environment, most preferred first, as BCP 47 tags. The effective POSIX locale is the
// first non-empty value of LC_ALL, LC_MESSAGES and LANG; as in GNU gettext, the
// colon-separated LANGUAGE list takes precedence over it unless that locale is C or POSIX.
// A C or POSIX locale yields no candidates. Codesets and modifiers ("ja_JP.UTF-8@x") are
// dropped. lookupEnv is normally os.LookupEnv.
func EnvLocales(lookupEnv func(string) (string, bool)) []string {
	var posix string
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v, ok := lookupEnv(name); ok && v != "" {
			posix = v
			break
		}
	}
	posix = posixToBCP47(posix)
	if posix == "" {
		return nil
	}
	var out []string
	if list, ok := lookupEnv("LANGUAGE"); ok {
		for _, v := range strings.Split(list, ":") {
			if t := posixToBCP47(v); t != "" {
				out = append(out, t)
			}
		}
	}
	return append(out, posix)
}

// EnvLocale returns the locale with a catalog that best matches EnvLocales(lookupEnv), or
// SourceLocale when none does.
func (b *Bundle) EnvLocale(lookupEnv func(string) (string, bool)) string {
	if l, ok := b.match(EnvLocales(lookupEnv)); ok {
		return l
	}
	return SourceLocale
}

// posixToBCP47 converts a POSIX locale name such as "ja_JP.UTF-8@modifier" into a
// canonical BCP 47 tag ("ja-JP"). It returns "" for C, POSIX, and malformed names.
func posixToBCP47(name string) string {
	if i := strings.IndexAny(name, ".@"); i >= 0 {
		name = name[:i]
	}
	if name == "C" || name == "POSIX" {
		return ""
	}
	return Canonicalize(name)
}
