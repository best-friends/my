// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestDescribe(t *testing.T) {
	declared := map[string]bool{"read": true, "read:accounts": true, "write:statuses": true}
	token := func(scopes ...string) openapi3.SecurityRequirement {
		return openapi3.SecurityRequirement{schemeName: scopes}
	}
	tests := []struct {
		name    string
		reqs    openapi3.SecurityRequirements
		want    access
		wantErr string
	}{
		{name: "anonymous", reqs: openapi3.SecurityRequirements{}, want: access{anonymous: true}},
		{name: "token", reqs: openapi3.SecurityRequirements{token("write:statuses", "read:accounts", "read:accounts")},
			want: access{scopes: []string{"read:accounts", "write:statuses"}}},
		{name: "optional", reqs: openapi3.SecurityRequirements{{}, token("read")},
			want: access{anonymous: true, scopes: []string{"read"}}},
		{name: "no scope", reqs: openapi3.SecurityRequirements{token()}, wantErr: "names its scopes"},
		{name: "undeclared scope", reqs: openapi3.SecurityRequirements{token("admin")}, wantErr: "not declared"},
		{name: "other scheme", reqs: openapi3.SecurityRequirements{{"apiKey": nil}}, wantErr: "only the oauth2 scheme"},
		{name: "two requirements", reqs: openapi3.SecurityRequirements{token("read"), token("read:accounts")}, wantErr: "more than one"},
		{name: "only empty", reqs: openapi3.SecurityRequirements{{}}, wantErr: "security: []"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := describe(tt.reqs, declared)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("describe = %+v, %v; want an error containing %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil || got.anonymous != tt.want.anonymous || !slices.Equal(got.scopes, tt.want.scopes) {
				t.Errorf("describe = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestRenderFirstPartyOnly(t *testing.T) {
	src, err := render(map[string]access{"GET /api/v1/p": {operation: "getP", scopes: []string{"read:accounts"}, firstPartyOnly: true}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{Operation: "getP", FirstPartyOnly: true, Scopes: []string{"read:accounts"}}`
	if !strings.Contains(string(src), want) {
		t.Errorf("render =\n%s\nwant %s", src, want)
	}
}

func TestFirstPartyOnlyExtension(t *testing.T) {
	if got, err := firstPartyOnly(json.RawMessage("true")); err != nil || !got {
		t.Errorf("firstPartyOnly(true) = %v, %v", got, err)
	}
	if got, err := firstPartyOnly(json.RawMessage(`"yes"`)); err == nil || got {
		t.Errorf("firstPartyOnly(non-boolean) = %v, %v; want an error", got, err)
	}
}

func TestParseDeprecation(t *testing.T) {
	got, err := parseDeprecation(json.RawMessage(`{"deprecated":"2026-11-01","sunset":"2027-02-01"}`))
	if err != nil || got.deprecated.Unix() != 1793491200 || got.sunset.Unix() != 1801440000 {
		t.Errorf("parseDeprecation = %+v, %v", got, err)
	}
	for name, raw := range map[string]string{
		"sunset before deprecated": `{"deprecated":"2027-02-01","sunset":"2026-11-01"}`,
		"not a date":               `{"deprecated":"soon","sunset":"2027-02-01"}`,
		"missing sunset":           `{"deprecated":"2026-11-01"}`,
		"unknown key":              `{"deprecated":"2026-11-01","sunset":"2027-02-01","x":1}`,
		"not an object":            `true`,
	} {
		if got, err := parseDeprecation(json.RawMessage(raw)); err == nil {
			t.Errorf("%s: parseDeprecation = %+v, want an error", name, got)
		}
	}
}

func TestRenderDeprecation(t *testing.T) {
	src, err := render(map[string]access{"GET /api/v1/p": {operation: "getP", anonymous: true,
		deprecation: &deprecation{deprecated: time.Unix(1793491200, 0), sunset: time.Unix(1801440000, 0)}}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{Operation: "getP", Anonymous: true, Deprecated: 1793491200, Sunset: 1801440000}`
	if !strings.Contains(string(src), want) {
		t.Errorf("render =\n%s\nwant %s", src, want)
	}
}

func TestRender(t *testing.T) {
	src, err := render(map[string]access{
		"GET /api/v1/b": {operation: "getB", scopes: []string{"read"}},
		"GET /api/v1/a": {operation: "getA", anonymous: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `	"GET /api/v1/a": {Operation: "getA", Anonymous: true},
	"GET /api/v1/b": {Operation: "getB", Scopes: []string{"read"}},
`
	if !strings.Contains(string(src), want) {
		t.Errorf("render =\n%s\nwant the sorted entries\n%s", src, want)
	}
}
