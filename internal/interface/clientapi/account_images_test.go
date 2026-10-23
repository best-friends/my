// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"net/http"
	"testing"

	appcontent "github.com/mycroblog/my/internal/app/content"
	"github.com/mycroblog/my/internal/domain/account"
	"github.com/mycroblog/my/internal/domain/content"
	"github.com/mycroblog/my/internal/interface/clientapi"
)

const (
	bobAvatarID = "01890a5d-ac96-774b-bcce-b302099a80ab"
	bobHeaderID = "01890a5d-ac96-774b-bcce-b302099a80ac"
)

// imageMedia is profileMedia with bob's processed avatar and his header still being processed.
func imageMedia() *fakeMedia {
	m := profileMedia()
	m.files[bobAvatarID] = appcontent.Media{File: content.MediaFile{ID: bobAvatarID, OwnerAccountID: bobAccountID, Origin: content.MediaOriginUpload}}
	m.process(bobAvatarID, content.MediaReady)
	m.files[bobHeaderID] = appcontent.Media{File: content.MediaFile{ID: bobHeaderID, OwnerAccountID: bobAccountID, Origin: content.MediaOriginUpload}}
	return m
}

var (
	aliceAvatarURL = "https://social.example/media/" + avatarID + "/r/large.png"
	bobAvatarURL   = "https://social.example/media/" + bobAvatarID + "/r/large.png"
)

// TestTimelineReadsAccountImagesTogether checks that a page of statuses presents the images of
// its authors and boosters, read in one call, without an image that is not processed.
func TestTimelineReadsAccountImagesTogether(t *testing.T) {
	m := imageMedia()
	e := newTimelineEnv(t)
	post := e.views.statuses[postID]
	post.Author.Profile = &account.ProfileRevision{AvatarMediaID: avatarID}
	e.views.statuses[postID] = post
	e.authEnv = newAuthEnv(t, func(o *clientapi.Options) {
		o.Timelines, o.TimelineReader, o.Media = e.timelines, e.views, m
		bob := bobActor
		bob.Profile = &account.ProfileRevision{AvatarMediaID: bobAvatarID, HeaderMediaID: bobHeaderID}
		o.Accounts.(fakeAccounts)[bobAccountID] = bob
	})

	page := decodePage(t, e.get(t, "/timelines/home", map[string]string{"Authorization": "Bearer read"}))
	if len(page.Items) != 2 || page.Items[0].Boost == nil {
		t.Fatalf("items = %+v, want the boost and the post", page.Items)
	}
	boost, original := page.Items[0], page.Items[1]
	if largeURL(original.Account.Avatar) != aliceAvatarURL || original.Account.Header != nil {
		t.Errorf("author images = %v %v, want the avatar only", original.Account.Avatar, original.Account.Header)
	}
	if largeURL(boost.Boost.Account.Avatar) != aliceAvatarURL {
		t.Errorf("boosted author avatar = %v", boost.Boost.Account.Avatar)
	}
	if largeURL(boost.Account.Avatar) != bobAvatarURL || boost.Account.Header != nil {
		t.Errorf("booster images = %v %v, want the processed avatar only", boost.Account.Avatar, boost.Account.Header)
	}
	if m.imageCalls != 1 {
		t.Errorf("ImageURLs calls = %d, want 1 for the page", m.imageCalls)
	}
}

// TestNotificationsReadAccountImagesTogether checks the images of the source accounts and of
// the authors of the embedded statuses of a page of notifications, read in one call.
func TestNotificationsReadAccountImagesTogether(t *testing.T) {
	m := imageMedia()
	_, notifications, views := newNotificationEnvWithViews(t)
	view := views.statuses[statusID]
	view.Author.Profile = &account.ProfileRevision{AvatarMediaID: avatarID}
	views.statuses[statusID] = view
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Notifications, o.TimelineReader, o.Media = notifications, views, m
		bob := bobActor
		bob.Profile = &account.ProfileRevision{AvatarMediaID: bobAvatarID}
		o.Accounts.(fakeAccounts)[bobAccountID] = bob
	})

	res := e.send(t, http.MethodGet, "/notifications", "read", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	page := decode[clientapi.NotificationPage](t, res)
	var sources, authors int
	for _, n := range page.Items {
		if n.Account != nil {
			sources++
			if largeURL(n.Account.Avatar) != bobAvatarURL {
				t.Errorf("%s: source avatar = %v, want bob's", n.Type, n.Account.Avatar)
			}
		}
		if n.Status != nil {
			authors++
			if largeURL(n.Status.Account.Avatar) != aliceAvatarURL {
				t.Errorf("%s: author avatar = %v, want alice's", n.Type, n.Status.Account.Avatar)
			}
		}
	}
	if sources == 0 || authors == 0 {
		t.Errorf("notifications = %+v, want source accounts and a status", page.Items)
	}
	if m.imageCalls != 1 {
		t.Errorf("ImageURLs calls = %d, want 1 for the page", m.imageCalls)
	}
}

// TestGetAccountHasImages checks the images on an account's own endpoint.
func TestGetAccountHasImages(t *testing.T) {
	e := newAuthEnv(t, func(o *clientapi.Options) {
		o.Media = imageMedia()
		bob := bobActor
		bob.Profile = &account.ProfileRevision{AvatarMediaID: bobAvatarID, HeaderMediaID: bobHeaderID}
		bob.Lifecycle = account.InitialLifecycleState(bobAccountID)
		o.Accounts.(fakeAccounts)[bobAccountID] = bob
	})
	res := e.get(t, "/accounts/"+bobAccountID, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	got := decode[clientapi.Account](t, res)
	if largeURL(got.Avatar) != bobAvatarURL || got.Header != nil {
		t.Errorf("images = %v %v, want the processed avatar only", got.Avatar, got.Header)
	}
}

// largeURL returns the URL of the large still variant of an image, "" for none.
func largeURL(v *clientapi.MediaVariants) string {
	if v == nil {
		return ""
	}
	return v.Large.Url
}
