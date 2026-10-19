// SPDX-License-Identifier: AGPL-3.0-only

// Package valkeytest gives tests a Valkey server, for the adapters on internal/adapter/valkey,
// which the tests connect with valkey.Open.
//
// One server serves every test of a test binary, and every test gets its own key prefix, so
// tests do not see each other's keys. Environment variables choose the server:
//
//   - TEST_VALKEY_URL: an existing server, as a valkey:// URL; the tests write keys under a
//     random prefix there and delete them at the end.
//   - Without it, a container of TEST_VALKEY_IMAGE (default DefaultImage) is started with
//     testcontainers-go when Docker is available, and the tests are skipped otherwise.
//   - TEST_VALKEY_REQUIRED: when non-empty, a container that cannot start because Docker is
//     unavailable fails the tests instead of skipping them. CI sets it.
//
// The server starts on the first New call. The TestMain of a package that uses valkeytest
// calls Close after m.Run (or os.Exit(valkeytest.Main(m))) to stop it.
package valkeytest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	valkeygo "github.com/valkey-io/valkey-go"
)

// The environment variables that choose the server; see the package documentation.
const (
	EnvURL      = "TEST_VALKEY_URL"
	EnvImage    = "TEST_VALKEY_IMAGE"
	EnvRequired = "TEST_VALKEY_REQUIRED"
)

// DefaultImage is the container image of the tests.
const DefaultImage = "valkey/valkey:9.1.2-alpine"

var server struct {
	once sync.Once
	url  string // empty when there is no server
	skip string // why url is empty, when the tests skip
	err  error  // why url is empty, when the tests fail
	stop func()
}

// Main runs the tests of m and then stops the server, if New started one.
func Main(m *testing.M) int {
	code := m.Run()
	Close()
	return code
}

// Close stops the server, if New started one.
func Close() {
	if server.stop != nil {
		server.stop()
		server.stop = nil
	}
}

// Server is the server of one test.
type Server struct {
	// URL is the server's valkey:// URL.
	URL string
	// KeyPrefix is the test's own key prefix.
	KeyPrefix string
}

// New returns the test server, with a new key prefix whose keys are deleted when t ends. It
// skips t when no server is available.
func New(t testing.TB) Server {
	t.Helper()
	server.once.Do(setUp)
	if server.err != nil {
		t.Fatalf("valkeytest: %v", server.err)
	}
	if server.url == "" {
		t.Skip(server.skip)
	}
	opt, err := valkeygo.ParseURL(server.url)
	if err != nil {
		t.Fatalf("valkeytest: %v", err)
	}
	opt.DisableCache = true
	c, err := valkeygo.NewClient(opt)
	if err != nil {
		t.Fatalf("valkeytest: %v", err)
	}
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	prefix := "test:" + hex.EncodeToString(suffix) + ":"
	t.Cleanup(func() {
		defer c.Close()
		ctx := context.Background()
		var cursor uint64
		for {
			e, err := c.Do(ctx, c.B().Scan().Cursor(cursor).Match(prefix+"*").Count(1000).Build()).AsScanEntry()
			if err != nil {
				return
			}
			for _, k := range e.Elements {
				_ = c.Do(ctx, c.B().Del().Key(k).Build()).Error()
			}
			if cursor = e.Cursor; cursor == 0 {
				return
			}
		}
	})
	return Server{URL: server.url, KeyPrefix: prefix}
}

func setUp() {
	if u := os.Getenv(EnvURL); u != "" {
		server.url = u
		return
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		reason := fmt.Sprintf("Docker is unavailable for the Valkey container (%v); set %s to use a server", err, EnvURL)
		if os.Getenv(EnvRequired) != "" {
			server.err = fmt.Errorf("%s, but %s is set", reason, EnvRequired)
			return
		}
		server.skip = reason
		return
	}
	image := os.Getenv(EnvImage)
	if image == "" {
		image = DefaultImage
	}
	ctx := context.Background()
	c, err := testcontainers.Run(ctx, image,
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("6379/tcp").WithStartupTimeout(time.Minute)),
	)
	if c != nil {
		server.stop = func() { _ = testcontainers.TerminateContainer(c) }
	}
	if err != nil {
		server.err = fmt.Errorf("start container %s: %w", image, err)
		return
	}
	endpoint, err := c.PortEndpoint(ctx, "6379/tcp", "valkey")
	if err != nil {
		server.err = fmt.Errorf("container endpoint: %w", err)
		return
	}
	server.url = endpoint
}
