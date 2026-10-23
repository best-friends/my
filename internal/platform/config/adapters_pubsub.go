// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_pubsub

package config

import pubsubstream "github.com/mycroblog/my/internal/adapter/stream/pubsub"

func checkPubSubTopic(v string) error { return pubsubstream.CheckTopic(v) }

func checkPubSubSubscriptionPrefix(v string) error { return pubsubstream.CheckSubscriptionPrefix(v) }
