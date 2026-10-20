// SPDX-License-Identifier: AGPL-3.0-only

package notification_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/best-friends/my/internal/domain/notification"
)

const (
	p256dh = "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"
	auth   = "BTBZMqHH6r4Tts7J_aSIgg"
)

func TestNormalizeAlerts(t *testing.T) {
	got, ok := notification.NormalizeAlerts([]notification.Type{notification.TypeMention, notification.TypeFollow, notification.TypeMention})
	if want := []notification.Type{notification.TypeFollow, notification.TypeMention}; !ok || !slices.Equal(got, want) {
		t.Errorf("NormalizeAlerts = %v, %v; want %v", got, ok, want)
	}
	if got, ok := notification.NormalizeAlerts(nil); !ok || len(got) != 0 {
		t.Errorf("NormalizeAlerts(nil) = %v, %v", got, ok)
	}
	if _, ok := notification.NormalizeAlerts([]notification.Type{"poke"}); ok {
		t.Error("NormalizeAlerts accepted an unknown type")
	}
}

func TestPushSubscriptionValidate(t *testing.T) {
	valid := notification.PushSubscription{
		ID: "id", AccountID: "acct", TokenFamilyID: "family", Endpoint: "https://push.example.net/send/1",
		KeyP256DH: p256dh, KeyAuth: auth, Alerts: []notification.Type{notification.TypeMention},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	cases := []struct {
		name   string
		change func(*notification.PushSubscription)
		want   error
	}{
		{"http", func(s *notification.PushSubscription) { s.Endpoint = "http://push.example.net/1" }, notification.ErrInvalidPushEndpoint},
		{"no host", func(s *notification.PushSubscription) { s.Endpoint = "https:///1" }, notification.ErrInvalidPushEndpoint},
		{"user info", func(s *notification.PushSubscription) { s.Endpoint = "https://u:p@push.example.net/1" }, notification.ErrInvalidPushEndpoint},
		{"fragment", func(s *notification.PushSubscription) { s.Endpoint = "https://push.example.net/1#x" }, notification.ErrInvalidPushEndpoint},
		{"long", func(s *notification.PushSubscription) {
			s.Endpoint = "https://push.example.net/" + strings.Repeat("a", notification.MaxPushEndpointBytes)
		}, notification.ErrInvalidPushEndpoint},
		{"compressed key", func(s *notification.PushSubscription) { s.KeyP256DH = "AyVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcx" }, notification.ErrInvalidPushKey},
		{"not on the curve", func(s *notification.PushSubscription) { s.KeyP256DH = "BA" + strings.Repeat("A", 85) }, notification.ErrInvalidPushKey},
		{"short auth", func(s *notification.PushSubscription) { s.KeyAuth = "BTBZ" }, notification.ErrInvalidPushKey},
		{"unsorted alerts", func(s *notification.PushSubscription) {
			s.Alerts = []notification.Type{notification.TypeMention, notification.TypeFollow}
		}, notification.ErrInvalidType},
	}
	for _, tc := range cases {
		s := valid
		tc.change(&s)
		if err := s.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("%s: Validate = %v, want %v", tc.name, err, tc.want)
		}
	}
	if p, a := notification.NormalizePushKeys(p256dh+"=", auth+"=="); p != p256dh || a != auth {
		t.Errorf("NormalizePushKeys = %q, %q", p, a)
	}
	if !valid.Alerted(notification.TypeMention) || valid.Alerted(notification.TypeBoost) {
		t.Error("Alerted")
	}
}
