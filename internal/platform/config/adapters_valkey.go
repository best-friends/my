// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_valkey

package config

import "github.com/mycroblog/my/internal/adapter/valkey"

func checkValkeyURL(v string) error { return valkey.CheckURL(v) }
