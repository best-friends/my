// SPDX-License-Identifier: AGPL-3.0-only

package instance_test

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/domain/instance"
)

func problemsOf(t *testing.T, err error) []instance.ThemeProblem {
	t.Helper()
	var te *instance.ThemeError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want a *ThemeError", err)
	}
	if !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("err does not wrap ErrInvalidValue")
	}
	return te.Problems
}

func TestThemeDefaultsMeetTheirContrastPairs(t *testing.T) {
	// The generated defaults are rounded to #rrggbb, which must not take a pair below its minimum.
	if _, err := instance.NormalizeTheme(nil, ""); err != nil {
		t.Fatalf("the defaults: %v", err)
	}
	for _, p := range instance.ThemeContrastPairs() {
		for _, name := range []string{p.FG, p.BG} {
			if tok, ok := instance.LookupThemeToken(name); !ok || tok.Kind != instance.ThemeTokenColor {
				t.Errorf("pair %s on %s names %s, which is not a color token", p.FG, p.BG, name)
			}
		}
	}
}

func TestContrastRatio(t *testing.T) {
	for _, tt := range []struct {
		fg, bg string
		want   float64
	}{
		{"#000000", "#ffffff", 21},
		{"#ffffff", "#ffffff", 1},
		{"#777777", "#ffffff", 4.48},
		// Half-transparent black over white is the gray #7f7f7f (gamma-space compositing).
		{"#00000080", "#ffffff", 4.00},
	} {
		if got := instance.ContrastRatio(tt.fg, tt.bg); math.Abs(got-tt.want) > 0.01 {
			t.Errorf("ContrastRatio(%s, %s) = %.3f, want %.2f", tt.fg, tt.bg, got, tt.want)
		}
	}
}

func TestNormalizeThemeCanonicalizes(t *testing.T) {
	got, err := instance.NormalizeTheme([]instance.ThemeTokenValue{
		{Token: "radius-md", Value: " 4px "},
		{Token: "color-accent", Light: "#1447E6", Dark: "#92C5FE"},
		{Token: "elevation-md", Value: "0  2px 4px  #00000033 ,inset 0 0 0 1px #000000"},
		{Token: "font-sans", Value: `"Noto Sans JP",  sans-serif`},
		{Token: "ease-standard", Value: "cubic-bezier(0.4, 0, 0.2, 1)"},
		{Token: "transition-duration-fast", Value: "0.05s"},
		{Token: "text-base--line-height", Value: "1.8"}, //nolint:gosec // a design token, not a credential
	}, "a { color: red }")
	if err != nil {
		t.Fatal(err)
	}
	want := []instance.ThemeTokenValue{
		{Token: "color-accent", Light: "#1447e6", Dark: "#92c5fe"},
		{Token: "font-sans", Value: `"Noto Sans JP", sans-serif`},
		{Token: "text-base--line-height", Value: "1.8"}, //nolint:gosec // a design token, not a credential
		{Token: "radius-md", Value: "4px"},
		{Token: "elevation-md", Value: "0 2px 4px #00000033, inset 0 0 0 1px #000000"},
		{Token: "ease-standard", Value: "cubic-bezier(0.4, 0, 0.2, 1)"},
		{Token: "transition-duration-fast", Value: "0.05s"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestNormalizeThemeRejects(t *testing.T) {
	for _, tt := range []struct {
		name  string
		token instance.ThemeTokenValue
		code  instance.ThemeProblemCode
	}{
		{"unknown token", instance.ThemeTokenValue{Token: "palette-blue-600", Value: "#000000"}, instance.ThemeUnknownToken},
		{"shadow alias", instance.ThemeTokenValue{Token: "shadow-md", Value: "none"}, instance.ThemeUnknownToken},
		{"named color", instance.ThemeTokenValue{Token: "color-accent", Light: "red", Dark: "#000000"}, instance.ThemeInvalidValue},
		{"missing dark", instance.ThemeTokenValue{Token: "color-accent", Light: "#155dfc"}, instance.ThemeInvalidValue},
		{"alpha on opaque token", instance.ThemeTokenValue{Token: "color-surface", Light: "#ffffff80", Dark: "#000000"}, instance.ThemeInvalidValue},
		{"injection in color", instance.ThemeTokenValue{Token: "color-accent", Light: "#155dfc;}", Dark: "#155dfc"}, instance.ThemeInvalidValue},
		{"length unit", instance.ThemeTokenValue{Token: "radius-md", Value: "1em"}, instance.ThemeInvalidValue},
		{"length injection", instance.ThemeTokenValue{Token: "radius-md", Value: "1px;} body{display:none"}, instance.ThemeInvalidValue},
		{"radius too large", instance.ThemeTokenValue{Token: "radius-md", Value: "40px"}, instance.ThemeOutOfRange},
		{"spacing too small", instance.ThemeTokenValue{Token: "spacing", Value: "0"}, instance.ThemeOutOfRange},
		{"text too small", instance.ThemeTokenValue{Token: "text-xs", Value: "8px"}, instance.ThemeOutOfRange},
		{"line height", instance.ThemeTokenValue{Token: "text-sm--line-height", Value: "3"}, instance.ThemeOutOfRange}, //nolint:gosec // a design token, not a credential
		{"duration", instance.ThemeTokenValue{Token: "transition-duration-slow", Value: "2s"}, instance.ThemeOutOfRange},
		{"easing", instance.ThemeTokenValue{Token: "ease-exit", Value: "cubic-bezier(2, 0, 0, 1)"}, instance.ThemeInvalidValue},
		{"steps easing", instance.ThemeTokenValue{Token: "ease-exit", Value: "steps(4)"}, instance.ThemeInvalidValue},
		{"font url", instance.ThemeTokenValue{Token: "font-sans", Value: `url(https://example.com/f.woff)`}, instance.ThemeInvalidValue},
		{"font comment", instance.ThemeTokenValue{Token: "font-sans", Value: `a /* b */`}, instance.ThemeInvalidValue},
		{"font escape", instance.ThemeTokenValue{Token: "font-sans", Value: `"a\"; }"`}, instance.ThemeInvalidValue},
		{"empty font", instance.ThemeTokenValue{Token: "font-mono", Value: ""}, instance.ThemeInvalidValue},
		{"shadow named color", instance.ThemeTokenValue{Token: "elevation-sm", Value: "0 1px 2px black"}, instance.ThemeInvalidValue},
		{"shadow too far", instance.ThemeTokenValue{Token: "elevation-sm", Value: "0 100px 2px #000000"}, instance.ThemeInvalidValue},
		{"shadow layers", instance.ThemeTokenValue{Token: "elevation-sm", Value: strings.Repeat("0 1px 1px #000000, ", 4) + "0 1px 1px #000000"}, instance.ThemeInvalidValue},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := instance.NormalizeTheme([]instance.ThemeTokenValue{tt.token}, "")
			ps := problemsOf(t, err)
			if len(ps) == 0 || ps[0].Code != tt.code || ps[0].Token != tt.token.Token {
				t.Errorf("problems = %+v, want %s for %s", ps, tt.code, tt.token.Token)
			}
		})
	}
}

func TestNormalizeThemeRejectsDuplicates(t *testing.T) {
	v := instance.ThemeTokenValue{Token: "radius-md", Value: "4px"}
	ps := problemsOf(t, func() error { _, err := instance.NormalizeTheme([]instance.ThemeTokenValue{v, v}, ""); return err }())
	if len(ps) != 1 || ps[0].Code != instance.ThemeDuplicateToken {
		t.Errorf("problems = %+v", ps)
	}
}

func TestNormalizeThemeChecksContrastWithDefaults(t *testing.T) {
	// A light accent fails on the default surfaces and under the default accent-fg (white), in the
	// light scheme only.
	_, err := instance.NormalizeTheme([]instance.ThemeTokenValue{{Token: "color-accent", Light: "#a0c4ff", Dark: "#57a2fe"}}, "")
	ps := problemsOf(t, err)
	var pairs []string
	for _, p := range ps {
		if p.Code != instance.ThemeLowContrast || p.Scheme != instance.SchemeLight || p.Ratio >= p.Minimum {
			t.Errorf("unexpected problem %+v", p)
		}
		pairs = append(pairs, p.Token+"/"+p.Background)
	}
	for _, want := range []string{"color-accent/color-surface", "color-accent-fg/color-accent"} {
		if !slices.Contains(pairs, want) {
			t.Errorf("pairs %v lack %s", pairs, want)
		}
	}
	// Changing accent-fg along with it fixes the pair that the accent is the background of; the pair
	// with the default accent-hover (dark blue) then fails instead.
	_, err = instance.NormalizeTheme([]instance.ThemeTokenValue{
		{Token: "color-accent", Light: "#a0c4ff", Dark: "#57a2fe"},
		{Token: "color-accent-fg", Light: "#000000", Dark: "#060d1a"}, //nolint:gosec // a design token, not a credential
	}, "")
	for _, p := range problemsOf(t, err) {
		if p.Token == "color-accent-fg" && p.Background == "color-accent" {
			t.Errorf("accent-fg on accent still fails: %+v", p)
		}
	}
}

func TestNormalizeThemeChecksCustomCSS(t *testing.T) {
	if _, err := instance.NormalizeTheme(nil, strings.Repeat("a", instance.MaxCustomCSSBytes)); err != nil {
		t.Errorf("CSS at the limit: %v", err)
	}
	for css, code := range map[string]instance.ThemeProblemCode{
		strings.Repeat("a", instance.MaxCustomCSSBytes+1): instance.ThemeCSSTooLong,
		"a\x00":   instance.ThemeCSSInvalid,
		"a\xff{}": instance.ThemeCSSInvalid,
	} {
		ps := problemsOf(t, func() error { _, err := instance.NormalizeTheme(nil, css); return err }())
		if len(ps) != 1 || ps[0].Code != code {
			t.Errorf("problems = %+v, want %s", ps, code)
		}
	}
}

func TestThemeStylesheet(t *testing.T) {
	r := instance.ThemeRevision{
		ID: "rev1",
		Tokens: []instance.ThemeTokenValue{
			{Token: "color-surface", Light: "#fffdf5", Dark: "#101010"},
			{Token: "radius-md", Value: "2px"},
		},
		CustomCSS: ".x { color: red }",
	}
	want := "/* The theme of this server (revision rev1). */\n" +
		":root {\n  --color-surface: light-dark(#fffdf5, #101010);\n  --radius-md: 2px;\n}\n" +
		".x { color: red }\n"
	if got := string(r.Stylesheet()); got != want {
		t.Errorf("Stylesheet() =\n%s\nwant\n%s", got, want)
	}
	if got := string((instance.ThemeRevision{ID: "r", CustomCSS: "a{}\n"}).Stylesheet()); strings.Contains(got, ":root") {
		t.Errorf("a theme without tokens has a :root rule: %s", got)
	}
}

func TestThemeTokenRows(t *testing.T) {
	values := []instance.ThemeTokenValue{
		{Token: "radius-md", Value: "2px"},
		{Token: "color-fg", Light: "#000000", Dark: "#ffffff"},
	}
	var rows []instance.ThemeTokenRow
	for _, v := range values {
		rows = append(rows, v.Rows()...)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	got := instance.ThemeTokenValuesFromRows(rows)
	// Back in the order of the tokens: colors come first in theme.css.
	if !slices.Equal(got, []instance.ThemeTokenValue{values[1], values[0]}) {
		t.Errorf("ThemeTokenValuesFromRows = %+v", got)
	}
}

func TestThemeColor(t *testing.T) {
	tokens := []instance.ThemeTokenValue{{Token: "color-surface", Light: "#fffdf5", Dark: "#101010"}}
	if got := instance.ThemeColor(tokens, "color-surface", instance.SchemeDark); got != "#101010" {
		t.Errorf("overridden = %s", got)
	}
	def, _ := instance.LookupThemeToken("color-fg")
	if got := instance.ThemeColor(tokens, "color-fg", instance.SchemeLight); got != def.Light {
		t.Errorf("default = %s, want %s", got, def.Light)
	}
	if got := instance.ThemeColor(tokens, "radius-md", instance.SchemeLight); got != "" {
		t.Errorf("non-color = %q", got)
	}
}
