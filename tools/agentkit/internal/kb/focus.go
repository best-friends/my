// SPDX-License-Identifier: AGPL-3.0-only

package kb

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mycroblog/my/tools/agentkit/internal/store"
	"github.com/mycroblog/my/tools/agentkit/internal/text"
)

func contains(s string, lower []string) bool {
	l := strings.ToLower(s)
	for _, t := range lower {
		if strings.Contains(l, t) {
			return true
		}
	}
	return false
}

func isTable(para string) bool {
	return strings.HasPrefix(strings.TrimSpace(para), "|")
}

// tableRows keeps the header of a Markdown table and the rows that mention a term.
func tableRows(para string, lower []string) string {
	lines := strings.Split(para, "\n")
	var out []string
	for i, l := range lines {
		if i < 2 || contains(l, lower) {
			out = append(out, l)
		}
	}
	if len(out) < len(lines) {
		out = append(out, fmt.Sprintf("| … %d rows without the terms |", len(lines)-len(out)))
	}
	return strings.Join(out, "\n")
}

var reItem = regexp.MustCompile(`^(?:[-*+]|\d+\.)\s`)

func isList(para string) bool {
	return reItem.MatchString(strings.TrimSpace(para))
}

// listItems keeps the items of a list (an item runs to the next line that starts one) that mention a term.
func listItems(para string, lower []string) string {
	var items []string
	for _, l := range strings.Split(para, "\n") {
		if reItem.MatchString(l) || len(items) == 0 {
			items = append(items, l)
		} else {
			items[len(items)-1] += "\n" + l
		}
	}
	var out []string
	for _, it := range items {
		if contains(it, lower) {
			out = append(out, it)
		}
	}
	if len(out) < len(items) {
		out = append(out, fmt.Sprintf("- … %d items without the terms", len(items)-len(out)))
	}
	return strings.Join(out, "\n")
}

// Focus returns the paragraphs of a node that mention any of terms (case-insensitive), each prefixed with its
// first line number, up to limit tokens; of a table only the header and the matching rows, of a list the
// matching items. It is how a big section without subsections is read in part.
func (k *KB) Focus(n store.Node, terms []string, limit int) (string, int) {
	body := k.Text(n)
	var lower []string
	for _, t := range terms {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			lower = append(lower, t)
		}
	}
	var b strings.Builder
	used, kept := 0, 0
	line := n.Start
	for _, para := range strings.Split(body, "\n\n") {
		start := line
		line += strings.Count(para, "\n") + 2
		if !contains(para, lower) {
			continue
		}
		switch {
		case isTable(para):
			para = tableRows(para, lower)
		case isList(para):
			para = listItems(para, lower)
		}
		tok := text.Tokens(para)
		if used+tok > limit {
			fmt.Fprintf(&b, "… more paragraphs match; narrow the terms or show %s:%d-%d\n", n.Path, start, n.End)
			break
		}
		fmt.Fprintf(&b, "[L%d] %s\n\n", start, strings.TrimSpace(para))
		used += tok
		kept++
	}
	return b.String(), kept
}
