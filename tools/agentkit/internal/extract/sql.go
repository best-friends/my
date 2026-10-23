// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"regexp"
	"strings"

	"github.com/mycroblog/my/tools/agentkit/internal/text"
)

var (
	reCreateTable = regexp.MustCompile(`(?i)^CREATE TABLE (?:IF NOT EXISTS )?"?([a-z_][a-z0-9_]*)"?`)
	reReferences  = regexp.MustCompile(`(?i)\bREFERENCES\s+"?([a-z_][a-z0-9_]*)`)
	reQueryName   = regexp.MustCompile(`^-- name: (\w+) :\w+`)
	reQueryTable  = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE)\s+"?([a-z_][a-z0-9_]*)`)
)

// sqlKeywords are words reQueryTable matches that are not tables.
var sqlKeywords = map[string]bool{"select": true, "set": true, "values": true, "lateral": true, "unnest": true, "json_each": true, "excluded": true}

// Schema makes one node per CREATE TABLE of a desired-state schema file, with the comment lines above it,
// and an edge to every table it references.
func Schema(p string, src []byte) Result {
	lines := strings.Split(string(src), "\n")
	var r Result
	for i := 0; i < len(lines); i++ {
		m := reCreateTable.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		start := i
		for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "--") {
			start--
		}
		end := i
		for end < len(lines)-1 && !strings.HasSuffix(strings.TrimSpace(lines[end]), ";") {
			end++
		}
		body := strings.Join(lines[start:end+1], "\n")
		id := "table:" + m[1]
		r.Nodes = append(r.Nodes, Node{ID: id, Name: m[1], Kind: "table", Title: "table " + m[1] + " (" + p + ")", Start: start + 1, OwnEnd: end + 1, End: end + 1, Body: body})
		seen := map[string]bool{}
		for _, ref := range reReferences.FindAllStringSubmatch(body, -1) {
			if !seen[ref[1]] && ref[1] != m[1] {
				seen[ref[1]] = true
				r.Edges = append(r.Edges, Edge{id, "table:" + ref[1], "references"})
			}
		}
		i = end
	}
	return r
}

// Queries makes one node per sqlc query (the comment above it included) and an edge to every table it reads
// or writes.
func Queries(p string, src []byte) Result {
	lines := strings.Split(string(src), "\n")
	type q struct {
		name        string
		start, line int
	}
	var qs []q
	for i, l := range lines {
		if m := reQueryName.FindStringSubmatch(l); m != nil {
			s := i
			for s > 0 && strings.HasPrefix(lines[s-1], "--") && !reQueryName.MatchString(lines[s-1]) {
				s--
			}
			qs = append(qs, q{m[1], s + 1, i + 1})
		}
	}
	var r Result
	for i, x := range qs {
		end := len(lines)
		if i+1 < len(qs) {
			end = qs[i+1].start - 1
		}
		for end > x.line && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		body := text.Lines(src, x.start, end)
		id := "query:" + x.name
		r.Nodes = append(r.Nodes, Node{ID: id, Name: x.name, Kind: "query", Title: "query " + x.name + " (" + p + ")", Start: x.start, OwnEnd: end, End: end, Body: body})
		seen := map[string]bool{}
		for _, m := range reQueryTable.FindAllStringSubmatch(body, -1) {
			t := strings.ToLower(m[1])
			if !seen[t] && !sqlKeywords[t] {
				seen[t] = true
				r.Edges = append(r.Edges, Edge{id, "table:" + t, "uses"})
			}
		}
	}
	return r
}
