// SPDX-License-Identifier: AGPL-3.0-only

package content

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestPollClosed(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Minute), now.Add(time.Minute)
	cases := []struct {
		name     string
		closesAt *time.Time
		want     bool
	}{
		{"no end", nil, false},
		{"ended", &past, true},
		{"ends now", &now, true},
		{"open", &future, false},
	}
	for _, c := range cases {
		if got := (Poll{ClosesAt: c.closesAt}).Closed(now); got != c.want {
			t.Errorf("%s: Closed = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPollChoices(t *testing.T) {
	options := []PollOption{{Position: 0, Title: "a"}, {Position: 1, Title: "b"}, {Position: 2, Title: "c"}}
	single := Poll{Options: options}
	multiple := Poll{Multiple: true, Options: options}
	cases := []struct {
		name    string
		poll    Poll
		choices []int
		want    []int
	}{
		{"single", single, []int{1}, []int{1}},
		{"multiple sorted", multiple, []int{2, 0}, []int{0, 2}},
		{"none", single, nil, nil},
		{"two in single", single, []int{0, 1}, nil},
		{"unknown", single, []int{3}, nil},
		{"negative", multiple, []int{-1}, nil},
		{"twice", multiple, []int{1, 1}, nil},
	}
	for _, c := range cases {
		got, err := c.poll.Choices(c.choices)
		if c.want == nil {
			if !errors.Is(err, ErrInvalidPollChoices) {
				t.Errorf("%s: err = %v, want ErrInvalidPollChoices", c.name, err)
			}
			continue
		}
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s: Choices = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestPollOption(t *testing.T) {
	p := Poll{Options: []PollOption{{Position: 0, Title: "a"}, {Position: 1, Title: "b"}}}
	if o, ok := p.Option(1); !ok || o.Title != "b" {
		t.Errorf("Option(1) = %v, %v", o, ok)
	}
	if _, ok := p.Option(2); ok {
		t.Error("Option(2) found a choice")
	}
}

func TestPollVoteURI(t *testing.T) {
	if got, want := PollVoteURI("example.org", "v1"), "https://example.org/ap/objects/v1"; got != want {
		t.Errorf("PollVoteURI = %q, want %q", got, want)
	}
}
