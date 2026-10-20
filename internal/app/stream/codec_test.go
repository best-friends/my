// SPDX-License-Identifier: AGPL-3.0-only

package stream

import (
	"errors"
	"strings"
	"testing"
)

func TestSignalRoundTrips(t *testing.T) {
	for _, e := range []Event{
		{Kind: KindHomeEntry, EntryID: "0199f0a8-0000-7000-8000-000000000001"},
		{Kind: KindNotifications, AccountID: "0199f0a8-0000-7000-8000-000000000002"},
		{Kind: KindAnnouncement, AnnouncementID: "0199f0a8-0000-7000-8000-000000000003"},
	} {
		b, err := encodeSignal("p1", e)
		if err != nil {
			t.Fatalf("encode %+v: %v", e, err)
		}
		origin, got, err := decodeSignal(b)
		if err != nil || origin != "p1" || got != e {
			t.Errorf("decode %s = %q, %+v, %v; want p1, %+v", b, origin, got, err, e)
		}
	}
	b, _ := encodeSignal("p1", Event{Kind: KindHomeEntry, EntryID: "s1"})
	if want := `{"v":1,"o":"p1","k":"home_entry","e":"s1"}`; string(b) != want {
		t.Errorf("payload = %s, want %s", b, want)
	}
	b, _ = encodeSignal("p1", Event{Kind: KindNotifications, AccountID: "a1"})
	if want := `{"v":1,"o":"p1","k":"notifications","a":"a1"}`; string(b) != want {
		t.Errorf("payload = %s, want %s", b, want)
	}
	b, _ = encodeSignal("p1", Event{Kind: KindAnnouncement, AnnouncementID: "n1"})
	if want := `{"v":1,"o":"p1","k":"announcement","n":"n1"}`; string(b) != want {
		t.Errorf("payload = %s, want %s", b, want)
	}
}

func TestSignalSizeLimit(t *testing.T) {
	if _, err := encodeSignal("p1", Event{Kind: KindHomeEntry, EntryID: strings.Repeat("x", MaxPayloadSize)}); err == nil {
		t.Error("a signal over MaxPayloadSize was encoded")
	}
}

func TestSignalDecodeRejects(t *testing.T) {
	for payload, want := range map[string]error{
		`{"v":2,"o":"p1","k":"home_entry","e":"s1"}`:    errUnknownVersion,
		`{"o":"p1","k":"home_entry","e":"s1"}`:          errUnknownVersion,
		`{"v":1,"o":"p1","k":"home_entry"}`:             errMalformedSignal,
		`{"v":1,"o":"p1","k":"notifications","e":"s1"}`: errMalformedSignal,
		`{"v":1,"o":"p1","k":"reaction","e":"s1"}`:      errMalformedSignal,
		`{"v":1,"o":"p1","k":"announcement","a":"a1"}`:  errMalformedSignal,
		`not json`: errMalformedSignal,
	} {
		if _, _, err := decodeSignal([]byte(payload)); !errors.Is(err, want) {
			t.Errorf("decode %s: %v, want %v", payload, err, want)
		}
	}
}
