// SPDX-License-Identifier: AGPL-3.0-only

package valkey_test

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	valkeygo "github.com/valkey-io/valkey-go"

	streamvalkey "github.com/best-friends/my/internal/adapter/stream/valkey"
	"github.com/best-friends/my/internal/adapter/valkey"
	"github.com/best-friends/my/internal/app/stream"
	"github.com/best-friends/my/pkg/testkit"
	"github.com/best-friends/my/pkg/testkit/valkeytest"
)

// TestMain stops the Valkey container of package valkeytest after the tests.
func TestMain(m *testing.M) {
	os.Exit(valkeytest.Main(m))
}

// The contract on miniredis needs neither Docker nor a server.
func TestStreamBusContractMiniredis(t *testing.T) {
	testkit.StreamBusContract(t, func(t *testing.T) testkit.StreamBus {
		m := miniredis.RunT(t)
		c := open(t, "redis://"+m.Addr())
		return testkit.StreamBus{
			NewBus:      func(*testing.T) stream.Bus { return streamvalkey.New(c, "") },
			Unreachable: unreachable,
			// Closing the server cuts every connection; the publishers reconnect to the restarted one.
			Break: func(t *testing.T) {
				m.Close()
				if err := m.Restart(); err != nil {
					t.Fatal(err)
				}
			},
			Resources: func(t *testing.T) int { return numsub(t, c, streamvalkey.Channel("")) },
		}
	})
}

// The contract on a Valkey server (pkg/testkit/valkeytest), only with TEST_STREAM=container
// (scripts/task.sh go:test-stream; CI on main and nightly, ADR-0050 §5): pull requests run it on miniredis.
func TestStreamBusContractValkey(t *testing.T) {
	if os.Getenv("TEST_STREAM") != "container" {
		t.Skip("the Valkey server runs with TEST_STREAM=container (scripts/task.sh go:test-stream)")
	}
	testkit.StreamBusContract(t, func(t *testing.T) testkit.StreamBus {
		s := valkeytest.New(t)
		// The connections of the test carry a name of the test's own, by which Break finds the
		// listeners to cut on the shared server.
		name := strings.TrimSuffix(strings.ReplaceAll(s.KeyPrefix, ":", "-"), "-")
		c := openNamed(t, s.URL, name)
		return testkit.StreamBus{
			NewBus:      func(*testing.T) stream.Bus { return streamvalkey.New(c, s.KeyPrefix) },
			Unreachable: unreachable,
			Break:       func(t *testing.T) { killSubscribers(t, c, name) },
			Resources:   func(t *testing.T) int { return numsub(t, c, streamvalkey.Channel(s.KeyPrefix)) },
		}
	})
}

// Relays on Buses of one server carry each other's signals.
func TestStreamBrokerContractRelay(t *testing.T) {
	testkit.StreamBrokerContract(t, func(t *testing.T, o stream.HubOptions) testkit.StreamBroker {
		m := miniredis.RunT(t)
		c := open(t, "redis://"+m.Addr())
		local := stream.NewRelay(streamvalkey.New(c, ""), stream.RelayOptions{Hub: o})
		remote := stream.NewRelay(streamvalkey.New(c, ""), stream.RelayOptions{})
		for _, r := range []*stream.Relay{local, remote} {
			t.Cleanup(func() {
				if err := r.Close(context.Background()); err != nil {
					t.Errorf("Close: %v", err)
				}
			})
		}
		return testkit.StreamBroker{
			Publisher: local, Subscriber: local, Remote: remote,
			Break: func(t *testing.T) {
				m.Close()
				if err := m.Restart(); err != nil {
					t.Fatal(err)
				}
			},
		}
	})
}

// Buses with different key prefixes on one server do not receive each other's payloads.
func TestBusesAreSeparate(t *testing.T) {
	m := miniredis.RunT(t)
	c := open(t, "redis://"+m.Addr())
	a, b := streamvalkey.New(c, "a:"), streamvalkey.New(c, "b:")
	got := make(chan []byte, 1)
	l, err := b.Listen(t.Context(), func(p []byte) { got <- p })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close(context.Background()) })
	if err := a.Send(t.Context(), []byte("for a")); err != nil {
		t.Fatal(err)
	}
	if err := b.Send(t.Context(), []byte("for b")); err != nil {
		t.Fatal(err)
	}
	if p := <-got; string(p) != "for b" {
		t.Fatalf("b's listener received %q; want only b's payload", p)
	}
}

// unreachable returns a Bus whose server has stopped since the client connected.
func unreachable(t *testing.T) stream.Bus {
	m := miniredis.RunT(t)
	c := open(t, "redis://"+m.Addr())
	m.Close()
	return streamvalkey.New(c, "")
}

// open connects to the server at url and closes the client when t ends.
func open(t *testing.T, url string) valkey.Client {
	t.Helper()
	c, err := valkey.Open(t.Context(), valkey.Options{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// openNamed is open for a client whose connections are named name (CLIENT SETNAME).
func openNamed(t *testing.T, url, name string) valkey.Client {
	t.Helper()
	opt, err := valkeygo.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	opt.DisableCache = true
	opt.ClientName = name
	c, err := valkeygo.NewClient(opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// numsub returns the number of connections subscribed to channel.
func numsub(t *testing.T, c valkey.Client, channel string) int {
	t.Helper()
	n, err := c.Do(t.Context(), c.B().PubsubNumsub().Channel(channel).Build()).AsIntMap()
	if err != nil {
		t.Fatal(err)
	}
	return int(n[channel])
}

// killSubscribers cuts the connections named name that are subscribed to a channel.
func killSubscribers(t *testing.T, c valkey.Client, name string) {
	t.Helper()
	list, err := c.Do(t.Context(), c.B().ClientList().Build()).ToString()
	if err != nil {
		t.Fatal(err)
	}
	killed := 0
	sc := bufio.NewScanner(strings.NewReader(list))
	for sc.Scan() {
		fields := map[string]string{}
		for f := range strings.FieldsSeq(sc.Text()) {
			if k, v, ok := strings.Cut(f, "="); ok {
				fields[k] = v
			}
		}
		if fields["name"] != name || fields["sub"] == "0" || fields["sub"] == "" {
			continue
		}
		id, err := strconv.ParseInt(fields["id"], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Do(t.Context(), c.B().ClientKill().Id(id).Build()).Error(); err != nil {
			t.Fatal(err)
		}
		killed++
	}
	if killed == 0 {
		t.Fatal("no subscribed connection to cut")
	}
}
