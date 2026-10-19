// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/identity"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/clientapi"
)

// Texts lists the texts of a fakeInstance by key and locale, as appinstance.Service does.
func (f *fakeInstance) Texts(context.Context) ([]instance.Text, error) {
	var out []instance.Text
	for _, d := range instance.TextDefinitions() {
		for _, locale := range slices.Sorted(maps.Keys(f.texts[d.Key])) {
			if body := f.texts[d.Key][locale]; body != "" {
				out = append(out, instance.Text{Key: d.Key, Locale: locale, Body: body, RevisedAt: adminNow})
			}
		}
	}
	return out, nil
}

// SetText changes a text of a fakeInstance with the checks of appinstance.Service, and keeps
// who changed it.
func (f *fakeInstance) SetText(_ context.Context, in appinstance.SetTextInput) (instance.TextRevision, error) {
	def, err := instance.LookupText(in.Key)
	if err != nil {
		return instance.TextRevision{}, err
	}
	locale := i18n.Canonicalize(in.Locale)
	if err := instance.ValidateLocale(locale); err != nil {
		return instance.TextRevision{}, err
	}
	if err := def.Validate(in.Body); err != nil {
		return instance.TextRevision{}, err
	}
	if f.texts == nil {
		f.texts = map[instance.TextKey]map[string]string{}
	}
	if f.texts[in.Key] == nil {
		f.texts[in.Key] = map[string]string{}
	}
	f.texts[in.Key][locale] = in.Body
	f.revisedBy = append(f.revisedBy, in.RevisedByUserID)
	return instance.TextRevision{ID: "rev", Key: in.Key, Locale: locale, Body: in.Body, RevisedByUserID: in.RevisedByUserID, RevisedAt: adminNow}, nil
}

// newTextsEnv is newAuthEnv with the instance texts and the admin tokens: alice holds every
// permission and bob none.
func newTextsEnv(t *testing.T) (*authEnv, *fakeInstance) {
	t.Helper()
	f := &fakeInstance{}
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Permissions = fakePermissions{"user-" + aliceAccountID: identity.AllPermissions()}
		o.Instance, o.Texts = f, f
	})
	e.auth.tokens["admin"] = bearer("admin", aliceAccountID)
	e.auth.tokens["admin-read"] = bearer("admin:read", aliceAccountID)
	e.auth.tokens["bob-admin"] = bearer("admin", bobAccountID)
	return e, f
}

func TestInstanceTextOperationsNeedManageSettings(t *testing.T) {
	e, f := newTextsEnv(t)
	for _, op := range []struct{ method, path, body string }{
		{http.MethodGet, "/admin/instance/texts", ""},
		{http.MethodPut, "/admin/instance/texts/name/en", `{"text":"Example"}`},
	} {
		res := e.send(t, op.method, op.path, "bob-admin", op.body)
		if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeMissingPermission ||
			p.Detail == nil || !strings.Contains(*p.Detail, "manage_settings") {
			t.Errorf("%s %s by bob = %d %+v, want 403 for manage_settings", op.method, op.path, res.StatusCode, p)
		}
	}
	res := e.sendJSON(t, http.MethodPut, "/admin/instance/texts/name/en", "admin-read", `{"text":"Example"}`)
	if p := decodeProblem(t, res); res.StatusCode != http.StatusForbidden || p.Type != clientapi.ProblemTypeInsufficientScope {
		t.Errorf("set with admin:read = %d %+v, want 403 insufficient-scope", res.StatusCode, p)
	}
	if len(f.texts) != 0 {
		t.Errorf("a refused operation changed the texts: %v", f.texts)
	}
}

// TestSetInstanceName checks that the name set through the admin API names the instance in GET
// /instance, in the reader's locale, while the domain stays the domain; clearing every locale
// names it by the domain again.
func TestSetInstanceName(t *testing.T) {
	e, f := newTextsEnv(t)
	instanceInfo := func(lang string) clientapi.Instance {
		t.Helper()
		res := e.get(t, "/instance", map[string]string{"Accept-Language": lang})
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET /instance = %d", res.StatusCode)
		}
		return decodeJSON[clientapi.Instance](t, res)
	}
	if got := instanceInfo("en"); got.Name != "social.example" || got.Domain != "social.example" {
		t.Fatalf("instance without a name = %+v, want the domain for both", got)
	}

	res := e.sendJSON(t, http.MethodPut, "/admin/instance/texts/name/en", "admin", `{"text":"Example Social"}`)
	set := decodeJSON[clientapi.AdminInstanceText](t, res)
	want := clientapi.AdminInstanceText{Key: clientapi.AdminInstanceTextKeyName, Locale: "en", Text: "Example Social", RevisedAt: adminNow}
	if res.StatusCode != http.StatusOK || set != want || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("set = %d %+v, want %+v", res.StatusCode, set, want)
	}
	if !reflect.DeepEqual(f.revisedBy, []string{"user-" + aliceAccountID}) {
		t.Errorf("revised by %q, want alice's user", f.revisedBy)
	}
	if res := e.sendJSON(t, http.MethodPut, "/admin/instance/texts/name/ja", "admin", `{"text":"例のソーシャル"}`); res.StatusCode != http.StatusOK {
		t.Errorf("set ja = %d, want 200", res.StatusCode)
	}
	// The locale is canonicalized.
	if res := e.sendJSON(t, http.MethodPut, "/admin/instance/texts/short_description/ja_jp", "admin", `{"text":"小さなサーバー"}`); res.StatusCode != http.StatusOK ||
		decodeJSON[clientapi.AdminInstanceText](t, res).Locale != "ja-JP" {
		t.Errorf("set ja_jp = %d, want 200 in ja-JP", res.StatusCode)
	}
	for lang, name := range map[string]string{"en": "Example Social", "ja-JP": "例のソーシャル", "fr": "Example Social"} {
		if got := instanceInfo(lang); got.Name != name || got.Domain != "social.example" {
			t.Errorf("instance for %s = name %q domain %q, want %q and the domain", lang, got.Name, got.Domain, name)
		}
	}

	res = e.admin(t, http.MethodGet, "/admin/instance/texts", "admin-read", nil)
	list := decodeJSON[clientapi.AdminInstanceTextList](t, res)
	wantList := []clientapi.AdminInstanceText{
		{Key: clientapi.AdminInstanceTextKeyName, Locale: "en", Text: "Example Social", RevisedAt: adminNow},
		{Key: clientapi.AdminInstanceTextKeyName, Locale: "ja", Text: "例のソーシャル", RevisedAt: adminNow},
		{Key: clientapi.AdminInstanceTextKeyShortDescription, Locale: "ja-JP", Text: "小さなサーバー", RevisedAt: adminNow},
	}
	if res.StatusCode != http.StatusOK || !reflect.DeepEqual(list.Items, wantList) || res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("list = %d %+v, want %+v", res.StatusCode, list.Items, wantList)
	}

	// An empty text clears the name in its locale.
	for _, locale := range []string{"en", "ja"} {
		if res := e.sendJSON(t, http.MethodPut, "/admin/instance/texts/name/"+locale, "admin", `{"text":""}`); res.StatusCode != http.StatusOK {
			t.Errorf("clear %s = %d", locale, res.StatusCode)
		}
	}
	if got := instanceInfo("ja"); got.Name != "social.example" {
		t.Errorf("instance after clearing = %q, want the domain", got.Name)
	}
	res = e.admin(t, http.MethodGet, "/admin/instance/texts", "admin-read", nil)
	if list := decodeJSON[clientapi.AdminInstanceTextList](t, res); len(list.Items) != 1 || list.Items[0].Key != clientapi.AdminInstanceTextKeyShortDescription {
		t.Errorf("list after clearing = %+v, want only the description", list.Items)
	}
}

func TestSetInstanceTextProblems(t *testing.T) {
	e, f := newTextsEnv(t)
	for _, tc := range []struct {
		name, path, body string
		status           int
		// detail is the property that the detail names.
		detail string
	}{
		{"too long", "/admin/instance/texts/name/en", `{"text":"` + strings.Repeat("あ", 101) + `","x":"invalid-on-purpose"}`, http.StatusUnprocessableEntity, "text"},
		{"line break", "/admin/instance/texts/name/en", `{"text":"a\nb"}`, http.StatusUnprocessableEntity, "text"},
		{"blank", "/admin/instance/texts/name/en", `{"text":"   "}`, http.StatusUnprocessableEntity, "text"},
		{"bad locale", "/admin/instance/texts/name/e%20n", `{"text":"Example"}`, http.StatusUnprocessableEntity, "locale"},
		{"unknown key", "/admin/instance/texts/motto/en", `{"text":"Example","x":"invalid-on-purpose"}`, http.StatusNotFound, ""},
	} {
		res := e.sendJSON(t, http.MethodPut, tc.path, "admin", tc.body)
		p := decodeProblem(t, res)
		if res.StatusCode != tc.status || (tc.detail != "" && (p.Detail == nil || !strings.Contains(*p.Detail, tc.detail))) {
			t.Errorf("%s: %d %+v, want %d %q", tc.name, res.StatusCode, p, tc.status, tc.detail)
		}
	}
	// A description may have several lines.
	if res := e.sendJSON(t, http.MethodPut, "/admin/instance/texts/description/en", "admin", `{"text":"a\nb"}`); res.StatusCode != http.StatusOK {
		t.Errorf("multi-line description = %d, want 200", res.StatusCode)
	}
	if len(f.texts[instance.TextName]) != 0 {
		t.Errorf("a refused change set a name: %v", f.texts[instance.TextName])
	}
}
