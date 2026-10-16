// SPDX-License-Identifier: AGPL-3.0-only

package instance_test

import (
	"errors"
	"testing"

	"github.com/best-friends/my/internal/domain/instance"
)

func TestDefinitionsAreValid(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range instance.Definitions() {
		if seen[string(d.Key)] {
			t.Errorf("duplicate key %s", d.Key)
		}
		seen[string(d.Key)] = true
		if v, err := d.Parse(d.Default); err != nil || v != d.Default {
			t.Errorf("default of %s = %q parses to %q, %v; want itself", d.Key, d.Default, v, err)
		}
	}
	for _, d := range instance.TextDefinitions() {
		if seen[string(d.Key)] {
			t.Errorf("text key %s is also a setting key", d.Key)
		}
		seen[string(d.Key)] = true
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		key  instance.SettingKey
		raw  string
		want string // "" means invalid
	}{
		{instance.SettingLocalTimelineEnabled, "true", "true"},
		{instance.SettingLocalTimelineEnabled, "False", "false"},
		{instance.SettingLocalTimelineEnabled, "1", ""},
		{instance.SettingMaxStatusLength, "1", "1"},
		{instance.SettingMaxStatusLength, "+42", "42"},
		{instance.SettingMaxStatusLength, "100000", "100000"},
		{instance.SettingMaxStatusLength, "100001", ""},
		{instance.SettingMaxStatusLength, "-1", ""},
		{instance.SettingMaxStatusLength, "5e2", ""},
		{instance.SettingRegistrationMode, "closed", "closed"},
		{instance.SettingRegistrationMode, "Closed", ""},
		{instance.SettingDefaultLocale, "ja", "ja"},
		{instance.SettingDefaultLocale, "zh-Hant-TW", "zh-Hant-TW"},
		{instance.SettingDefaultLocale, "x", ""},
		{instance.SettingDefaultLocale, "1a", ""},
		{instance.SettingDefaultLocale, "en--US", ""},
		{instance.SettingDefaultLocale, "", ""},
	} {
		d, err := instance.Lookup(tc.key)
		if err != nil {
			t.Fatal(err)
		}
		got, err := d.Parse(tc.raw)
		switch {
		case tc.want == "" && !errors.Is(err, instance.ErrInvalidValue):
			t.Errorf("Parse(%s, %q) = %q, %v; want ErrInvalidValue", tc.key, tc.raw, got, err)
		case tc.want != "" && (err != nil || got != tc.want):
			t.Errorf("Parse(%s, %q) = %q, %v; want %q", tc.key, tc.raw, got, err, tc.want)
		}
	}
	if _, err := instance.Lookup("nope"); !errors.Is(err, instance.ErrUnknownKey) {
		t.Errorf("Lookup(nope) = %v, want ErrUnknownKey", err)
	}
	if _, err := instance.LookupText("nope"); !errors.Is(err, instance.ErrUnknownKey) {
		t.Errorf("LookupText(nope) = %v, want ErrUnknownKey", err)
	}
}

func TestSettingsFallBackToDefaults(t *testing.T) {
	var zero instance.Settings
	if zero.MaxStatusLength() != 500 || zero.RegistrationMode() != instance.RegistrationApproval {
		t.Errorf("zero Settings = %d, %s; want the defaults", zero.MaxStatusLength(), zero.RegistrationMode())
	}
	s := instance.NewSettings([]instance.CurrentSetting{
		{Key: instance.SettingMaxStatusLength, RevisionID: "r1", Value: "42"},
		{Key: "retired_key", RevisionID: "r2", Value: "x"},
	})
	if v, stored := s.Value(instance.SettingMaxStatusLength); v != "42" || !stored || s.MaxStatusLength() != 42 {
		t.Errorf("max_status_length = %q, %v", v, stored)
	}
	if v, stored := s.Value("retired_key"); v != "" || stored {
		t.Errorf("unknown key = %q, %v; want ignored", v, stored)
	}
}

func TestSupersedes(t *testing.T) {
	rev := instance.SettingRevision{ID: "b"}
	for _, tc := range []struct {
		cur  string
		want bool
	}{{"", true}, {"a", true}, {"b", false}, {"c", false}} {
		if got := rev.Supersedes(instance.CurrentSetting{RevisionID: tc.cur}); got != tc.want {
			t.Errorf("revision b supersedes %q = %v, want %v", tc.cur, got, tc.want)
		}
		if got := (instance.TextRevision{ID: "b"}).Supersedes(instance.CurrentText{RevisionID: tc.cur}); got != tc.want {
			t.Errorf("text revision b supersedes %q = %v, want %v", tc.cur, got, tc.want)
		}
	}
}

func TestPickText(t *testing.T) {
	texts := []instance.Text{
		{Locale: "en", Body: "Example"},
		{Locale: "ja", Body: ""},
		{Locale: "ja-JP", Body: "例"},
	}
	for _, tc := range []struct {
		chain []string
		want  string
		ok    bool
	}{
		{[]string{"ja-JP", "ja", "en"}, "例", true},
		{[]string{"ja", "en"}, "Example", true},
		{[]string{"fr"}, "", false},
		{nil, "", false},
	} {
		got, ok := instance.PickText(texts, tc.chain)
		if ok != tc.ok || got.Body != tc.want {
			t.Errorf("PickText(%q) = %q, %v; want %q, %v", tc.chain, got.Body, ok, tc.want, tc.ok)
		}
	}
}

func TestTextValidate(t *testing.T) {
	name, _ := instance.LookupText(instance.TextName)
	desc, _ := instance.LookupText(instance.TextDescription)
	for _, tc := range []struct {
		d    instance.TextDefinition
		body string
		ok   bool
	}{
		{name, "", true},
		{name, "Example", true},
		{name, "a\nb", false},
		{desc, "a\nb", true},
		{name, "\xff", false},
	} {
		if err := tc.d.Validate(tc.body); (err == nil) != tc.ok {
			t.Errorf("Validate(%s, %q) = %v, want ok %v", tc.d.Key, tc.body, err, tc.ok)
		}
	}
}

func TestParseMedia(t *testing.T) {
	d, err := instance.Lookup(instance.SettingPreviewImage)
	if err != nil {
		t.Fatal(err)
	}
	for raw, want := range map[string]string{
		"":                                     "",
		"0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f": "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f",
		"0192F3A4-7B1C-7D2E-9F00-1A2B3C4D5E6F": "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f",
	} {
		if got, err := d.Parse(raw); err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{
		"x", "0192f3a47b1c7d2e9f001a2b3c4d5e6f", "0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6g", "0192f3a4_7b1c-7d2e-9f00-1a2b3c4d5e6f",
	} {
		if got, err := d.Parse(raw); !errors.Is(err, instance.ErrInvalidValue) {
			t.Errorf("Parse(%q) = %q, %v; want ErrInvalidValue", raw, got, err)
		}
	}
	if keys := instance.MediaKeys(); len(keys) != 1 || keys[0] != instance.SettingPreviewImage {
		t.Errorf("MediaKeys() = %v", keys)
	}
	var zero instance.Settings
	if zero.PreviewImageMediaID() != "" {
		t.Errorf("default preview image = %q", zero.PreviewImageMediaID())
	}
}
