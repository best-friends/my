// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_pubsub

package config

func init() { exclude("no_pubsub", "stream", "pubsub") }

// checkPubSubTopic accepts every value: no adapter of this build reads stream_pubsub_topic.
func checkPubSubTopic(string) error { return nil }

// checkPubSubSubscriptionPrefix accepts every value: no adapter of this build reads
// stream_pubsub_subscription_prefix.
func checkPubSubSubscriptionPrefix(string) error { return nil }
