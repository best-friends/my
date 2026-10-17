// SPDX-License-Identifier: AGPL-3.0-only

//go:build federationlive

package federationlive

import (
	"fmt"
	"strings"
	"testing"
)

const localHandle = localUser + "@" + localHost

// TestPeerResolvesLocal: each peer finds alice@my.test through WebFinger and fetches her actor
// (Mastodon in secure mode signs the fetch, GoToSocial always does).
func TestPeerResolvesLocal(t *testing.T) {
	e := stack(t)
	for _, p := range e.peers {
		t.Run(p.name, func(t *testing.T) {
			a, err := p.resolve(t.Context(), localHandle)
			if err != nil {
				t.Fatal(err)
			}
			if a.Username != localUser || a.Acct != localHandle {
				t.Errorf("%s resolved %s as %+v", p.host, localHandle, a)
			}
		})
	}
}

// TestLocalResolvesPeer: this server finds each peer user through WebFinger and fetches the actor
// with a GET signed by its instance actor (federation.md §2, §6), which Mastodon's secure mode and
// GoToSocial require.
func TestLocalResolvesPeer(t *testing.T) {
	e := stack(t)
	for _, p := range e.peers {
		t.Run(p.name, func(t *testing.T) {
			a, err := e.local.resolve(t.Context(), p.handle())
			if err != nil {
				t.Fatal(err)
			}
			if a.Username != p.user || a.Acct != p.handle() || !strings.HasPrefix(a.URI, "https://"+p.host+"/") {
				t.Errorf("%s resolved %s as %+v", localHost, p.handle(), a)
			}
		})
	}
}

// TestFollowFromPeer: a peer user follows alice. The signed Follow reaches the inbox of this
// server, which verifies and records it (federation.md §3.1); then this server accepts it and the
// peer shows the follow.
func TestFollowFromPeer(t *testing.T) {
	e := stack(t)
	for _, p := range e.peers {
		t.Run(p.name, func(t *testing.T) {
			ctx := t.Context()
			actor, err := e.local.resolve(ctx, p.handle())
			if err != nil {
				t.Fatal(err)
			}
			alice, err := p.resolve(ctx, localHandle)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.follow(ctx, alice.ID); err != nil {
				t.Fatal(err)
			}

			t.Run("delivered to the inbox", func(t *testing.T) {
				eventually(t, deliveryTimeout, "the Follow of "+actor.URI+" in received_activities", func() (bool, string) {
					got := e.receivedActivities(t, actor.URI)
					for _, a := range got {
						if a.Type == "Follow" {
							if a.Trust != "signed" {
								t.Errorf("Follow %s: trust = %s, want signed", a.URI, a.Trust)
							}
							t.Logf("Follow %s signed with %s", a.URI, a.SignatureScheme)
							return true, ""
						}
					}
					return false, fmt.Sprintf("%+v", got)
				})
			})

			t.Run("accepted", func(t *testing.T) {
				pending(t, "#248")
				eventually(t, deliveryTimeout, p.host+" follows "+localHandle, func() (bool, string) {
					r, err := p.relationship(ctx, alice.ID)
					return err == nil && r.Following, fmt.Sprintf("%+v %v", r, err)
				})
				eventually(t, deliveryTimeout, localHost+" is followed by "+p.handle(), func() (bool, string) {
					r, err := e.local.relationship(ctx, actor.ID)
					return err == nil && r.FollowedBy, fmt.Sprintf("%+v %v", r, err)
				})
			})
		})
	}
}

// TestFollowFromLocal: alice follows a peer user, the peer accepts, and both sides show the
// follow.
func TestFollowFromLocal(t *testing.T) {
	e := stack(t)
	for _, p := range e.peers {
		t.Run(p.name, func(t *testing.T) {
			pending(t, "#248")
			ctx := t.Context()
			followLocalToPeer(t, e, p)
			alice, err := p.resolve(ctx, localHandle)
			if err != nil {
				t.Fatal(err)
			}
			eventually(t, deliveryTimeout, p.handle()+" is followed by "+localHandle, func() (bool, string) {
				r, err := p.relationship(ctx, alice.ID)
				return err == nil && r.FollowedBy, fmt.Sprintf("%+v %v", r, err)
			})
		})
	}
}

// followLocalToPeer makes alice follow the peer user and waits for the Accept, and returns the
// peer user's account on this server.
func followLocalToPeer(t *testing.T, e *env, p *peerClient) account {
	t.Helper()
	ctx := t.Context()
	actor, err := e.local.resolve(ctx, p.handle())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.local.follow(ctx, actor.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, deliveryTimeout, localHandle+" follows "+p.handle(), func() (bool, string) {
		r, err := e.local.relationship(ctx, actor.ID)
		return err == nil && r.Following, fmt.Sprintf("%+v %v", r, err)
	})
	return actor
}

// followPeerToLocal makes the peer user follow alice and waits until the peer shows the follow.
func followPeerToLocal(t *testing.T, p *peerClient) {
	t.Helper()
	ctx := t.Context()
	alice, err := p.resolve(ctx, localHandle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.follow(ctx, alice.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, deliveryTimeout, p.handle()+" follows "+localHandle, func() (bool, string) {
		r, err := p.relationship(ctx, alice.ID)
		return err == nil && r.Following, fmt.Sprintf("%+v %v", r, err)
	})
}

// TestPostFromPeer: a public post of a followed peer user reaches alice's home timeline.
func TestPostFromPeer(t *testing.T) {
	e := stack(t)
	for _, p := range e.peers {
		t.Run(p.name, func(t *testing.T) {
			pending(t, "#248", "#249")
			ctx := t.Context()
			followLocalToPeer(t, e, p)
			posted, err := p.post(ctx, unique("hello from "+p.name), "public")
			if err != nil {
				t.Fatal(err)
			}
			eventually(t, deliveryTimeout, posted.URI+" in the home timeline of "+localHandle, func() (bool, string) {
				items, err := e.local.home(ctx)
				for _, s := range items {
					if s.URI == posted.URI {
						return true, ""
					}
				}
				return false, fmt.Sprintf("%d statuses %v", len(items), err)
			})
		})
	}
}

// TestPostFromLocal: a public post of alice reaches the home timeline of a peer user who follows
// her.
func TestPostFromLocal(t *testing.T) {
	e := stack(t)
	for _, p := range e.peers {
		t.Run(p.name, func(t *testing.T) {
			pending(t, "#248")
			ctx := t.Context()
			followPeerToLocal(t, p)
			posted, err := e.local.post(ctx, unique("hello to "+p.name), "public")
			if err != nil {
				t.Fatal(err)
			}
			eventually(t, deliveryTimeout, posted.URI+" in the home timeline of "+p.handle(), func() (bool, string) {
				items, err := p.home(ctx)
				for _, s := range items {
					if s.URI == posted.URI {
						return true, ""
					}
				}
				return false, fmt.Sprintf("%d statuses %v", len(items), err)
			})
		})
	}
}

// The scenarios below are named here so that the list of what the live test covers is complete;
// each is written when the issues it waits for land (federation.md §12).

// TestReply: replies in both directions keep the thread (in_reply_to).
func TestReply(t *testing.T) {
	stack(t)
	t.Skip("not written yet: needs #248 and #249 (and #253 for the thread)")
}

// TestBoost: boosts and their undo in both directions.
func TestBoost(t *testing.T) {
	stack(t)
	t.Skip("not written yet: needs #248 and #251")
}

// TestFavourite: favourites and their undo in both directions.
func TestFavourite(t *testing.T) {
	stack(t)
	t.Skip("not written yet: needs #248 and #251")
}

// TestEdit: an edited post shows the new text on the other side.
func TestEdit(t *testing.T) {
	stack(t)
	t.Skip("not written yet: needs #248 and #249")
}

// TestDelete: a deleted post disappears on the other side.
func TestDelete(t *testing.T) {
	stack(t)
	t.Skip("not written yet: needs #248 and #249")
}

// TestVisibilityAudiences: each visibility (public, unlisted, private, direct, and local, which
// never leaves this server except to the author's followers, ADR-0017) reaches exactly its
// audience.
func TestVisibilityAudiences(t *testing.T) {
	stack(t)
	t.Skip("not written yet: needs #248")
}
