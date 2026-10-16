// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
)

const (
	contentTypeActivity = "application/activity+json"
	maxInboxBody        = 1 << 20
)

// Server is the minimal single-user ActivityPub server.
type Server struct {
	base     *url.URL
	username string
	key      *rsa.PrivateKey

	verifier *Verifier
	client   *SigningClient
	log      *slog.Logger

	// Async runs background work (delivery). Product code would enqueue a job
	// through the outbox instead (ADR-0004); tests replace it to run inline.
	Async func(func())
}

// NewServer wires a server. httpClient is used for every outbound request
// (actor fetch and delivery); product code would pass the SSRF-guarded client.
func NewServer(base *url.URL, username string, key *rsa.PrivateKey, httpClient *http.Client, log *slog.Logger) *Server {
	s := &Server{base: base, username: username, key: key, log: log}
	s.client = &SigningClient{HTTP: httpClient, KeyID: s.KeyID(), Key: key}
	s.verifier = &Verifier{
		Resolver:   &HTTPKeyResolver{Client: s.client},
		PublicBase: base,
	}
	s.Async = func(f func()) { go f() }
	return s
}

func (s *Server) ActorID() string  { return s.base.JoinPath("users", s.username).String() }
func (s *Server) KeyID() string    { return s.ActorID() + "#main-key" }
func (s *Server) inboxURL() string { return s.ActorID() + "/inbox" }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/webfinger", s.handleWebFinger)
	mux.HandleFunc("GET /users/{name}", s.handleActor)
	mux.HandleFunc("POST /users/{name}/inbox", s.handleInbox)
	mux.HandleFunc("POST /inbox", s.handleInbox) // shared inbox
	return mux
}

// --- WebFinger (RFC 7033) ---

func (s *Server) handleWebFinger(w http.ResponseWriter, r *http.Request) {
	res := r.URL.Query().Get("resource")
	want := []string{"acct:" + s.username + "@" + s.base.Host, s.ActorID()}
	found := false
	for _, v := range want {
		if strings.EqualFold(res, v) {
			found = true
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	jrd := map[string]any{
		"subject": want[0],
		"aliases": []string{s.ActorID()},
		"links": []map[string]string{
			{"rel": "self", "type": contentTypeActivity, "href": s.ActorID()},
		},
	}
	w.Header().Set("Content-Type", "application/jrd+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(jrd)
}

// --- Actor ---

// actorDocument is hand-written rather than built with go-ap/activitypub:
// go-ap's Actor has no extension mechanism, so Mastodon properties such as
// manuallyApprovesFollowers / discoverable / indexable cannot be emitted.
func (s *Server) actorDocument() map[string]any {
	der, _ := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	id := s.ActorID()
	return map[string]any{
		"@context": []any{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1",
			map[string]any{
				"manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
				"toot":                      "http://joinmastodon.org/ns#",
				"discoverable":              "toot:discoverable",
				"indexable":                 "toot:indexable",
			},
		},
		"id":                        id,
		"type":                      "Person",
		"preferredUsername":         s.username,
		"name":                      s.username,
		"inbox":                     s.inboxURL(),
		"outbox":                    id + "/outbox",
		"followers":                 id + "/followers",
		"following":                 id + "/following",
		"endpoints":                 map[string]string{"sharedInbox": s.base.JoinPath("inbox").String()},
		"manuallyApprovesFollowers": false,
		"discoverable":              false,
		"indexable":                 false,
		"publicKey": map[string]string{
			"id":           s.KeyID(),
			"owner":        id,
			"publicKeyPem": pemStr,
		},
	}
}

func (s *Server) handleActor(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("name") != s.username {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentTypeActivity)
	_ = json.NewEncoder(w).Encode(s.actorDocument())
}

// --- Inbox ---

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("name") != "" && r.PathValue("name") != s.username {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboxBody+1))
	if err != nil || len(body) > maxInboxBody {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}

	// 1. Verify the HTTP signature synchronously (NFR-PERF-4).
	signer, err := s.verifier.Verify(r, body)
	if err != nil {
		s.log.Warn("signature rejected", "err", err)
		http.Error(w, "signature verification failed", http.StatusUnauthorized)
		return
	}

	// 2. Parse the activity (go-ap/activitypub).
	item, err := vocab.UnmarshalJSON(body)
	if err != nil || item == nil || !vocab.ActivityTypes.Match(item.GetType()) {
		http.Error(w, "not an activity", http.StatusBadRequest)
		return
	}
	act, err := vocab.ToActivity(item)
	if err != nil {
		http.Error(w, "not an activity", http.StatusBadRequest)
		return
	}

	// 3. Authorization: the key owner must be the activity's actor.
	if act.Actor == nil || act.Actor.GetLink().String() != signer.ActorID {
		http.Error(w, "actor does not match signature", http.StatusForbidden)
		return
	}

	s.log.Info("inbox", "type", act.Type, "actor", signer.ActorID, "scheme", signer.Scheme)

	switch {
	case vocab.FollowType.Match(act.Type):
		if act.Object == nil || act.Object.GetLink().String() != s.ActorID() {
			http.Error(w, "follow target is not this actor", http.StatusUnprocessableEntity)
			return
		}
		accept, err := s.buildAccept(act)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		inbox := signer.Actor.Inbox
		s.Async(func() {
			// Product code: enqueue a delivery job; the job must be idempotent.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.client.Post(ctx, inbox, accept); err != nil {
				s.log.Error("deliver Accept", "inbox", inbox, "err", err)
			}
		})
	default:
		// Ignored in this spike.
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) buildAccept(follow *vocab.Activity) ([]byte, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	accept := &vocab.Activity{
		ID:     vocab.ID(s.ActorID() + "#accepts/" + hex.EncodeToString(nonce[:])),
		Type:   vocab.AcceptType,
		Actor:  vocab.IRI(s.ActorID()),
		Object: follow,
	}
	b, err := jsonld.WithContext(jsonld.IRI(vocab.ActivityBaseURI)).Marshal(accept)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("empty Accept")
	}
	return b, nil
}
