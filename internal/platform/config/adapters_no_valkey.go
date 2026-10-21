// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_valkey

package config

func init() {
	exclude("no_valkey", "kv", "valkey")
	exclude("no_valkey", "stream", "valkey")
}

// checkValkeyURL accepts every value: no adapter of this build reads valkey_url.
func checkValkeyURL(string) error { return nil }
