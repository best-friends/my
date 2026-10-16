// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/instance"
)

// InstanceHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, and the instance repository on it.
type InstanceHarness struct {
	UnitOfWork uow.UnitOfWork
	Instance   appinstance.Repository
}

// InstanceFactory returns a harness on a new, empty database whose UnitOfWork uses opts.
// The factory registers the cleanup of everything it creates with t.Cleanup.
type InstanceFactory func(t *testing.T, opts uow.Options) InstanceHarness

// InstanceRepositoryContract verifies the instance repository of a database adapter through the
// use cases that drive it: defaults for unset settings, every change kept as a revision,
// canonical values, rejection of invalid input without writing, localized texts with their
// history, lookups through the locale fallback chain, and the property that the synchronous
// projections equal a rebuild from the revisions.
func InstanceRepositoryContract(t *testing.T, newHarness InstanceFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *instanceEnv)
	}{
		{"ReadsDefaultsWhenUnset", instanceReadsDefaults},
		{"KeepsSettingRevisions", instanceKeepsSettingRevisions},
		{"StoresCanonicalValues", instanceStoresCanonicalValues},
		{"RejectsInvalidSettings", instanceRejectsInvalidSettings},
		{"KeepsTextRevisions", instanceKeepsTextRevisions},
		{"LooksUpTextsThroughTheFallbackChain", instanceLooksUpTextsThroughFallback},
		{"RejectsInvalidTexts", instanceRejectsInvalidTexts},
		{"IncrementalProjectionEqualsRebuild", instanceProjectionEqualsRebuild},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newInstanceEnv(t, newHarness)) })
	}
}

type instanceEnv struct {
	h       InstanceHarness
	clock   *Clock
	service *appinstance.Service
}

func newInstanceEnv(t *testing.T, newHarness InstanceFactory) *instanceEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	appinstance.RegisterProjectors(projectors, h.Instance)
	clock := NewClock(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	return &instanceEnv{
		h:     h,
		clock: clock,
		service: appinstance.NewService(appinstance.Deps{
			UnitOfWork: h.UnitOfWork, Repository: h.Instance, Clock: clock, IDs: NewIDGenerator(clock),
		}),
	}
}

func (e *instanceEnv) set(t *testing.T, key instance.SettingKey, value, by string) instance.SettingRevision {
	t.Helper()
	e.clock.Advance(time.Second)
	rev, err := e.service.SetSetting(t.Context(), appinstance.SetSettingInput{Key: key, Value: value, RevisedByUserID: by})
	if err != nil {
		t.Fatalf("SetSetting(%s, %q) = %v", key, value, err)
	}
	return rev
}

func (e *instanceEnv) setText(t *testing.T, key instance.TextKey, locale, body string) instance.TextRevision {
	t.Helper()
	e.clock.Advance(time.Second)
	rev, err := e.service.SetText(t.Context(), appinstance.SetTextInput{Key: key, Locale: locale, Body: body})
	if err != nil {
		t.Fatalf("SetText(%s, %s, %q) = %v", key, locale, body, err)
	}
	return rev
}

func (e *instanceEnv) settings(t *testing.T) instance.Settings {
	t.Helper()
	s, err := e.service.Settings(t.Context())
	if err != nil {
		t.Fatalf("Settings = %v", err)
	}
	return s
}

func instanceReadsDefaults(t *testing.T, e *instanceEnv) {
	s := e.settings(t)
	for _, d := range instance.Definitions() {
		if v, stored := s.Value(d.Key); v != d.Default || stored {
			t.Errorf("Value(%s) = %q, %v; want the default %q, false", d.Key, v, stored, d.Default)
		}
	}
	if s.RegistrationMode() != instance.RegistrationApproval || s.MaxStatusLength() != 500 || s.DefaultLocale() != "en" ||
		!s.LocalTimelineEnabled() || !s.FederatedTimelineEnabled() {
		t.Errorf("typed defaults = %v %d %q %v %v", s.RegistrationMode(), s.MaxStatusLength(), s.DefaultLocale(),
			s.LocalTimelineEnabled(), s.FederatedTimelineEnabled())
	}
	texts, err := e.service.Texts(t.Context())
	if err != nil || len(texts) != 0 {
		t.Errorf("Texts = %+v, %v; want none", texts, err)
	}
	if _, ok, err := e.service.Text(t.Context(), instance.TextName, "en"); ok || err != nil {
		t.Errorf("Text(name) = _, %v, %v; want not found", ok, err)
	}
}

func instanceKeepsSettingRevisions(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	first := e.set(t, instance.SettingRegistrationMode, "open", "")
	second := e.set(t, instance.SettingRegistrationMode, "closed", "0199f0e0-0000-7000-8000-000000000001")
	e.set(t, instance.SettingMaxStatusLength, "1000", "")

	s := e.settings(t)
	if v, stored := s.Value(instance.SettingRegistrationMode); v != "closed" || !stored {
		t.Errorf("registration_mode = %q, %v; want the latest value closed", v, stored)
	}
	if s.MaxStatusLength() != 1000 {
		t.Errorf("MaxStatusLength = %d, want 1000", s.MaxStatusLength())
	}
	history, err := e.service.SettingHistory(ctx, instance.SettingRegistrationMode, 10)
	if err != nil {
		t.Fatalf("SettingHistory = %v", err)
	}
	if want := []instance.SettingRevision{second, first}; !reflect.DeepEqual(history, want) {
		t.Errorf("history = %+v\nwant      %+v", history, want)
	}
	if one, err := e.service.SettingHistory(ctx, instance.SettingRegistrationMode, 1); err != nil || len(one) != 1 || one[0].ID != second.ID {
		t.Errorf("SettingHistory(limit 1) = %+v, %v; want the newest revision", one, err)
	}
	if _, err := e.service.SettingHistory(ctx, instance.SettingRegistrationMode, 0); !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("SettingHistory(limit 0) = %v, want ErrInvalidValue", err)
	}
}

func instanceStoresCanonicalValues(t *testing.T, e *instanceEnv) {
	for _, tc := range []struct {
		key       instance.SettingKey
		raw, want string
	}{
		{instance.SettingLocalTimelineEnabled, "FALSE", "false"},
		{instance.SettingFederatedTimelineEnabled, "True", "true"},
		{instance.SettingMaxStatusLength, "0750", "750"},
		{instance.SettingDefaultLocale, "zh_hant_tw", "zh-Hant-TW"},
		{instance.SettingRegistrationMode, "invite", "invite"},
	} {
		if rev := e.set(t, tc.key, tc.raw, ""); rev.Value != tc.want {
			t.Errorf("SetSetting(%s, %q) stored %q, want %q", tc.key, tc.raw, rev.Value, tc.want)
		}
		if v, _ := e.settings(t).Value(tc.key); v != tc.want {
			t.Errorf("Value(%s) = %q, want %q", tc.key, v, tc.want)
		}
	}
	s := e.settings(t)
	if s.LocalTimelineEnabled() || !s.FederatedTimelineEnabled() || s.MaxStatusLength() != 750 ||
		s.DefaultLocale() != "zh-Hant-TW" || s.RegistrationMode() != instance.RegistrationInvite {
		t.Errorf("typed values = %v %v %d %q %v", s.LocalTimelineEnabled(), s.FederatedTimelineEnabled(),
			s.MaxStatusLength(), s.DefaultLocale(), s.RegistrationMode())
	}
}

func instanceRejectsInvalidSettings(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	for _, tc := range []struct {
		key  instance.SettingKey
		raw  string
		want error
	}{
		{"no_such_key", "1", instance.ErrUnknownKey},
		{instance.SettingRegistrationMode, "everyone", instance.ErrInvalidValue},
		{instance.SettingMaxStatusLength, "0", instance.ErrInvalidValue},
		{instance.SettingMaxStatusLength, "100001", instance.ErrInvalidValue},
		{instance.SettingMaxStatusLength, "lots", instance.ErrInvalidValue},
		{instance.SettingLocalTimelineEnabled, "yes", instance.ErrInvalidValue},
		{instance.SettingDefaultLocale, "", instance.ErrInvalidValue},
		{instance.SettingDefaultLocale, "not a locale", instance.ErrInvalidValue},
	} {
		_, err := e.service.SetSetting(ctx, appinstance.SetSettingInput{Key: tc.key, Value: tc.raw})
		if !errors.Is(err, tc.want) {
			t.Errorf("SetSetting(%s, %q) = %v, want %v", tc.key, tc.raw, err, tc.want)
		}
	}
	cur, err := e.h.Instance.ListCurrentSettings(ctx)
	if err != nil || len(cur) != 0 {
		t.Errorf("ListCurrentSettings = %+v, %v; want nothing written", cur, err)
	}
	if _, err := e.service.SettingHistory(ctx, "no_such_key", 10); !errors.Is(err, instance.ErrUnknownKey) {
		t.Errorf("SettingHistory(unknown) = %v, want ErrUnknownKey", err)
	}
}

func instanceKeepsTextRevisions(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	first := e.setText(t, instance.TextDescription, "ja_jp", "最初の説明")
	second := e.setText(t, instance.TextDescription, "ja-JP", "二番目の説明\n改行あり")
	e.setText(t, instance.TextDescription, "en", "English description")
	e.setText(t, instance.TextName, "en", "Example Social")

	if first.Locale != "ja-JP" {
		t.Errorf("stored locale = %q, want the canonical ja-JP", first.Locale)
	}
	history, err := e.service.TextHistory(ctx, instance.TextDescription, "ja-jp", 10)
	if err != nil {
		t.Fatalf("TextHistory = %v", err)
	}
	if want := []instance.TextRevision{second, first}; !reflect.DeepEqual(history, want) {
		t.Errorf("history = %+v\nwant      %+v", history, want)
	}
	texts, err := e.service.Texts(ctx)
	if err != nil {
		t.Fatalf("Texts = %v", err)
	}
	var got []string
	for _, x := range texts {
		got = append(got, fmt.Sprintf("%s/%s=%s", x.Key, x.Locale, x.Body))
	}
	want := []string{"description/en=English description", "description/ja-JP=二番目の説明\n改行あり", "name/en=Example Social"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Texts = %q, want %q", got, want)
	}
	if texts[1].RevisionID != second.ID || !texts[1].RevisedAt.Equal(second.RevisedAt) {
		t.Errorf("current ja-JP text = %+v, want revision %+v", texts[1], second)
	}
}

func instanceLooksUpTextsThroughFallback(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	e.setText(t, instance.TextName, "en", "Example")
	e.setText(t, instance.TextName, "ja", "例")
	e.setText(t, instance.TextName, "zh-Hant", "範例")

	lookup := func(locale string) string {
		t.Helper()
		x, ok, err := e.service.Text(ctx, instance.TextName, locale)
		if err != nil {
			t.Fatalf("Text(%q) = %v", locale, err)
		}
		if !ok {
			return "<none>"
		}
		return x.Locale + ":" + x.Body
	}
	check := func(locale, want string) {
		t.Helper()
		if got := lookup(locale); got != want {
			t.Errorf("Text(name, %q) = %q, want %q", locale, got, want)
		}
	}
	check("ja-JP", "ja:例")
	check("ja", "ja:例")
	check("zh-TW", "zh-Hant:範例")
	check("zh", "en:Example")
	check("fr", "en:Example")
	check("", "en:Example")
	check("en-GB", "en:Example")

	// The instance's default locale comes before the source locale.
	e.set(t, instance.SettingDefaultLocale, "ja", "")
	check("fr", "ja:例")
	check("en-GB", "en:Example")

	// A cleared text falls back to the next locale of the chain.
	e.setText(t, instance.TextName, "ja", "")
	check("ja-JP", "en:Example")
	check("fr", "en:Example")
	e.setText(t, instance.TextName, "en", "")
	check("fr", "<none>")
	check("zh-HK", "zh-Hant:範例")
}

func instanceRejectsInvalidTexts(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	for _, tc := range []struct {
		in   appinstance.SetTextInput
		want error
	}{
		{appinstance.SetTextInput{Key: "motto", Locale: "en", Body: "x"}, instance.ErrUnknownKey},
		{appinstance.SetTextInput{Key: instance.TextName, Locale: "", Body: "x"}, instance.ErrInvalidValue},
		{appinstance.SetTextInput{Key: instance.TextName, Locale: "e n", Body: "x"}, instance.ErrInvalidValue},
		{appinstance.SetTextInput{Key: instance.TextName, Locale: "en", Body: "two\nlines"}, instance.ErrInvalidValue},
		{appinstance.SetTextInput{Key: instance.TextName, Locale: "en", Body: strings.Repeat("a", 101)}, instance.ErrInvalidValue},
		{appinstance.SetTextInput{Key: instance.TextName, Locale: "en", Body: "\xff"}, instance.ErrInvalidValue},
	} {
		if _, err := e.service.SetText(ctx, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("SetText(%+v) = %v, want %v", tc.in, err, tc.want)
		}
	}
	all, err := e.h.Instance.ListAllCurrentTexts(ctx)
	if err != nil || len(all) != 0 {
		t.Errorf("ListAllCurrentTexts = %+v, %v; want nothing written", all, err)
	}
	// The longest allowed name is accepted, counted in characters.
	e.setText(t, instance.TextName, "ja", strings.Repeat("あ", 100))
}

func instanceProjectionEqualsRebuild(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	rng := rand.New(rand.NewPCG(98, 20261016)) //nolint:gosec // deterministic test input, not a secret
	defs := instance.Definitions()
	texts := instance.TextDefinitions()
	locales := []string{"en", "ja", "ja-JP", "zh-Hant", "fr"}
	values := func(d instance.Definition) string {
		switch d.Kind {
		case instance.KindBool:
			return fmt.Sprint(rng.IntN(2) == 0)
		case instance.KindInt:
			return fmt.Sprint(d.Min + rng.IntN(1000))
		case instance.KindEnum:
			return d.Choices[rng.IntN(len(d.Choices))]
		default:
			return locales[rng.IntN(len(locales))]
		}
	}
	for round := range 3 {
		for step := range 40 {
			if rng.IntN(2) == 0 {
				d := defs[rng.IntN(len(defs))]
				e.set(t, d.Key, values(d), "")
			} else {
				body := ""
				if rng.IntN(4) > 0 {
					body = fmt.Sprintf("text %d.%d", round, step)
				}
				e.setText(t, texts[rng.IntN(len(texts))].Key, locales[rng.IntN(len(locales))], body)
			}
		}
		incremental := snapshotInstanceProjections(t, e.h.Instance)
		if len(incremental.settings) == 0 || len(incremental.texts) == 0 {
			t.Fatalf("round %d: projections are empty: %+v", round, incremental)
		}
		if err := appinstance.RebuildProjections(ctx, e.h.UnitOfWork, e.h.Instance); err != nil {
			t.Fatalf("RebuildProjections = %v", err)
		}
		rebuilt := snapshotInstanceProjections(t, e.h.Instance)
		if !reflect.DeepEqual(incremental, rebuilt) {
			t.Fatalf("round %d: incremental projections differ from a rebuild:\nincremental %+v\nrebuilt     %+v", round, incremental, rebuilt)
		}
	}
}

type instanceProjections struct {
	settings []instance.CurrentSetting
	texts    []instance.Text
}

func snapshotInstanceProjections(t *testing.T, repo appinstance.Repository) instanceProjections {
	t.Helper()
	settings, err := repo.ListCurrentSettings(t.Context())
	if err != nil {
		t.Fatalf("ListCurrentSettings = %v", err)
	}
	texts, err := repo.ListAllCurrentTexts(t.Context())
	if err != nil {
		t.Fatalf("ListAllCurrentTexts = %v", err)
	}
	return instanceProjections{settings: settings, texts: texts}
}
