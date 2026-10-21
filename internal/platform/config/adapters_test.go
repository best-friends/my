// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"errors"
	"slices"
	"testing"
)

func TestLoadRejectsExcludedAdapters(t *testing.T) {
	saved := excludedValues
	t.Cleanup(func() { excludedValues = saved })
	excludedValues = map[string]map[string]string{}
	exclude("no_postgres", "db", "postgres")
	exclude("no_postgres", "stream", "postgres")

	if _, _, err := Load(source(minimal(), nil)); err != nil {
		t.Fatalf("Load() with db = sqlite: %v", err)
	}

	_, _, err := Load(source(with(minimal(), "MY_DB", "postgres", "MY_STREAM", "postgres"), nil))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Load() error = %v, want *ValidationError", err)
	}
	var got []string
	for _, is := range ve.Issues {
		got = append(got, is.String())
	}
	for _, want := range []string{
		`db (MY_DB): "postgres" is not in this build: the build tag no_postgres left its adapter out (docs/reference/build-tags.md)`,
		`stream (MY_STREAM): "postgres" is not in this build: the build tag no_postgres left its adapter out (docs/reference/build-tags.md)`,
	} {
		if !slices.Contains(got, want) {
			t.Errorf("issues = %q, want %q among them", got, want)
		}
	}
}

func TestExcludedError(t *testing.T) {
	got := ExcludedError("kv", "valkey", "no_valkey").Error()
	want := `kv = "valkey" is not in this build: the build tag no_valkey left its adapter out (docs/reference/build-tags.md)`
	if got != want {
		t.Errorf("ExcludedError() = %q, want %q", got, want)
	}
}
