// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// entry is one line of emoji-test.txt.
type entry struct {
	points  []rune // the code points
	hex     string // the code points as space-separated hexadecimal, as in the file
	status  string // fully-qualified, minimally-qualified, unqualified, or component
	version string // the emoji version that added it, without the leading E ("1.0")
	name    string // the English name ("waving hand: light skin tone")
	group   string // the group's name ("Smileys & Emotion")
}

func (e entry) text() string { return string(e.points) }

// parseEmojiTest reads the lines of emoji-test.txt in the file's order.
func parseEmojiTest(src []byte) ([]entry, error) {
	var entries []entry
	group := ""
	sc := bufio.NewScanner(bytes.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if g, ok := strings.CutPrefix(line, "# group: "); ok {
			group = strings.TrimSpace(g)
			continue
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		e, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		if group == "" {
			return nil, fmt.Errorf("line %q comes before any group", line)
		}
		e.group = group
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no emoji in emoji-test.txt")
	}
	return entries, nil
}

// parseCodePoint reads a code point written in hexadecimal ("1F44B").
func parseCodePoint(s string) (rune, error) {
	n, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0, err
	}
	if n < 0 || n > unicode.MaxRune {
		return 0, fmt.Errorf("%s is not a code point", s)
	}
	return rune(n), nil
}

// parseLine reads a line such as
// "1F44B 1F3FB ; fully-qualified # 👋🏻 E1.0 waving hand: light skin tone".
func parseLine(line string) (entry, error) {
	points, rest, ok := strings.Cut(line, ";")
	if !ok {
		return entry{}, fmt.Errorf("malformed line %q", line)
	}
	status, comment, ok := strings.Cut(rest, "#")
	if !ok {
		return entry{}, fmt.Errorf("malformed line %q", line)
	}
	e := entry{hex: strings.Join(strings.Fields(points), " "), status: strings.TrimSpace(status)}
	for _, p := range strings.Fields(points) {
		r, err := parseCodePoint(p)
		if err != nil {
			return entry{}, fmt.Errorf("malformed code point in %q: %w", line, err)
		}
		e.points = append(e.points, r)
	}
	// The comment is the emoji, its version, and its name.
	fields := strings.SplitN(strings.TrimSpace(comment), " ", 3)
	if len(fields) != 3 || !strings.HasPrefix(fields[1], "E") {
		return entry{}, fmt.Errorf("malformed comment in %q", line)
	}
	if fields[0] != e.text() {
		return entry{}, fmt.Errorf("the emoji in the comment of %q differs from its code points", line)
	}
	e.version = strings.TrimPrefix(fields[1], "E")
	e.name = fields[2]
	return e, nil
}
