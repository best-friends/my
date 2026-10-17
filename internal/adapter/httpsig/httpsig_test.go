// SPDX-License-Identifier: AGPL-3.0-only

package httpsig_test

import (
	"testing"

	"github.com/best-friends/my/internal/adapter/httpsig"
	"github.com/best-friends/my/pkg/testkit"
)

func TestContract(t *testing.T) {
	a := httpsig.New()
	testkit.HTTPSignatureContract(t, a, a)
}
