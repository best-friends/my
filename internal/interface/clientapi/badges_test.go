// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"

	appcontent "github.com/mycroblog/my/internal/app/content"
	appidentity "github.com/mycroblog/my/internal/app/identity"
	"github.com/mycroblog/my/internal/domain/identity"
	"github.com/mycroblog/my/internal/interface/clientapi"
)

const supporterRoleID = "01890a5d-ac96-774b-bcce-b302099a8100"

// fakeBadges holds the badge choices of each user: the roles they hold with a badge, and
// whether they show it. reads counts the profile reads.
type fakeBadges struct {
	choices map[string][]identity.BadgeChoice
	reads   int
}

func (f *fakeBadges) ProfileBadges(_ context.Context, userID string) ([]identity.ProfileBadge, error) {
	f.reads++
	var out []identity.ProfileBadge
	for _, c := range f.choices[userID] {
		if c.Displayed {
			out = append(out, c.ProfileBadge)
		}
	}
	return out, nil
}

func (f *fakeBadges) BadgeChoices(_ context.Context, userID string) ([]identity.BadgeChoice, error) {
	return f.choices[userID], nil
}

func (f *fakeBadges) SetBadgeDisplayed(_ context.Context, userID, roleID string, displayed bool) (identity.BadgeChoice, error) {
	i := slices.IndexFunc(f.choices[userID], func(c identity.BadgeChoice) bool { return c.RoleID == roleID })
	if i < 0 {
		return identity.BadgeChoice{}, appidentity.ErrNotFound
	}
	f.choices[userID][i].Displayed = displayed
	return f.choices[userID][i], nil
}

func newBadgeEnv(t *testing.T, displayed bool, tweak func(*clientapi.Options)) (*authEnv, *fakeBadges) {
	t.Helper()
	badges := &fakeBadges{choices: map[string][]identity.BadgeChoice{
		userOf(aliceAccountID): {{
			ProfileBadge: identity.ProfileBadge{RoleID: supporterRoleID, Label: "Supporter", Color: identity.BadgeGreen}, Displayed: displayed,
		}},
	}}
	return newAuthEnv(t, func(o *clientapi.Options) {
		o.Badges = badges
		if tweak != nil {
			tweak(o)
		}
	}), badges
}

func TestProfileShowsTheChosenBadges(t *testing.T) {
	want := []clientapi.AccountBadge{{RoleId: supporterRoleID, Label: "Supporter", Color: clientapi.BadgeColorGreen}}
	e, _ := newBadgeEnv(t, true, nil)
	for _, path := range []string{"/accounts/" + aliceAccountID, "/accounts/lookup?acct=alice"} {
		res := e.get(t, path, nil)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", path, res.StatusCode)
		}
		var got clientapi.Account
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Badges, want) {
			t.Errorf("GET %s: badges = %+v, want %+v", path, got.Badges, want)
		}
	}
	res := e.get(t, "/accounts/verify_credentials", map[string]string{"Authorization": "Bearer read"})
	var own clientapi.CredentialAccount
	if err := json.NewDecoder(res.Body).Decode(&own); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(own.Badges, want) {
		t.Errorf("verify_credentials: badges = %+v, want %+v", own.Badges, want)
	}

	// A badge that the user has not chosen to show is not shown.
	hidden, _ := newBadgeEnv(t, false, nil)
	res = hidden.get(t, "/accounts/"+aliceAccountID, nil)
	var got clientapi.Account
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Badges == nil || len(got.Badges) != 0 {
		t.Errorf("hidden badge: badges = %#v, want an empty list", got.Badges)
	}
}

// Accounts embedded in posts (and listed elsewhere) have no badges and read none.
func TestEmbeddedAccountsHaveNoBadges(t *testing.T) {
	f := &fakeStatuses{view: func(v appcontent.Viewer, id string) (appcontent.StatusView, error) {
		if id != sampleStatusID {
			return appcontent.StatusView{}, errNotFound
		}
		return sampleView(v), nil
	}}
	e, badges := newBadgeEnv(t, true, func(o *clientapi.Options) { o.Statuses, o.StatusReader, o.Interactions = f, f, f })
	res := e.send(t, http.MethodGet, "/statuses/"+sampleStatusID, "", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", res.StatusCode)
	}
	s := decodeStatus(t, res)
	if s.Account.Username != "alice" || s.Account.Badges == nil || len(s.Account.Badges) != 0 {
		t.Errorf("the author's badges = %#v, want an empty list", s.Account.Badges)
	}
	if badges.reads != 0 {
		t.Errorf("presenting a post read the badges %d times, want 0", badges.reads)
	}
}

func TestBadgeChoices(t *testing.T) {
	e, badges := newBadgeEnv(t, false, nil)
	res := e.get(t, "/preferences/badges", map[string]string{"Authorization": "Bearer read-accounts"})
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET = %d, Cache-Control %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	var list clientapi.BadgeChoiceList
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	want := clientapi.BadgeChoice{RoleId: supporterRoleID, Label: "Supporter", Color: clientapi.BadgeColorGreen}
	if !reflect.DeepEqual(list.Items, []clientapi.BadgeChoice{want}) {
		t.Errorf("choices = %+v, want supporter hidden", list.Items)
	}

	res = e.sendJSON(t, http.MethodPut, "/preferences/badges/"+supporterRoleID, "write", `{"displayed":true}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d", res.StatusCode)
	}
	var got clientapi.BadgeChoice
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want.Displayed = true
	if got != want || !badges.choices[userOf(aliceAccountID)][0].Displayed {
		t.Errorf("PUT = %+v, want %+v saved", got, want)
	}

	// A role that the user does not hold, or that has no badge, is not found.
	other := "01890a5d-ac96-774b-bcce-b302099a8101"
	if res := e.sendJSON(t, http.MethodPut, "/preferences/badges/"+other, "write", `{"displayed":true}`); res.StatusCode != http.StatusNotFound {
		t.Errorf("PUT of another role = %d, want 404", res.StatusCode)
	}
	if res := e.sendJSON(t, http.MethodPut, "/preferences/badges/"+supporterRoleID, "read", `{"displayed":false}`); res.StatusCode != http.StatusForbidden {
		t.Errorf("PUT with a read token = %d, want 403", res.StatusCode)
	}
	// Bob holds no badge.
	res = e.get(t, "/preferences/badges", map[string]string{"Authorization": "Bearer bob"})
	list = clientapi.BadgeChoiceList{}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Items == nil || len(list.Items) != 0 {
		t.Errorf("bob's choices = %#v, want an empty list", list.Items)
	}
}

func TestBadgeChoicesWithoutBadges(t *testing.T) {
	e := newAuthEnv(t, nil)
	res := e.get(t, "/preferences/badges", map[string]string{"Authorization": "Bearer read"})
	var list clientapi.BadgeChoiceList
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || len(list.Items) != 0 {
		t.Errorf("GET = %d %+v, want 200 and none", res.StatusCode, list)
	}
	if res := e.sendJSON(t, http.MethodPut, "/preferences/badges/"+supporterRoleID, "write", `{"displayed":true}`); res.StatusCode != http.StatusNotFound {
		t.Errorf("PUT = %d, want 404", res.StatusCode)
	}
}
