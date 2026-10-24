// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"net/http"
	"testing"
	"time"
)

func TestMetadataDocumentTTL(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name   string
		header http.Header
		want   time.Duration
	}{
		{"none", http.Header{}, DefaultMetadataDocumentTTL},
		{"max-age", http.Header{"Cache-Control": {"public, max-age=600"}}, 10 * time.Minute},
		{"s-maxage over max-age", http.Header{"Cache-Control": {"max-age=600, s-maxage=60"}}, time.Minute},
		{"quoted", http.Header{"Cache-Control": {`max-age="120"`}}, 2 * time.Minute},
		{"capped", http.Header{"Cache-Control": {"max-age=31536000"}}, MaxMetadataDocumentTTL},
		{"huge", http.Header{"Cache-Control": {"max-age=99999999999999999999999"}}, MaxMetadataDocumentTTL},
		{"no-store", http.Header{"Cache-Control": {"max-age=600", "no-store"}}, 0},
		{"no-cache", http.Header{"Cache-Control": {"No-Cache"}}, 0},
		{"malformed max-age", http.Header{"Cache-Control": {"max-age=-1"}}, 0},
		{"expires against date", http.Header{
			"Date": {now.Add(-time.Hour).Format(http.TimeFormat)}, "Expires": {now.Format(http.TimeFormat)},
		}, time.Hour},
		{"expires against now", http.Header{"Expires": {now.Add(30 * time.Minute).Format(http.TimeFormat)}}, 30 * time.Minute},
		{"expired", http.Header{"Expires": {now.Add(-time.Minute).Format(http.TimeFormat)}}, 0},
		{"malformed expires", http.Header{"Expires": {"0"}}, 0},
		{"max-age over expires", http.Header{"Cache-Control": {"max-age=60"}, "Expires": {"0"}}, time.Minute},
	} {
		if got := metadataDocumentTTL(c.header, now); got != c.want {
			t.Errorf("%s: metadataDocumentTTL = %v, want %v", c.name, got, c.want)
		}
	}
}
