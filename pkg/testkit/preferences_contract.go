// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/domain/identity"
)

// PreferencesRepositoryContract verifies the user preferences of a database adapter's identity
// repository (data-model §4.1) through the identity use cases: the defaults written on
// registration, replacing preferences and timeline languages with canonical tags, rejecting
// invalid language tags, time zones, and visibilities without writing anything, and reporting
// users without preferences. It uses the same harness as AccountRepositoryContract.
func PreferencesRepositoryContract(t *testing.T, newHarness AccountFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *accountEnv)
	}{
		{"WritesDefaultsOnRegistration", preferencesDefaults},
		{"RejectsAnInvalidRegistrationLocale", preferencesRejectRegistrationLocale},
		{"SetsPreferences", preferencesSet},
		{"RejectsInvalidPreferences", preferencesRejectInvalid},
		{"SetsTimelineLanguages", preferencesSetTimelineLanguages},
		{"RejectsInvalidTimelineLanguages", preferencesRejectTimelineLanguages},
		{"ReportsMissingPreferences", preferencesReportMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newAccountEnv(t, newHarness)) })
	}
}

// withDefaultLocale returns an identity service on e whose instance default locale is locale.
func (e *accountEnv) withDefaultLocale(locale string) *appidentity.Service {
	return appidentity.NewService(appidentity.Deps{
		UnitOfWork:    e.h.UnitOfWork,
		Repository:    e.h.Identity,
		Clock:         e.clock,
		IDs:           e.ids,
		Passwords:     e.hasher,
		Accounts:      e.accounts,
		DefaultLocale: func(context.Context) (string, error) { return locale, nil },
	})
}

func (e *accountEnv) preferences(t *testing.T, userID string) appidentity.UserPreferences {
	t.Helper()
	p, err := e.identity.Preferences(t.Context(), userID)
	if err != nil {
		t.Fatalf("Preferences(%s) = %v", userID, err)
	}
	return p
}

func defaultPreferencesOf(userID, locale string, at time.Time) appidentity.UserPreferences {
	return appidentity.UserPreferences{
		Preferences: identity.Preferences{
			UserID:              userID,
			Locale:              locale,
			TimeZone:            "UTC",
			DefaultVisibility:   identity.VisibilityPublic,
			DefaultPostLanguage: "und",
			UpdatedAt:           at,
		},
		TimelineLanguages: []string{},
	}
}

func checkPreferences(t *testing.T, got, want appidentity.UserPreferences) {
	t.Helper()
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, want.UpdatedAt)
	}
	got.UpdatedAt, want.UpdatedAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("preferences = %+v, want %+v", got, want)
	}
}

func preferencesDefaults(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "alice")
	checkPreferences(t, e.preferences(t, reg.User.ID), defaultPreferencesOf(reg.User.ID, "en", reg.User.CreatedAt))

	e.clock.Advance(time.Second)
	bob, err := e.identity.RegisterLocalUser(ctx, appidentity.RegisterInput{
		Username: "bob", Password: "correct horse battery staple", Locale: "ja-jp",
	})
	if err != nil {
		t.Fatalf("RegisterLocalUser(locale ja-jp) = %v", err)
	}
	checkPreferences(t, e.preferences(t, bob.User.ID), defaultPreferencesOf(bob.User.ID, "ja-JP", bob.User.CreatedAt))

	ja := e.withDefaultLocale("ja")
	e.clock.Advance(time.Second)
	carol, err := ja.RegisterLocalUser(ctx, appidentity.RegisterInput{Username: "carol", Password: "correct horse battery staple"})
	if err != nil {
		t.Fatalf("RegisterLocalUser(instance default ja) = %v", err)
	}
	checkPreferences(t, e.preferences(t, carol.User.ID), defaultPreferencesOf(carol.User.ID, "ja", carol.User.CreatedAt))

	e.clock.Advance(time.Second)
	admin, err := ja.CreateAdmin(ctx, appidentity.CreateAdminInput{
		Username: "root", Email: "root@example.com", Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("CreateAdmin = %v", err)
	}
	checkPreferences(t, e.preferences(t, admin.User.ID), defaultPreferencesOf(admin.User.ID, "ja", admin.User.CreatedAt))
}

func preferencesRejectRegistrationLocale(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	for _, locale := range []string{"xx", "und", "en--US", "x-private"} {
		_, err := e.identity.RegisterLocalUser(ctx, appidentity.RegisterInput{
			Username: "alice", Password: "correct horse battery staple", Locale: locale,
		})
		if !errors.Is(err, identity.ErrInvalidLanguageTag) {
			t.Errorf("RegisterLocalUser(locale %q) = %v, want ErrInvalidLanguageTag", locale, err)
		}
	}
	// Nothing was written: the username is still free.
	e.register(t, "alice")

	_, err := e.withDefaultLocale("not a tag").RegisterLocalUser(ctx, appidentity.RegisterInput{
		Username: "bob", Password: "correct horse battery staple",
	})
	if !errors.Is(err, identity.ErrInvalidLanguageTag) {
		t.Errorf("RegisterLocalUser(invalid instance default) = %v, want ErrInvalidLanguageTag", err)
	}
}

func preferencesSet(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "alice")
	e.clock.Advance(time.Minute)
	got, err := e.identity.SetPreferences(ctx, reg.User.ID, appidentity.PreferencesInput{
		Locale:                  "zh_hant_tw",
		TimeZone:                "Asia/Tokyo",
		DefaultVisibility:       identity.VisibilityUnlisted,
		DefaultPostLanguage:     "iw",
		RequireMediaDescription: true,
	})
	if err != nil {
		t.Fatalf("SetPreferences = %v", err)
	}
	want := appidentity.UserPreferences{
		Preferences: identity.Preferences{
			UserID:                  reg.User.ID,
			Locale:                  "zh-Hant-TW",
			TimeZone:                "Asia/Tokyo",
			DefaultVisibility:       identity.VisibilityUnlisted,
			DefaultPostLanguage:     "he",
			RequireMediaDescription: true,
			UpdatedAt:               e.clock.Now(),
		},
		TimelineLanguages: []string{},
	}
	checkPreferences(t, appidentity.UserPreferences{Preferences: got, TimelineLanguages: []string{}}, want)
	checkPreferences(t, e.preferences(t, reg.User.ID), want)

	// Every visibility and a return to the defaults are accepted.
	for _, v := range []identity.Visibility{
		identity.VisibilityLocal, identity.VisibilityPrivate, identity.VisibilityDirect, identity.VisibilityPublic,
	} {
		e.clock.Advance(time.Second)
		_, err := e.identity.SetPreferences(ctx, reg.User.ID, appidentity.PreferencesInput{
			Locale: "en", TimeZone: "UTC", DefaultVisibility: v, DefaultPostLanguage: "und",
		})
		if err != nil {
			t.Fatalf("SetPreferences(visibility %s) = %v", v, err)
		}
		if got := e.preferences(t, reg.User.ID); got.DefaultVisibility != v {
			t.Errorf("DefaultVisibility = %s, want %s", got.DefaultVisibility, v)
		}
	}
	checkPreferences(t, e.preferences(t, reg.User.ID), defaultPreferencesOf(reg.User.ID, "en", e.clock.Now()))
}

func preferencesRejectInvalid(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "alice")
	valid := appidentity.PreferencesInput{
		Locale: "ja", TimeZone: "Asia/Tokyo", DefaultVisibility: identity.VisibilityPrivate, DefaultPostLanguage: "ja",
	}
	cases := []struct {
		name   string
		modify func(*appidentity.PreferencesInput)
		want   error
	}{
		{"unknown locale", func(in *appidentity.PreferencesInput) { in.Locale = "xx" }, identity.ErrInvalidLanguageTag},
		{"malformed locale", func(in *appidentity.PreferencesInput) { in.Locale = "ja--JP" }, identity.ErrInvalidLanguageTag},
		{"empty locale", func(in *appidentity.PreferencesInput) { in.Locale = "" }, identity.ErrInvalidLanguageTag},
		{"undetermined locale", func(in *appidentity.PreferencesInput) { in.Locale = "und" }, identity.ErrInvalidLanguageTag},
		{"private-use locale", func(in *appidentity.PreferencesInput) { in.Locale = "x-klingon" }, identity.ErrInvalidLanguageTag},
		{"unknown post language", func(in *appidentity.PreferencesInput) { in.DefaultPostLanguage = "zz-ZZ" }, identity.ErrInvalidLanguageTag},
		{"empty post language", func(in *appidentity.PreferencesInput) { in.DefaultPostLanguage = "" }, identity.ErrInvalidLanguageTag},
		{"unknown time zone", func(in *appidentity.PreferencesInput) { in.TimeZone = "Mars/Olympus_Mons" }, identity.ErrInvalidTimeZone},
		{"empty time zone", func(in *appidentity.PreferencesInput) { in.TimeZone = "" }, identity.ErrInvalidTimeZone},
		{"host time zone", func(in *appidentity.PreferencesInput) { in.TimeZone = "Local" }, identity.ErrInvalidTimeZone},
		{"path time zone", func(in *appidentity.PreferencesInput) { in.TimeZone = "../etc/passwd" }, identity.ErrInvalidTimeZone},
		{"offset time zone", func(in *appidentity.PreferencesInput) { in.TimeZone = "+09:00" }, identity.ErrInvalidTimeZone},
		{"unknown visibility", func(in *appidentity.PreferencesInput) { in.DefaultVisibility = "friends" }, identity.ErrInvalidVisibility},
		{"empty visibility", func(in *appidentity.PreferencesInput) { in.DefaultVisibility = "" }, identity.ErrInvalidVisibility},
	}
	for _, tc := range cases {
		in := valid
		tc.modify(&in)
		if _, err := e.identity.SetPreferences(ctx, reg.User.ID, in); !errors.Is(err, tc.want) {
			t.Errorf("SetPreferences(%s: %+v) = %v, want %v", tc.name, in, err, tc.want)
		}
	}
	checkPreferences(t, e.preferences(t, reg.User.ID), defaultPreferencesOf(reg.User.ID, "en", reg.User.CreatedAt))
}

func preferencesSetTimelineLanguages(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "alice")
	other := e.register(t, "bob")
	steps := []struct {
		in   []string
		want []string
	}{
		{[]string{"zh-hant", "ja-jp", "en", "ja-JP", "en-US", "und"}, []string{"en", "en-US", "ja-JP", "und", "zh-Hant"}},
		{[]string{"fr"}, []string{"fr"}},
		{nil, []string{}},
		{[]string{"ja"}, []string{"ja"}},
	}
	for _, s := range steps {
		got, err := e.identity.SetTimelineLanguages(ctx, reg.User.ID, s.in)
		if err != nil {
			t.Fatalf("SetTimelineLanguages(%v) = %v", s.in, err)
		}
		if !reflect.DeepEqual(got, s.want) {
			t.Errorf("SetTimelineLanguages(%v) = %v, want %v", s.in, got, s.want)
		}
		if stored := e.preferences(t, reg.User.ID).TimelineLanguages; !reflect.DeepEqual(stored, s.want) {
			t.Errorf("stored timeline languages after %v = %v, want %v", s.in, stored, s.want)
		}
	}
	if langs := e.preferences(t, other.User.ID).TimelineLanguages; len(langs) != 0 {
		t.Errorf("another user's timeline languages = %v, want none", langs)
	}
}

func preferencesRejectTimelineLanguages(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	reg := e.register(t, "alice")
	if _, err := e.identity.SetTimelineLanguages(ctx, reg.User.ID, []string{"en", "ja"}); err != nil {
		t.Fatalf("SetTimelineLanguages = %v", err)
	}
	for _, bad := range [][]string{{"en", "xx"}, {""}, {"x-private"}, {"en_"}} {
		if _, err := e.identity.SetTimelineLanguages(ctx, reg.User.ID, bad); !errors.Is(err, identity.ErrInvalidLanguageTag) {
			t.Errorf("SetTimelineLanguages(%q) = %v, want ErrInvalidLanguageTag", bad, err)
		}
	}
	tooMany := make([]string, identity.MaxTimelineLanguageCount+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("en-%03d", i)
	}
	if _, err := e.identity.SetTimelineLanguages(ctx, reg.User.ID, tooMany); !errors.Is(err, identity.ErrTooManyLanguages) {
		t.Errorf("SetTimelineLanguages(%d languages) = %v, want ErrTooManyLanguages", len(tooMany), err)
	}
	if langs := e.preferences(t, reg.User.ID).TimelineLanguages; !reflect.DeepEqual(langs, []string{"en", "ja"}) {
		t.Errorf("timeline languages after rejected updates = %v, want [en ja]", langs)
	}
}

func preferencesReportMissing(t *testing.T, e *accountEnv) {
	ctx := t.Context()
	missing := e.ids.NewID()
	if _, err := e.identity.Preferences(ctx, missing); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("Preferences(missing) = %v, want ErrNotFound", err)
	}
	_, err := e.identity.SetPreferences(ctx, missing, appidentity.PreferencesInput{
		Locale: "en", TimeZone: "UTC", DefaultVisibility: identity.VisibilityPublic, DefaultPostLanguage: "und",
	})
	if !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("SetPreferences(missing) = %v, want ErrNotFound", err)
	}
	if _, err := e.identity.SetTimelineLanguages(ctx, missing, []string{"en"}); !errors.Is(err, appidentity.ErrNotFound) {
		t.Errorf("SetTimelineLanguages(missing) = %v, want ErrNotFound", err)
	}
}
