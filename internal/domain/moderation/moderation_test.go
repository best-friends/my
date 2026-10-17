// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMergeKeepsTheLatestEndOfEachRestriction(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { t := now.Add(d); return &t }
	rows := []RestrictionRow{
		{Restriction: RestrictFederation, SourceActionID: "a", EffectiveUntil: at(time.Hour)},
		{Restriction: RestrictFederation, SourceActionID: "b", EffectiveUntil: at(2 * time.Hour)},
		{Restriction: RestrictProfileView, SourceActionID: "a", EffectiveUntil: at(time.Hour)},
		{Restriction: RestrictProfileView, SourceActionID: "c"},
		{Restriction: RestrictProfileView, SourceActionID: "d", EffectiveUntil: at(3 * time.Hour)},
		{Restriction: RestrictMediaFetch, SourceActionID: "e", EffectiveUntil: at(0)},
	}
	got := Merge(rows, now)
	want := []Effect{
		{Restriction: RestrictFederation, Until: at(2 * time.Hour)},
		{Restriction: RestrictProfileView},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Merge = %+v, want %+v", got, want)
	}
	if got := Merge(nil, now); got == nil || len(got) != 0 {
		t.Errorf("Merge(nil) = %#v, want an empty slice", got)
	}
}

func TestDomainAndParents(t *testing.T) {
	if got, want := DomainAndParents("a.b.example"), []string{"a.b.example", "b.example", "example"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DomainAndParents = %v, want %v", got, want)
	}
	if got := DomainAndParents(""); got != nil {
		t.Errorf("DomainAndParents(\"\") = %v, want none", got)
	}
}

func TestActionsAndTheirRestrictions(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	ok := Action{ID: "a", TargetAccountID: "x", Kind: ActionSuspend, ReasonCategory: CategorySpam, Issuer: IssuerModerator, IssuedAt: now, ExpiresAt: &later}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	if rows := ok.Rows(); len(rows) != 5 || rows[0].Target != "x" || rows[0].SourceActionID != "a" || rows[0].EffectiveUntil != &later {
		t.Errorf("Rows = %+v, want the 5 restrictions of a suspension", rows)
	}
	ban := ok
	ban.Kind = ActionBan
	early := now
	past := ok
	past.ExpiresAt = &early
	for name, a := range map[string]Action{"expiring ban": ban, "expiry at issue": past} {
		if err := a.Validate(); !errors.Is(err, ErrInvalidAction) {
			t.Errorf("Validate(%s) = %v, want ErrInvalidAction", name, err)
		}
	}
	d := DomainAction{ID: "d", Domain: "bad.example", Kind: DomainSuspend, IssuedAt: now}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	if got := d.Rows(); len(got) != 2 || got[0].Restriction != RestrictProfileView || got[1].Restriction != RestrictFederation {
		t.Errorf("Rows = %+v, want profile_view and federation", got)
	}
	for _, domain := range []string{"", "Bad.example", "bad.example:443", "bad..example", "https://bad.example", "bad.example/"} {
		if ValidDomain(domain) {
			t.Errorf("ValidDomain(%q) = true", domain)
		}
	}
}

func TestTruncateComment(t *testing.T) {
	long := strings.Repeat("é", MaxReportComment+1)
	if got := TruncateComment(long); got != strings.Repeat("é", MaxReportComment) {
		t.Errorf("TruncateComment kept %d bytes", len(got))
	}
	if got := TruncateComment("short"); got != "short" {
		t.Errorf("TruncateComment(short) = %q", got)
	}
}
