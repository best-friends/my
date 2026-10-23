// SPDX-License-Identifier: AGPL-3.0-only

// Package adr lists the Architecture Decision Records in docs/adr from the working tree.
package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Dir is the directory of the ADRs, relative to the repository root.
const Dir = "docs/adr"

var (
	reFile   = regexp.MustCompile(`^(\d{4})-.*\.md$`)
	reTitle  = regexp.MustCompile(`^# ADR-\d+:\s*(.*)$`)
	reStatus = regexp.MustCompile(`^- Status:\s*([A-Za-z]+)`)
)

// ADR is one record: its number, title, and the first word of its Status line (Accepted, Superseded, ...).
type ADR struct {
	Number string
	Title  string
	Status string
	Path   string
}

// List reads every docs/adr/NNNN-*.md under root, ordered by number then file name.
func List(root string) ([]ADR, error) {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		return nil, err
	}
	var out []ADR
	for _, e := range entries {
		m := reFile.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			continue
		}
		p := Dir + "/" + e.Name()
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			return nil, err
		}
		a := ADR{Number: m[1], Path: p}
		for _, line := range strings.Split(string(b), "\n") {
			if a.Title == "" {
				if t := reTitle.FindStringSubmatch(line); t != nil {
					a.Title = strings.TrimSpace(t[1])
				}
			}
			if a.Status == "" {
				if s := reStatus.FindStringSubmatch(line); s != nil {
					a.Status = s[1]
				}
			}
		}
		if a.Title == "" || a.Status == "" {
			return nil, fmt.Errorf("%s: must start with '# ADR-NNNN: <title>' and contain '- Status: <status>'", p)
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Number != out[j].Number {
			return out[i].Number < out[j].Number
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// Filter keeps the ADRs whose status is status, ignoring case; an empty status keeps all of them.
func Filter(adrs []ADR, status string) []ADR {
	if status == "" {
		return adrs
	}
	var out []ADR
	for _, a := range adrs {
		if strings.EqualFold(a.Status, status) {
			out = append(out, a)
		}
	}
	return out
}
