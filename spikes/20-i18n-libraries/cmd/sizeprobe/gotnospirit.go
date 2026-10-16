// SPDX-License-Identifier: AGPL-3.0-only

//go:build gotnospirit

package main

import "github.com/gotnospirit/messageformat"

func init() {
	format = func(locale, source string, args map[string]any) (string, error) {
		p, err := messageformat.NewWithCulture(locale)
		if err != nil {
			return "", err
		}
		mf, err := p.Parse(source)
		if err != nil {
			return "", err
		}
		return mf.FormatMap(args)
	}
}
