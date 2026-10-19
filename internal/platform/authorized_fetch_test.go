// SPDX-License-Identifier: AGPL-3.0-only

package platform_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/adapter/httpsig"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/federation"
	"github.com/best-friends/my/internal/app/identity"
	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/app/kernel"
	domcontent "github.com/best-friends/my/internal/domain/content"
	"github.com/best-friends/my/internal/domain/instance"
	"github.com/best-friends/my/internal/platform"
)

// peerNet is the kernel.HTTPClient of the servers of TestAuthorizedFetch: it sends a request
// for https://{host}/... to the test server serving that host, with its headers, as if the host
// were public. Any other host is unreachable.
type peerNet struct {
	mu      sync.Mutex
	servers map[string]*server // by host
}

func (n *peerNet) add(host string, s *server) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.servers[host] = s
}

func (n *peerNet) Do(ctx context.Context, req *kernel.HTTPRequest) (*kernel.HTTPResponse, error) {
	u, err := url.Parse(req.URL)
	if err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("%w: %s", kernel.ErrInvalidHTTPRequest, req.URL)
	}
	n.mu.Lock()
	s := n.servers[u.Host]
	n.mu.Unlock()
	if s == nil {
		return nil, fmt.Errorf("%w: %s", kernel.ErrForbiddenDestination, u.Host)
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	r, err := http.NewRequestWithContext(ctx, method, s.url+u.RequestURI(), bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	r.Header = req.Header.Clone()
	if r.Header == nil {
		r.Header = http.Header{}
	}
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	return &kernel.HTTPResponse{StatusCode: resp.StatusCode, Header: resp.Header, URL: req.URL, Body: resp.Body}, nil
}

// peer is one server of TestAuthorizedFetch with a user, a public status, and its instance actor.
type peer struct {
	host                       string
	srv                        *server
	userID, statusID, instance string
}

func startPeer(t *testing.T, host string, network *peerNet) *peer {
	t.Helper()
	cfg := loadConfig(t, map[string]string{"MY_HOST": host, "MY_DATABASE_URL": newDatabase(t, true)})
	p := &peer{host: host}
	p.srv = startApp(t, cfg, platform.AppOptions{HTTPClient: network}, platform.ServeOptions{}, func(a *platform.App) {
		ctx := t.Context()
		reg, err := a.Identity.RegisterLocalUser(ctx, identity.RegisterInput{
			Username: "alice", Password: "correct horse battery staple", DisplayName: "Alice",
		})
		if err != nil {
			t.Fatalf("RegisterLocalUser: %v", err)
		}
		p.userID = reg.AccountID
		st, err := a.Content.Create(ctx, appcontent.CreateInput{
			AuthorAccountID: p.userID, TextInput: appcontent.TextInput{Text: "hello"}, Visibility: domcontent.VisibilityPublic,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		p.statusID = st.Status.ID
		ia, err := a.Accounts.EnsureInstanceActor(ctx)
		if err != nil {
			t.Fatalf("EnsureInstanceActor: %v", err)
		}
		p.instance = ia.ID
		if _, err := a.Instance.SetSetting(ctx, appinstance.SetSettingInput{Key: instance.SettingAuthorizedFetch, Value: "true"}); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
	})
	network.add(host, p.srv)
	return p
}

// signedGet sends a GET of path to the peer to, signed (draft-cavage, as the outbound
// profile sends first) with the instance actor key of from.
func signedGet(t *testing.T, from, to *peer, path string) (*http.Response, string) {
	t.Helper()
	keyID, key, err := from.srv.app.Accounts.SigningKey(t.Context(), from.instance)
	if err != nil {
		t.Fatalf("SigningKey: %v", err)
	}
	req := &kernel.HTTPRequest{Method: http.MethodGet, URL: "https://" + to.host + path, Header: http.Header{
		"Accept": {"application/activity+json"}, "Date": {time.Now().UTC().Format(http.TimeFormat)},
	}}
	err = httpsig.New().Sign(req, federation.SigningKey{KeyID: keyID, PrivateKey: key}, federation.SignOptions{
		Scheme: federation.SchemeCavage, Algorithm: federation.AlgRSAPKCS1SHA256,
		Components: []string{"(request-target)", "host", "date"},
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	header := map[string]string{}
	for k := range req.Header {
		if k != "Host" {
			header[k] = req.Header.Get(k)
		}
	}
	return to.srv.do(t, http.MethodGet, path, header)
}

// TestAuthorizedFetch checks two servers that both use authorized fetch (FR-408) on the real
// database adapter: each fetches the other's objects and collections signed with its instance
// actor's key, and the other verifies the signature with the key that its remote actor resolver
// fetches (signed with its own instance actor's key, which the first serves without checking). Unsigned GETs of objects and collections get 401; user actors give unsigned
// requests only the document that verifying their signatures needs.
func TestAuthorizedFetch(t *testing.T) {
	t.Parallel()
	network := &peerNet{servers: map[string]*server{}}
	a := startPeer(t, "a.example", network)
	b := startPeer(t, "b.example", network)
	ap := map[string]string{"Accept": "application/activity+json"}

	for _, pair := range [][2]*peer{{a, b}, {b, a}} {
		from, to := pair[0], pair[1]
		object := "/ap/objects/" + to.statusID
		outbox := "/ap/actors/" + to.userID + "/outbox"
		user := "/ap/actors/" + to.userID

		for _, path := range []string{object, object + "/activity", outbox, outbox + "?page=true"} {
			resp, body := to.srv.do(t, http.MethodGet, path, ap)
			if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("Accept-Signature") == "" || resp.Header.Get("Cache-Control") != "no-store" {
				t.Errorf("unsigned GET %s%s = %d %v %s, want 401 with Accept-Signature", to.host, path, resp.StatusCode, resp.Header, body)
			}
			if p := decodeProblem(t, resp, body); p.Status != http.StatusUnauthorized {
				t.Errorf("unsigned GET %s%s problem = %+v", to.host, path, p)
			}
			resp, body = signedGet(t, from, to, path)
			if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "private, no-store" {
				t.Errorf("GET %s%s signed by %s = %d %v %s, want 200 private", to.host, path, from.host, resp.StatusCode, resp.Header, body)
			}
		}

		resp, body := to.srv.do(t, http.MethodGet, user, ap)
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"publicKeyPem"`) || strings.Contains(body, `"outbox"`) ||
			resp.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("unsigned GET of the user = %d %v %s, want the private key document", resp.StatusCode, resp.Header, body)
		}
		if resp, body := signedGet(t, from, to, user); resp.StatusCode != http.StatusOK || !strings.Contains(body, `"outbox"`) {
			t.Errorf("signed GET of the user = %d %s, want the full document", resp.StatusCode, body)
		}
		if resp, body := to.srv.do(t, http.MethodGet, "/ap/actors/"+to.instance, ap); resp.StatusCode != http.StatusOK ||
			!strings.Contains(body, `"outbox"`) || resp.Header.Get("Cache-Control") != "public, max-age=60" {
			t.Errorf("unsigned GET of the instance actor = %d %v %s, want its whole public document", resp.StatusCode, resp.Header, body)
		}
		if resp, body := to.srv.do(t, http.MethodGet, "/.well-known/webfinger?resource=acct:alice@"+to.host, nil); resp.StatusCode != http.StatusOK {
			t.Errorf("WebFinger = %d %s, want 200 without a signature", resp.StatusCode, body)
		}
	}
}
