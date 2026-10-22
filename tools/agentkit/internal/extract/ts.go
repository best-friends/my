// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"path"
	"regexp"
	"strings"

	"github.com/best-friends/my/tools/agentkit/internal/text"
)

var (
	reTSDecl   = regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:async\s+)?(?:function\*?|const|let|class|interface|type|enum)\s+([A-Za-z_$][\w$]*)`)
	reTSImport = regexp.MustCompile(`from\s+["'](\.{1,2}/[^"']+)["']`)
	reTSJSX    = regexp.MustCompile(`<([A-Z][A-Za-z0-9]+)[\s/>]`)
	reTSHook   = regexp.MustCompile(`\b(use[A-Z][A-Za-z0-9]+)\(`)
	reTSAPI1   = regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE)\(\s*"(/[^"]*)"`)
	reTSAPI2   = regexp.MustCompile(`"(get|post|put|patch|delete)",\s*"(/[^"]*)"`)
)

// TS makes one node per top-level declaration of a TypeScript file (a declaration runs to the next one, with
// the comment above it), a file node for the imports, and edges for relative imports, the components and
// hooks it uses, and the client API paths it calls.
func TS(p string, src []byte) Result {
	lines := strings.Split(string(src), "\n")
	type decl struct {
		name  string
		start int // first line of the comment above, 1-based
		line  int
	}
	var ds []decl
	for i, l := range lines {
		if m := reTSDecl.FindStringSubmatch(l); m != nil {
			s := i
			for s > 0 && isCommentLine(lines[s-1]) {
				s--
			}
			ds = append(ds, decl{m[1], s + 1, i + 1})
		}
	}
	kind := "ts"
	if strings.Contains(p, ".test.") || strings.Contains(p, "/e2e/") || strings.Contains(p, ".stories.") {
		kind = "ts-test"
	}
	var r Result
	fileID := "file:" + p
	headEnd := len(lines)
	if len(ds) > 0 {
		headEnd = ds[0].start - 1
	}
	head := text.Lines(src, 1, headEnd)
	r.Nodes = append(r.Nodes, Node{ID: fileID, Name: path.Base(p), Kind: "file", Title: p, Start: 1, OwnEnd: headEnd, End: lineCount(src), Body: head})
	for _, m := range reTSImport.FindAllStringSubmatch(head, -1) {
		r.Edges = append(r.Edges, Edge{fileID, "file:" + path.Clean(path.Join(path.Dir(p), m[1])), "imports"})
	}
	for i, d := range ds {
		end := lineCount(src)
		if i+1 < len(ds) {
			end = ds[i+1].start - 1
		}
		for end > d.line && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		body := text.Lines(src, d.start, end)
		id := "ts:" + p + ":" + d.name
		r.Nodes = append(r.Nodes, Node{ID: id, Name: d.name, Kind: kind, Title: text.Truncate(strings.TrimSpace(lines[d.line-1]), 160), Start: d.start, OwnEnd: end, End: end, Body: body})
		r.Edges = append(r.Edges, mentions(id, body, false)...)
		seen := map[string]bool{}
		add := func(dst, rel string) {
			if !seen[dst] {
				seen[dst] = true
				r.Edges = append(r.Edges, Edge{id, dst, rel})
			}
		}
		for _, m := range reTSJSX.FindAllStringSubmatch(body, -1) {
			add("name:"+m[1], "renders")
		}
		for _, m := range reTSHook.FindAllStringSubmatch(body, -1) {
			add("name:"+m[1], "calls")
		}
		for _, m := range reTSAPI1.FindAllStringSubmatch(body, -1) {
			add(APIKey(m[1], m[2]), "calls-api")
		}
		for _, m := range reTSAPI2.FindAllStringSubmatch(body, -1) {
			add(APIKey(m[1], m[2]), "calls-api")
		}
	}
	return r
}

// APIKey is the edge destination of a client API call: api:<method> <path below /api/v1>.
func APIKey(method, p string) string {
	return "api:" + strings.ToLower(method) + " " + strings.TrimPrefix(p, "/api/v1")
}

func isCommentLine(l string) bool {
	t := strings.TrimSpace(l)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "*")
}
