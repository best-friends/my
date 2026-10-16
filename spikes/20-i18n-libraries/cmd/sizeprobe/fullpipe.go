// SPDX-License-Identifier: AGPL-3.0-only

//go:build fullpipe

package main

import (
	"github.com/fullpipe/icu-mf/message"
	"github.com/fullpipe/icu-mf/parse"
	"golang.org/x/text/language"
)

func init() {
	format = func(locale, source string, args map[string]any) (string, error) {
		ast, err := parse.NewParser().ParseString("", source)
		if err != nil {
			return "", err
		}
		ev, err := message.Build(*ast, language.MustParse(locale))
		if err != nil {
			return "", err
		}
		ctx := message.Context{}
		for k, v := range args {
			ctx.Set(k, v)
		}
		return ev.Eval(ctx)
	}
}
