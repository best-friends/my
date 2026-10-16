// SPDX-License-Identifier: AGPL-3.0-only

package i18nspike

import (
	"fmt"
	"strings"

	"github.com/fullpipe/icu-mf/message"
	"github.com/fullpipe/icu-mf/parse"
	gotnospirit "github.com/gotnospirit/messageformat"
	"github.com/kaptinlin/messageformat-go/mf1"
	"golang.org/x/text/language"
)

// Engine formats one ICU MessageFormat (MF1) source string.
type Engine interface {
	Name() string
	Format(locale, source string, args map[string]any) (string, error)
}

// Engines lists every Go library compared by the spike.
var Engines = []Engine{Kaptinlin{}, Gotnospirit{}, Fullpipe{}}

// Kaptinlin uses github.com/kaptinlin/messageformat-go/mf1 (MIT), a port of
// messageformat.js backed by github.com/agentable/go-intl (CLDR data in Go).
type Kaptinlin struct{}

func (Kaptinlin) Name() string { return "kaptinlin/messageformat-go/mf1" }

func (Kaptinlin) Format(locale, source string, args map[string]any) (string, error) {
	// RequireAllArguments turns a missing argument into an error instead of "".
	// StrictPluralKeys (default on) rejects categories the locale does not have,
	// e.g. `one` in a Japanese plural, which doubles as a catalog lint.
	mf, err := mf1.New(locale, &mf1.MessageFormatOptions{RequireAllArguments: true})
	if err != nil {
		return "", err
	}
	msg, err := mf.Compile(source)
	if err != nil {
		return "", err
	}
	return msg.Format(args)
}

// Gotnospirit uses github.com/gotnospirit/messageformat (BSD-2-Clause, last release 2022).
type Gotnospirit struct{}

func (Gotnospirit) Name() string { return "gotnospirit/messageformat" }

func (Gotnospirit) Format(locale, source string, args map[string]any) (string, error) {
	base, _, _ := strings.Cut(locale, "-")
	p, err := gotnospirit.NewWithCulture(base)
	if err != nil {
		return "", err
	}
	mf, err := p.Parse(source)
	if err != nil {
		return "", err
	}
	return mf.FormatMap(args)
}

// Fullpipe uses github.com/fullpipe/icu-mf (MIT), built on golang.org/x/text.
type Fullpipe struct{}

func (Fullpipe) Name() string { return "fullpipe/icu-mf" }

func (Fullpipe) Format(locale, source string, args map[string]any) (string, error) {
	ast, err := parse.NewParser().ParseString("", source)
	if err != nil {
		return "", err
	}
	tag, err := language.Parse(locale)
	if err != nil {
		return "", err
	}
	ev, err := message.Build(*ast, tag)
	if err != nil {
		return "", err
	}
	ctx := message.Context{}
	for k, v := range args {
		// icu-mf wants integers for plural selectors.
		if f, ok := v.(float64); ok && f == float64(int64(f)) {
			v = int64(f)
		}
		ctx.Set(k, v)
	}
	out, err := ev.Eval(ctx)
	if err != nil {
		return "", fmt.Errorf("eval: %w", err)
	}
	return out, nil
}
