// SPDX-License-Identifier: AGPL-3.0-only

package ops

import (
	"reflect"
	"testing"
)

func TestTransitions(t *testing.T) {
	pending := prSnapshot{ci: "pending", mergeable: "MERGEABLE"}
	queued := prSnapshot{ci: "passed", mergeable: "MERGEABLE", queued: true}
	cases := []struct {
		name string
		old  prSnapshot
		had  bool
		cur  prSnapshot
		want []string
	}{
		{"first poll, pending", prSnapshot{}, false, pending, nil},
		{"ci passes", pending, true, prSnapshot{ci: "passed", mergeable: "MERGEABLE"}, []string{"ci-green"}},
		{"ci fails", pending, true, prSnapshot{ci: "failed", mergeable: "MERGEABLE", failed: []string{"go"}}, []string{"ci-failed go"}},
		{"enters the queue", prSnapshot{ci: "passed", mergeable: "MERGEABLE"}, true, queued, []string{"queued"}},
		{"leaves the queue open", queued, true, prSnapshot{ci: "passed", mergeable: "CONFLICTING"}, []string{"conflicting", "dequeued (out of the merge queue without merging: a conflict or a failed merge group)"}},
		{"stays queued", queued, true, queued, nil},
		{"first poll, already queued", prSnapshot{}, false, queued, nil},
	}
	for _, c := range cases {
		if got := transitions(c.old, c.had, c.cur); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
