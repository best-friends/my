// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// fakeAutoDeletes holds the policies by account, validating them as the content module does.
type fakeAutoDeletes struct {
	byAccount map[string]content.AutoDeletePolicy
}

func (f *fakeAutoDeletes) AutoDeletePolicy(_ context.Context, accountID string) (content.AutoDeletePolicy, error) {
	if p, ok := f.byAccount[accountID]; ok {
		return p, nil
	}
	return content.DefaultAutoDeletePolicy(accountID), nil
}

func (f *fakeAutoDeletes) UpdateAutoDeletePolicy(_ context.Context, accountID string, p content.AutoDeletePolicy) (content.AutoDeletePolicy, error) {
	p.AccountID = accountID
	p, err := p.Normalize()
	if err != nil {
		return content.AutoDeletePolicy{}, err
	}
	f.byAccount[accountID] = p
	return p, nil
}

func newAutoDeleteEnv(t *testing.T) (*authEnv, *fakeAutoDeletes) {
	t.Helper()
	f := &fakeAutoDeletes{byAccount: map[string]content.AutoDeletePolicy{}}
	return newAuthEnv(t, func(o *clientapi.Options) { o.AutoDelete = f }), f
}

func TestGetAutoDeletePolicy(t *testing.T) {
	e, _ := newAutoDeleteEnv(t)
	res := e.get(t, "/preferences/auto_delete", map[string]string{"Authorization": "Bearer read-accounts"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var got clientapi.AutoDeletePolicy
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want := clientapi.AutoDeletePolicy{
		MinAgeDays: content.DefaultAutoDeleteAgeDays, KeepPinned: true, KeepBookmarked: true, KeepFavourited: true,
		KeepVisibilities: []clientapi.Visibility{clientapi.VisibilityDirect},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("policy = %+v, want the default %+v", got, want)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if res := e.get(t, "/preferences/auto_delete", map[string]string{"Authorization": "Bearer write"}); res.StatusCode != http.StatusForbidden {
		t.Errorf("write token = %d, want 403", res.StatusCode)
	}
}

func TestUpdateAutoDeletePolicy(t *testing.T) {
	e, f := newAutoDeleteEnv(t)
	res := e.sendJSON(t, http.MethodPut, "/preferences/auto_delete", "write", `{
		"enabled": true, "min_age_days": 14, "keep_pinned": true, "keep_bookmarked": false, "keep_favourited": false,
		"keep_media": true, "keep_polls": false, "keep_visibilities": ["direct", "private"],
		"keep_min_boosts": 5, "keep_min_favourites": 0
	}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var got clientapi.AutoDeletePolicy
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want := clientapi.AutoDeletePolicy{
		Enabled: true, MinAgeDays: 14, KeepPinned: true, KeepMedia: true,
		KeepVisibilities: []clientapi.Visibility{clientapi.VisibilityPrivate, clientapi.VisibilityDirect}, KeepMinBoosts: 5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("policy = %+v, want %+v", got, want)
	}
	if p := f.byAccount[aliceAccountID]; !p.Enabled || p.MinAgeDays != 14 {
		t.Errorf("stored policy = %+v", p)
	}
	if res := e.sendJSON(t, http.MethodPut, "/preferences/auto_delete", "read", `{"invalid-on-purpose": true}`); res.StatusCode != http.StatusForbidden {
		t.Errorf("read token = %d, want 403", res.StatusCode)
	}

	res = e.sendJSON(t, http.MethodPut, "/preferences/auto_delete", "write", `{
		"enabled": true, "min_age_days": 7, "keep_pinned": true, "keep_bookmarked": false, "keep_favourited": false,
		"keep_media": true, "keep_polls": false, "keep_visibilities": ["invalid-on-purpose"], "keep_min_boosts": 0, "keep_min_favourites": 0
	}`)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unknown visibility: status = %d, want 422", res.StatusCode)
	}
	if p := decodeProblem(t, res); p.Detail == nil || *p.Detail != "keep_visibilities の値が正しくありません。" {
		t.Errorf("problem = %+v (detail %v), want keep_visibilities named", p, p.Detail)
	}
}
