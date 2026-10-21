// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_sns

package config

import snsstream "github.com/best-friends/my/internal/adapter/stream/sns"

func checkSNSTopicARN(v string) error { return snsstream.CheckTopicARN(v) }

func checkSQSQueuePrefix(v string) error { return snsstream.CheckQueuePrefix(v) }
