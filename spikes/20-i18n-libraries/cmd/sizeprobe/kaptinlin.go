// SPDX-License-Identifier: AGPL-3.0-only

//go:build kaptinlin

package main

import "github.com/kaptinlin/messageformat-go/mf1"

func init() {
	format = func(locale, source string, args map[string]any) (string, error) {
		mf, err := mf1.New(locale, nil)
		if err != nil {
			return "", err
		}
		msg, err := mf.Compile(source)
		if err != nil {
			return "", err
		}
		return msg.Format(args)
	}
}
