// SPDX-License-Identifier: AGPL-3.0-only

package pubsub

import pubsubv2 "cloud.google.com/go/pubsub/v2"

// Client returns the Pub/Sub client of b, for the tests to manage topics and subscriptions.
func (b *Bus) Client() *pubsubv2.Client { return b.client }

// ProcessID returns the process ID of b.
func (b *Bus) ProcessID() string { return b.o.ProcessID }
