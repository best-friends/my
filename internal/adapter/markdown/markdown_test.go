// SPDX-License-Identifier: AGPL-3.0-only

package markdown_test

import (
	"testing"

	"github.com/best-friends/my/internal/adapter/markdown"
	"github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/pkg/testkit"
)

func TestContract(t *testing.T) {
	testkit.DocumentRendererContract(t, func(*testing.T) instance.DocumentRenderer { return markdown.New() })
}
