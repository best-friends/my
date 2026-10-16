// SPDX-License-Identifier: AGPL-3.0-only

package i18nspike

import (
	"fmt"
	"strings"

	"github.com/kaptinlin/messageformat-go/mf1"
)

// Bundle holds every catalog message precompiled with the recommended engine.
// Compiling at startup turns catalog mistakes into a boot-time (or test-time) error.
type Bundle struct {
	messages map[string]map[string]*mf1.CompiledMessage // locale -> "ns:key"
}

// NewBundle compiles all messages of all locales.
func NewBundle(c Catalog) (*Bundle, error) {
	b := &Bundle{messages: map[string]map[string]*mf1.CompiledMessage{}}
	for locale, namespaces := range c {
		mf, err := mf1.New(FormattingLocale(locale), &mf1.MessageFormatOptions{RequireAllArguments: true})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", locale, err)
		}
		b.messages[locale] = map[string]*mf1.CompiledMessage{}
		for ns, entries := range namespaces {
			for key, e := range entries {
				msg, err := mf.Compile(e.DefaultMessage)
				if err != nil {
					return nil, fmt.Errorf("%s/%s.json %q: %w", locale, ns, key, err)
				}
				b.messages[locale][ns+":"+key] = msg
			}
		}
	}
	return b, nil
}

// Localizer formats messages for one resolved locale with a fallback chain.
type Localizer struct {
	b     *Bundle
	chain []string
}

// Localizer returns a localizer for locale using the ADR-0009 fallback chain
// (zh-Hant-TW -> zh-Hant -> zh -> en; exception table omitted in the spike).
func (b *Bundle) Localizer(locale string) *Localizer {
	parts := strings.Split(locale, "-")
	var chain []string
	for i := len(parts); i > 0; i-- {
		chain = append(chain, strings.Join(parts[:i], "-"))
	}
	if chain[len(chain)-1] != "en" {
		chain = append(chain, "en")
	}
	return &Localizer{b: b, chain: chain}
}

// Format renders id ("ns:key") with args. Generated accessors in
// messages_gen.go call this; product code should not call it directly.
func (l *Localizer) Format(id string, args map[string]any) string {
	for _, locale := range l.chain {
		msg, ok := l.b.messages[locale][id]
		if !ok {
			continue
		}
		out, err := msg.Format(args)
		if err != nil {
			return id // a real implementation would log and fall back to the next locale
		}
		return out
	}
	return id
}
