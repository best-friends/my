// SPDX-License-Identifier: AGPL-3.0-only

package bobsketch

import "testing"

func TestBuildBoth(t *testing.T) {
	s, p, err := BuildBoth()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sqlite: %s", s)
	t.Logf("postgres: %s", p)
}
