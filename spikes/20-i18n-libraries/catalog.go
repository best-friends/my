// SPDX-License-Identifier: AGPL-3.0-only

// Package i18nspike is throwaway spike code for issue #20. It must never be
// imported by product code.
package i18nspike

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Locales embeds the shared catalog. Product code would embed the repository
// root `locales/` directory the same way (ADR-0009).
//
//go:embed locales
var Locales embed.FS

// Entry is one message in the Format.JS JSON shape (`{"key": {"defaultMessage", "description"}}`),
// which Weblate reads natively and which enables its ICU MessageFormat checks.
type Entry struct {
	DefaultMessage string `json:"defaultMessage"`
	Description    string `json:"description,omitempty"`
}

// Catalog maps locale -> namespace -> key -> entry.
type Catalog map[string]map[string]map[string]Entry

// LoadCatalog reads every `locales/<BCP47>/<namespace>.json` file in fsys.
func LoadCatalog(fsys fs.FS) (Catalog, error) {
	c := Catalog{}
	err := fs.WalkDir(fsys, "locales", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".json" {
			return err
		}
		parts := strings.Split(p, "/") // locales/<locale>/<ns>.json
		if len(parts) != 3 {
			return fmt.Errorf("unexpected catalog path %q", p)
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		var ns map[string]Entry
		if err := json.Unmarshal(b, &ns); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		locale, name := parts[1], strings.TrimSuffix(parts[2], ".json")
		if c[locale] == nil {
			c[locale] = map[string]map[string]Entry{}
		}
		c[locale][name] = ns
		return nil
	})
	return c, err
}

// FormattingLocale returns the locale whose CLDR plural and number rules apply.
// Pseudo-locales are generated from `en` and keep its plural categories, so
// they must be formatted with `en` rules (the TS side does the same).
func FormattingLocale(locale string) string {
	switch locale {
	case "en-XA", "ar-XB":
		return "en"
	}
	return locale
}
