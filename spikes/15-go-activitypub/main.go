// SPDX-License-Identifier: AGPL-3.0-only

// Command spike15 is a THROWAWAY prototype for issue #15: a minimal ActivityPub
// actor built on net/http that serves WebFinger and an Actor document, verifies
// HTTP Signatures (draft-cavage-12 and RFC 9421) on its inbox, and answers a
// Follow with a signed Accept (RFC 9421 first, draft-cavage fallback).
//
// It is not product code and must never be imported by product code.
// See docs/spikes/15-go-activitypub.md for the findings.
//
// Usage (needs a public HTTPS origin in front of it for real federation):
//
//	go run . -base https://my.example -user bob -addr :8080 -key bob.pem
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	base := flag.String("base", "http://localhost:8080", "public origin of this server (scheme://host)")
	user := flag.String("user", "bob", "local username")
	addr := flag.String("addr", ":8080", "listen address")
	keyPath := flag.String("key", "spike15-key.pem", "RSA private key (PKCS#8 PEM); created if missing")
	flag.Parse()

	baseURL, err := url.Parse(*base)
	if err != nil {
		return fmt.Errorf("parse -base: %w", err)
	}
	key, err := loadOrCreateKey(*keyPath)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := NewServer(baseURL, *user, key, http.DefaultClient, log)

	log.Info("listening", "addr", *addr, "actor", srv.ActorID())
	hs := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	return hs.ListenAndServe()
}

func loadOrCreateKey(path string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
			return nil, err
		}
		return key, nil
	}
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, fmt.Errorf("%s: no PEM block", path)
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s: not an RSA key", path)
	}
	return rk, nil
}
