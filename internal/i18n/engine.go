// SPDX-License-Identifier: AGPL-3.0-only

package i18n

import "github.com/kaptinlin/messageformat-go/mf1"

// compiledMessage is one ICU MessageFormat (MF1) message compiled for a locale.
// The engine behind it is an implementation detail: only this file imports it, so
// it can be replaced by a fork or our own implementation (ADR-0009). The parity
// tests against FormatJS (parity_test.go) define the behavior any replacement must keep.
type compiledMessage interface {
	format(args map[string]any) (string, error)
}

// compiler compiles messages for one formatting locale.
type compiler interface {
	compile(source string) (compiledMessage, error)
}

// newCompiler returns a compiler that formats with the CLDR rules of locale.
func newCompiler(locale string) (compiler, error) {
	mf, err := mf1.New(locale, &mf1.MessageFormatOptions{
		// A missing argument is an error instead of an empty string.
		// StrictPluralKeys (on by default) rejects plural categories the locale does not
		// have, such as `one` in Japanese.
		RequireAllArguments: true,
	})
	if err != nil {
		return nil, err
	}
	return mf1Compiler{mf: mf}, nil
}

type mf1Compiler struct{ mf *mf1.MessageFormat }

func (c mf1Compiler) compile(source string) (compiledMessage, error) {
	m, err := c.mf.Compile(source)
	if err != nil {
		return nil, err
	}
	return mf1Message{m: m}, nil
}

type mf1Message struct{ m *mf1.CompiledMessage }

func (m mf1Message) format(args map[string]any) (string, error) { return m.m.Format(args) }
