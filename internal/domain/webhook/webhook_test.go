// SPDX-License-Identifier: AGPL-3.0-only

package webhook

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// The example of the Standard Webhooks specification.
func TestSignMatchesTheSpecification(t *testing.T) {
	// The example's secret is 24 bytes, shorter than the secrets of this server.
	secret, err := base64.StdEncoding.DecodeString("MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Unix(1614265330, 0)
	body := []byte(`{"test": 2432232314}`)
	got := Sign(secret, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts, body)
	if want := "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="; got != want {
		t.Errorf("Sign = %s, want %s", got, want)
	}
	other := make([]byte, SecretSize)
	header := SignatureHeader([][]byte{other, secret}, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts, body)
	if !Verify(secret, header, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts, body) || !Verify(other, header, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts, body) {
		t.Errorf("Verify(%s) = false under either secret", header)
	}
	if Verify(secret, header, "msg_p5jXN8AQM9LWM0D4loKWxJek", ts.Add(time.Second), body) {
		t.Error("Verify accepts another timestamp")
	}
	if _, err := ParseSecret("whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"); !errors.Is(err, ErrInvalidSecret) {
		t.Errorf("ParseSecret(short) = %v", err)
	}
	full := make([]byte, SecretSize)
	full[0] = 1
	if b, err := ParseSecret(FormatSecret(full)); err != nil || !slices.Equal(b, full) {
		t.Errorf("ParseSecret(FormatSecret) = %v, %v", b, err)
	}
}

func TestSigningSecrets(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	newest := Secret{ID: "b", CreatedAt: now.Add(-time.Hour)}
	older := Secret{ID: "a", CreatedAt: now.Add(-48 * time.Hour)}
	if got := SigningSecrets([]Secret{newest, older}, now); len(got) != 2 {
		t.Errorf("within the overlap = %v, want both", got)
	}
	if got := SigningSecrets([]Secret{newest, older}, now.Add(SecretOverlap)); len(got) != 1 || got[0].ID != "b" {
		t.Errorf("after the overlap = %v, want the newest", got)
	}
	if got := SigningSecrets(nil, now); got != nil {
		t.Errorf("none = %v", got)
	}
}

func TestSettingsValidate(t *testing.T) {
	ok := EndpointSettings{URL: " https://hooks.example.com/x?y=1 ", Description: " Chat ", Events: []EventType{EventStatusCreated, EventReportCreated}}.Normalize()
	if err := ok.Validate(); err != nil || ok.URL != "https://hooks.example.com/x?y=1" || ok.Description != "Chat" || ok.Events[0] != EventReportCreated {
		t.Errorf("Validate(%+v) = %v", ok, err)
	}
	for _, tc := range []struct {
		s    EndpointSettings
		want error
	}{
		{EndpointSettings{URL: "ftp://x.example/", Events: []EventType{EventStatusCreated}}, ErrInvalidURL},
		{EndpointSettings{URL: "https:///path", Events: []EventType{EventStatusCreated}}, ErrInvalidURL},
		{EndpointSettings{URL: "https://x.example/" + strings.Repeat("a", MaxURL), Events: []EventType{EventStatusCreated}}, ErrInvalidURL},
		{EndpointSettings{URL: "https://x.example/", Description: "a\nb", Events: []EventType{EventStatusCreated}}, ErrInvalidDescription},
		{EndpointSettings{URL: "https://x.example/"}, ErrInvalidEvents},
		{EndpointSettings{URL: "https://x.example/", Events: []EventType{EventStatusCreated, EventStatusCreated}}, ErrInvalidEvents},
		{EndpointSettings{URL: "https://x.example/", Events: []EventType{"status.deleted"}}, ErrInvalidEvents},
	} {
		if err := tc.s.Normalize().Validate(); !errors.Is(err, tc.want) {
			t.Errorf("Validate(%+v) = %v, want %v", tc.s, err, tc.want)
		}
	}
	if h := Host("https://discord.com/api/webhooks/1/token"); h != "discord.com" {
		t.Errorf("Host = %q", h)
	}
	if l := AuditLabel(EndpointSettings{URL: "https://discord.com/api/webhooks/1/token", Description: "Mods"}); l != "Mods (discord.com)" {
		t.Errorf("AuditLabel = %q", l)
	}
}

func TestMissingPermissions(t *testing.T) {
	events := []EventType{EventReportCreated, EventReportResolved, EventStatusCreated, EventCustomEmojiDeleted, EventTest}
	got := MissingPermissions(events, []Permission{PermissionManageWebhooks})
	if want := []Permission{PermissionManageCustomEmojis, PermissionManageReports}; !slices.Equal(got, want) {
		t.Errorf("MissingPermissions = %v, want %v", got, want)
	}
	if EventTest.Subscribable() {
		t.Error("webhook.test is subscribable")
	}
	for _, e := range EventTypes() {
		if !e.Subscribable() {
			t.Errorf("%s is not subscribable", e)
		}
	}
}

func TestShouldDisable(t *testing.T) {
	first := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		n    int
		at   time.Time
		want bool
	}{
		{DisableAfterFailures, first.Add(DisableAfter), true},
		{DisableAfterFailures - 1, first.Add(DisableAfter * 2), false},
		{DisableAfterFailures * 2, first.Add(DisableAfter - time.Second), false},
	} {
		f := Failures{FirstFailureAt: first, Consecutive: tc.n}
		if got := f.ShouldDisable(tc.at); got != tc.want {
			t.Errorf("ShouldDisable(%d, %v) = %v, want %v", tc.n, tc.at, got, tc.want)
		}
	}
}

func TestRetryAfterAndOutcomes(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		v    string
		want time.Duration
		ok   bool
	}{
		{"120", 2 * time.Minute, true},
		{"86400", MaxRetryAfter, true},
		{now.Add(10 * time.Minute).Format("Mon, 02 Jan 2006 15:04:05 GMT"), 10 * time.Minute, true},
		{"", 0, false},
		{"-1", 0, false},
		{"soon", 0, false},
	} {
		if got, ok := RetryAfter(tc.v, now); got != tc.want || ok != tc.ok {
			t.Errorf("RetryAfter(%q) = %v, %v; want %v, %v", tc.v, got, ok, tc.want, tc.ok)
		}
	}
	if OutcomeOf(204) != OutcomeSucceeded || OutcomeOf(302) != OutcomeHTTPError || !Gone(410) {
		t.Error("outcomes of 204, 302, or 410")
	}
	if err := (Attempt{ID: "a", DeliveryID: "d", Outcome: OutcomeTimeout, StatusCode: 500}).Validate(); !errors.Is(err, ErrInvalidAttempt) {
		t.Errorf("a timeout with a status code: %v", err)
	}
	if len(RetryDelays) != MaxAttempts-1 {
		t.Errorf("%d delays for %d attempts", len(RetryDelays), MaxAttempts)
	}
	r := DeliveryRecord{Attempts: 2}
	if r.Status() != DeliveryFailing || (DeliveryRecord{}).Status() != DeliveryPending || (DeliveryRecord{Attempts: 3, Succeeded: true}).Status() != DeliverySucceeded {
		t.Error("delivery statuses")
	}
}
