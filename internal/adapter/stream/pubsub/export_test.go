// SPDX-License-Identifier: AGPL-3.0-only

package pubsub

import (
	pubsubv2 "cloud.google.com/go/pubsub/v2"

	"github.com/best-friends/my/internal/app/stream"
)

// Client returns the Pub/Sub client of b, for the tests to manage topics and subscriptions.
func (b *Bus) Client() *pubsubv2.Client { return b.client }

// ProcessID returns the process ID of b.
func (b *Bus) ProcessID() string { return b.o.ProcessID }

// SubscriptionOf returns the full name of the subscription of l, a listener of a Bus.
func SubscriptionOf(l stream.Listener) string { return l.(*listener).name }
