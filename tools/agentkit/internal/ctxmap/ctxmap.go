// SPDX-License-Identifier: AGPL-3.0-only

// Package ctxmap reads tools/agentkit/context-map.yaml, the map from the repository's paths to the document
// sections that govern them, the checks they need, and the shared files parallel work must not edit at once.
// Coding agents keep it current: Check fails when a path, or a directory under a strict pattern, has no entry.
package ctxmap

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/best-friends/my/tools/agentkit/internal/extract"
)

// File is where the map lives, relative to the repository root.
const File = "tools/agentkit/context-map.yaml"

// Map is the parsed context map.
type Map struct {
	// Ignore lists paths no entry needs to cover (generated files, fixtures).
	Ignore []string `yaml:"ignore"`
	// Strict lists directory patterns whose every directory must be named by an entry, so that a new module,
	// adapter or feature gets its own entry with the documents that govern it.
	Strict  []string `yaml:"strict"`
	Entries []Entry  `yaml:"entries"`
}

// Entry describes one area of the repository.
type Entry struct {
	Name   string   `yaml:"name"`
	Paths  []string `yaml:"paths"`
	Docs   []string `yaml:"docs"`   // <path>#<anchor> or <path>: what an agent reads before changing the area
	Checks []string `yaml:"checks"` // make check contexts
	Shared bool     `yaml:"shared"` // sources many changes edit; never two parallel agents at once (workflow.md §9)
	Notes  []string `yaml:"notes"`  // short rules or steps worth printing in every context pack of the area
}

// Load reads the map of the repository at root.
func Load(root string) (*Map, error) {
	b, err := os.ReadFile(filepath.Join(root, File))
	if err != nil {
		return nil, err
	}
	var m Map
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	return &m, nil
}

// Match reports whether the glob pattern matches the slash-separated path p. `*` matches within one segment,
// `**` any number of segments (zero included).
func Match(pattern, p string) bool {
	return matchSegs(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func matchSegs(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(segs); i++ {
				if matchSegs(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}

// For returns the entries whose paths match p.
func (m *Map) For(p string) []Entry {
	var out []Entry
	for _, e := range m.Entries {
		for _, pat := range e.Paths {
			if Match(pat, p) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

func (m *Map) ignored(p string) bool {
	for _, pat := range m.Ignore {
		if Match(pat, p) {
			return true
		}
	}
	return false
}

// names reports whether some entry names directory d: a pattern whose segments up to d's depth match d
// without `**`, and whose segment at d's last position is d's name, literally.
func (m *Map) names(d string) bool {
	segs := strings.Split(d, "/")
	for _, e := range m.Entries {
		for _, pat := range e.Paths {
			ps := strings.Split(pat, "/")
			if len(ps) < len(segs) || ps[len(segs)-1] != segs[len(segs)-1] {
				continue
			}
			ok := true
			for i := range segs {
				if m, _ := path.Match(ps[i], segs[i]); ps[i] == "**" || !m {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// Problem is one failure of Check.
type Problem struct {
	Path, Msg string
}

// Check verifies the map against the tracked files: every file is covered or ignored, every directory under
// a strict pattern is named by an entry, every doc reference resolves to an existing file and heading, and
// every check context exists.
func (m *Map) Check(root string, files []string, contexts []string) []Problem {
	var out []Problem
	dirs := map[string]bool{}
	uncovered := map[string][]string{}
	for _, f := range files {
		for d := path.Dir(f); d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
		if m.ignored(f) || len(m.For(f)) > 0 {
			continue
		}
		d := path.Dir(f)
		uncovered[d] = append(uncovered[d], path.Base(f))
	}
	for _, d := range sortedKeys(uncovered) {
		fs := uncovered[d]
		msg := fmt.Sprintf("no entry covers %s", strings.Join(fs[:min(len(fs), 3)], ", "))
		if len(fs) > 3 {
			msg += fmt.Sprintf(" and %d more", len(fs)-3)
		}
		out = append(out, Problem{d + "/", msg})
	}
	for _, pat := range m.Strict {
		for _, d := range sortedKeys(dirs) {
			if Match(pat, d) && !m.ignored(d+"/") && !m.names(d) {
				out = append(out, Problem{d + "/", fmt.Sprintf("matches strict pattern %q but no entry names it; add an entry with the docs that govern it", pat)})
			}
		}
	}
	anchors := map[string]map[string]bool{}
	known := map[string]bool{}
	for _, c := range contexts {
		known[c] = true
	}
	for _, e := range m.Entries {
		if e.Name == "" || len(e.Paths) == 0 {
			out = append(out, Problem{File, fmt.Sprintf("entry %q needs a name and paths", e.Name)})
		}
		for _, c := range e.Checks {
			if !known[c] {
				out = append(out, Problem{File, fmt.Sprintf("entry %q: unknown check context %q", e.Name, c)})
			}
		}
		for _, ref := range e.Docs {
			file, anchor, _ := strings.Cut(ref, "#")
			b, err := os.ReadFile(filepath.Join(root, file))
			if err != nil {
				out = append(out, Problem{File, fmt.Sprintf("entry %q: %s does not exist", e.Name, file)})
				continue
			}
			if anchor == "" {
				continue
			}
			if anchors[file] == nil {
				anchors[file] = map[string]bool{}
				for _, n := range extract.Markdown(file, b).Nodes {
					if strings.HasPrefix(n.ID, "doc:") {
						anchors[file][n.Name] = true
					}
				}
			}
			if !anchors[file][anchor] {
				out = append(out, Problem{File, fmt.Sprintf("entry %q: %s has no heading #%s (agentkit outline %s)", e.Name, file, anchor, file)})
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
