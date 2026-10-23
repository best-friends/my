// SPDX-License-Identifier: AGPL-3.0-only

package jsonld_test

import (
	"testing"

	"github.com/mycroblog/my/internal/adapter/jsonld"
	appfederation "github.com/mycroblog/my/internal/app/federation"
	"github.com/mycroblog/my/pkg/testkit"
)

// TestCanonicalizerContract compares the canonicalization and the linked data signature with
// jsonld.js, the processor of Misskey, on the bundled contexts.
func TestCanonicalizerContract(t *testing.T) {
	testkit.CanonicalizerContract(t, func(t *testing.T) appfederation.Canonicalizer {
		c, err := jsonld.New()
		if err != nil {
			t.Fatalf("New = %v", err)
		}
		return c
	})
}
