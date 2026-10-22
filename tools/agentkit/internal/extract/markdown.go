// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/best-friends/my/tools/agentkit/internal/text"
)

var (
	requirements = text.Requirements
	adrs         = text.ADRs
	codeSpans    = text.Idents

	reHeading = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	reFence   = regexp.MustCompile("^\\s*(```|~~~)")
	reLink    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	reADRFile = regexp.MustCompile(`^docs/adr/(\d{4})-`)
	reStatus  = regexp.MustCompile(`(?m)^- Status:\s*([A-Za-z]+)`)
	reReqRow  = regexp.MustCompile(`^\|\s*((?:NFR-[A-Z]+-\d+|NFR-\d{3}|FR-\d{3}))\s*\|`)
)

type heading struct {
	level int
	title string
	line  int
}

// Markdown makes one node per heading. A section's own text runs to the next heading of any level; its full
// range runs to the next heading of the same or a higher level. ADRs also get the ids ADR-xxxx and
// ADR-xxxx#<anchor>, and the requirement tables give each row the id of its requirement.
func Markdown(p string, src []byte) Result {
	lines := strings.Split(string(src), "\n")
	var hs []heading
	inFence := false
	for i, l := range lines {
		if reFence.MatchString(l) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := reHeading.FindStringSubmatch(l); m != nil {
			hs = append(hs, heading{len(m[1]), m[2], i + 1})
		}
	}
	total := lineCount(src)
	var r Result
	fileID := "file:" + p
	adr := ""
	if m := reADRFile.FindStringSubmatch(p); m != nil {
		adr = "ADR-" + m[1]
	}
	status := ""
	if m := reStatus.FindSubmatch(src); m != nil {
		status = string(m[1])
	}
	seen := map[string]int{}
	for i, h := range hs {
		ownEnd, end := total, total
		if i+1 < len(hs) {
			ownEnd = hs[i+1].line - 1
		}
		for _, n := range hs[i+1:] {
			if n.level <= h.level {
				end = n.line - 1
				break
			}
		}
		anchor := text.Slug(h.title)
		if c := seen[anchor]; c > 0 {
			seen[anchor] = c + 1
			anchor += "-" + strconv.Itoa(c)
		} else {
			seen[anchor] = 1
		}
		id := "doc:" + p + "#" + anchor
		kind := "section"
		meta := ""
		if num := text.SectionNumber(h.title); num != "" {
			meta = "§" + num
		}
		if adr != "" {
			kind = "adr-section"
			if h.level == 1 {
				kind = "adr"
				meta = status
			}
		}
		body := strings.Join(lines[h.line-1:min(ownEnd, len(lines))], "\n")
		n := Node{ID: id, Name: anchor, Kind: kind, Title: h.title, Start: h.line, OwnEnd: ownEnd, End: end, Body: body, Meta: meta}
		r.Nodes = append(r.Nodes, n)
		if adr != "" {
			alias := n
			alias.Body = "" // indexed once, under the doc: id
			if h.level == 1 {
				alias.ID, alias.Name = adr, adr
				alias.Start, alias.End = 1, total
			} else {
				alias.ID = adr + "#" + anchor
			}
			r.Nodes = append(r.Nodes, alias)
		}
		r.Edges = append(r.Edges, mentions(id, body, true)...)
		r.Edges = append(r.Edges, links(p, id, body)...)
		for _, n := range text.Issues(body) {
			r.Edges = append(r.Edges, Edge{id, "issue:" + n, "mentions"})
		}
	}
	if len(hs) == 0 || hs[0].line > 1 {
		end := total
		if len(hs) > 0 {
			end = hs[0].line - 1
		}
		body := text.Lines(src, 1, end)
		r.Nodes = append(r.Nodes, Node{ID: fileID, Name: path.Base(p), Kind: "file", Title: p, Start: 1, OwnEnd: end, End: total, Body: body})
		r.Edges = append(r.Edges, mentions(fileID, body, true)...)
	}
	for i, l := range lines {
		if m := reReqRow.FindStringSubmatch(l); m != nil {
			r.Nodes = append(r.Nodes, Node{ID: m[1], Name: m[1], Kind: "requirement", Title: text.Truncate(text.OneLine(l), 160), Start: i + 1, OwnEnd: i + 1, End: i + 1, Body: l})
		}
	}
	return r
}

// links turns the relative links of a section into edges to the files and sections they point at.
func links(p, src, body string) []Edge {
	var out []Edge
	for _, m := range reLink.FindAllStringSubmatch(body, -1) {
		target := m[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		file, anchor, _ := strings.Cut(target, "#")
		if file == "" {
			file = p
		} else {
			file = path.Clean(path.Join(path.Dir(p), file))
		}
		if strings.HasPrefix(file, "..") {
			continue
		}
		dst := "file:" + file
		if anchor != "" && strings.HasSuffix(file, ".md") {
			dst = "doc:" + file + "#" + anchor
		}
		out = append(out, Edge{src, dst, "links"})
	}
	return out
}
