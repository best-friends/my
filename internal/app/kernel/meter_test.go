// SPDX-License-Identifier: AGPL-3.0-only

package kernel_test

import (
	"testing"

	"github.com/best-friends/my/internal/app/kernel"
)

func TestLabels(t *testing.T) {
	l := kernel.NewLabels("a", "1", "b", "2").With("a", "3").With("c", "4")
	if l.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", l.Len())
	}
	if v, ok := l.Get("a"); !ok || v != "3" {
		t.Errorf("Get(a) = %q, %v; want 3", v, ok)
	}
	var keys []string
	for k := range l.All() {
		keys = append(keys, k)
	}
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("All() keys = %v", keys)
	}
	i := kernel.Instrument{Name: "my.x", LabelKeys: []string{"a", "b"}}
	if err := i.Check(l); err == nil {
		t.Error("Check() = nil with the undeclared label c")
	}
	if err := i.Check(kernel.NewLabels("b", "x")); err != nil {
		t.Errorf("Check() = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("With() beyond MaxLabels did not panic")
		}
	}()
	l.With("d", "5").With("e", "6")
}

func TestNopMeter(t *testing.T) {
	if !kernel.IsNop(nil) || !kernel.IsNop(kernel.NopMeter{}) || !kernel.IsNop(kernel.MeterOrNop(nil)) {
		t.Error("IsNop() = false for the nop meter")
	}
	m := kernel.MeterOrNop(nil)
	c := m.Counter(kernel.Instrument{Name: "my.x", LabelKeys: []string{"a"}})
	ctx := t.Context()
	if allocs := testing.AllocsPerRun(100, func() {
		ctx, end := m.Begin(ctx)
		c.Add(ctx, 1, kernel.NewLabels("a", "b"))
		end()
	}); allocs != 0 {
		t.Errorf("the nop meter allocates %v times per measurement", allocs)
	}
}
