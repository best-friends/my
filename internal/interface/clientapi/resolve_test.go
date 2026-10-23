// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"

	appaccount "github.com/mycroblog/my/internal/app/account"
	appfederation "github.com/mycroblog/my/internal/app/federation"
	"github.com/mycroblog/my/internal/domain/account"
	"github.com/mycroblog/my/internal/interface/clientapi"
)

const remoteAccountID = "01890a5d-ac96-774b-bcce-b302099a8100"

// remoteBob is the remote account that fakeResolver fetches.
var remoteBob = appaccount.Actor{
	Account:   account.Account{ID: remoteAccountID, URI: "https://remote.example/users/bob", Domain: "remote.example", ActorType: account.ActorPerson, CreatedAt: createdAt},
	Handle:    &account.Handle{AccountID: remoteAccountID, Username: "bob", Domain: "remote.example"},
	Profile:   &account.ProfileRevision{DisplayName: "Bob", NoteHTML: "<p>remote</p>", Bot: true},
	Lifecycle: account.InitialLifecycleState(remoteAccountID),
}

// fakeResolver resolves bob@remote.example and its actor URL, and nothing else; it records the
// handles and URLs it is asked about.
type fakeResolver struct {
	mu    sync.Mutex
	asked []string
}

func (f *fakeResolver) ResolveHandle(_ context.Context, username, domain string) (appaccount.Actor, error) {
	f.record(username + "@" + domain)
	if username == "bob" && domain == "remote.example" {
		return remoteBob, nil
	}
	return appaccount.Actor{}, fmt.Errorf("%w: no such handle", appfederation.ErrUnresolvable)
}

func (f *fakeResolver) ResolveURL(_ context.Context, uri string) (appaccount.Actor, error) {
	f.record(uri)
	if uri == remoteBob.Account.URI {
		return remoteBob, nil
	}
	return appaccount.Actor{}, fmt.Errorf("%w: no such actor", appfederation.ErrUnresolvable)
}

func (f *fakeResolver) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, s)
}

func (f *fakeResolver) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

// TestLookupAccountResolve checks that resolve fetches an unknown remote account by handle or
// URL, answers 404 for what does not resolve, needs a token, and is rate limited.
func TestLookupAccountResolve(t *testing.T) {
	resolver := &fakeResolver{}
	e := newAuthEnv(t, func(o *clientapi.Options) { o.Resolver = resolver })
	auth := map[string]string{"Authorization": "Bearer read-accounts"}
	lookup := func(acct string, resolve bool, header map[string]string) *http.Response {
		t.Helper()
		q := url.Values{"acct": {acct}}
		if resolve {
			q.Set("resolve", "true")
		}
		return e.get(t, "/accounts/lookup?"+q.Encode(), header)
	}

	// Without resolve, an unknown remote handle is not fetched.
	if res := lookup("@bob@remote.example", false, auth); res.StatusCode != http.StatusNotFound {
		t.Fatalf("lookup without resolve = %d, want 404", res.StatusCode)
	}
	if got := resolver.calls(); len(got) != 0 {
		t.Fatalf("resolver asked %v without resolve", got)
	}

	for _, acct := range []string{"@bob@remote.example", "bob@remote.example", remoteBob.Account.URI} {
		res := lookup(acct, true, auth)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("resolve %s = %d, want 200", acct, res.StatusCode)
		}
		var got clientapi.Account
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Id != remoteAccountID || got.Acct != "bob@remote.example" || got.Domain != "remote.example" ||
			got.Uri != remoteBob.Account.URI || got.DisplayName != "Bob" || !got.Bot || got.FollowersCount == nil {
			t.Errorf("resolve %s = %+v", acct, got)
		}
		if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("resolve %s: Cache-Control = %q", acct, cc)
		}
	}
	if got := resolver.calls(); len(got) != 3 || got[0] != "bob@remote.example" || got[2] != remoteBob.Account.URI {
		t.Errorf("resolver asked %v", got)
	}

	// A local account is found without resolving; the local domain is never resolved.
	res := lookup("alice@social.example", true, auth)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("resolve local = %d, want 200", res.StatusCode)
	}
	var alice clientapi.Account
	if err := json.NewDecoder(res.Body).Decode(&alice); err != nil {
		t.Fatal(err)
	}
	if alice.Domain != "social.example" || alice.Uri != "https://social.example/ap/actors/"+aliceAccountID {
		t.Errorf("local account = %+v", alice)
	}
	if res := lookup("nobody@social.example", true, auth); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown local = %d, want 404", res.StatusCode)
	}
	if got := resolver.calls(); len(got) != 3 {
		t.Errorf("resolver asked %v for local handles", got)
	}

	// What does not resolve is a localized 404.
	for _, acct := range []string{"carol@remote.example", "https://remote.example/users/carol", "a@b@c"} {
		res := lookup(acct, true, map[string]string{"Authorization": "Bearer read-accounts", "Accept-Language": "en"})
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("resolve %s = %d, want 404", acct, res.StatusCode)
		}
		if p := decodeProblem(t, res); p.Type != clientapi.ProblemTypeBlank {
			t.Errorf("resolve %s: problem = %+v", acct, p)
		}
	}

	// Resolving needs a token.
	res = lookup("bob@remote.example", true, nil)
	if res.StatusCode != http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("anonymous resolve = %d (WWW-Authenticate %q), want 401", res.StatusCode, res.Header.Get("WWW-Authenticate"))
	}
	if p := decodeProblem(t, res); p.Type != clientapi.ProblemTypeUnauthenticated {
		t.Errorf("anonymous resolve: problem = %+v", p)
	}

	// Each resolution takes one of ResolveLimit; finding a stored account takes none.
	for i := int64(len(resolver.calls())); i < clientapi.ResolveLimit.Max; i++ {
		if res := lookup("carol@remote.example", true, auth); res.StatusCode != http.StatusNotFound {
			t.Fatalf("resolve %d = %d, want 404", i, res.StatusCode)
		}
	}
	res = lookup("carol@remote.example", true, map[string]string{"Authorization": "Bearer read-accounts", "Accept-Language": "ja"})
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" || res.Header.Get("Content-Language") != "ja" {
		t.Fatalf("over the limit = %d (Retry-After %q)", res.StatusCode, res.Header.Get("Retry-After"))
	}
	if res := lookup("alice", true, auth); res.StatusCode != http.StatusOK {
		t.Errorf("stored account over the resolve limit = %d, want 200", res.StatusCode)
	}
}
