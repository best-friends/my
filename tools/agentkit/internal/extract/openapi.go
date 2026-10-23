// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"path"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/best-friends/my/tools/agentkit/internal/text"
)

var (
	reMethod = regexp.MustCompile(`^(get|put|post|delete|patch|options|head):\s*$`)
	reRef    = regexp.MustCompile(`\$ref:\s*["']?([^"'\s#]+)`)

	reDefaultMessage = regexp.MustCompile(`"defaultMessage":\s*"((?:[^"\\]|\\.)*)"`)
)

// OpenAPIRoot records which path file serves which API path: an edge file:<path file> -> apipath:<path>.
func OpenAPIRoot(p string, src []byte) Result {
	var doc struct {
		Paths map[string]struct {
			Ref string `yaml:"$ref"`
		} `yaml:"paths"`
	}
	var r Result
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return r
	}
	for apiPath, v := range doc.Paths {
		if v.Ref != "" {
			r.Edges = append(r.Edges, Edge{"file:" + path.Clean(path.Join(path.Dir(p), v.Ref)), "apipath:" + apiPath, "serves"})
		}
	}
	return r
}

// OpenAPIPath makes one node per operation of a path file; the node's Meta is its HTTP method.
func OpenAPIPath(_ string, src []byte) Result {
	var doc map[string]struct {
		OperationID string `yaml:"operationId"`
		Summary     string `yaml:"summary"`
	}
	var r Result
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return r
	}
	lines := strings.Split(string(src), "\n")
	var starts []int
	for i, l := range lines {
		if reMethod.MatchString(l) {
			starts = append(starts, i)
		}
	}
	for j, s := range starts {
		method := strings.TrimSuffix(strings.TrimSpace(lines[s]), ":")
		op, ok := doc[method]
		if !ok || op.OperationID == "" {
			continue
		}
		end := len(lines)
		if j+1 < len(starts) {
			end = starts[j+1]
		}
		body := strings.Join(lines[s:end], "\n")
		id := "op:" + op.OperationID
		r.Nodes = append(r.Nodes, Node{ID: id, Name: op.OperationID, Kind: "op", Title: strings.ToUpper(method) + " " + op.Summary, Start: s + 1, OwnEnd: end, End: end, Body: body, Meta: method})
		r.Edges = append(r.Edges, mentions(id, body, false)...)
		r.Edges = append(r.Edges, refs(id, body)...)
	}
	return r
}

// OpenAPIComponent makes one node for a component file (schema:<Name> for every kind of component).
func OpenAPIComponent(p string, src []byte) Result {
	name := strings.TrimSuffix(path.Base(p), path.Ext(p))
	id := "schema:" + name
	body := string(src)
	r := Result{Nodes: []Node{{ID: id, Name: name, Kind: "schema", Title: path.Base(path.Dir(p)) + " " + name, Start: 1, OwnEnd: lineCount(src), End: lineCount(src), Body: body}}}
	r.Edges = append(r.Edges, mentions(id, body, false)...)
	r.Edges = append(r.Edges, refs(id, body)...)
	return r
}

func refs(src, body string) []Edge {
	var out []Edge
	seen := map[string]bool{}
	for _, m := range reRef.FindAllStringSubmatch(body, -1) {
		n := strings.TrimSuffix(path.Base(m[1]), path.Ext(m[1]))
		if !seen[n] {
			seen[n] = true
			out = append(out, Edge{src, "schema:" + n, "refs"})
		}
	}
	return out
}

// Locale makes one node per message of a catalog file locales/<locale>/<namespace>/<area>.json:
// msg:<namespace>:<key>@<locale>, titled by its English or Japanese text.
func Locale(p string, src []byte) Result {
	segments := strings.Split(p, "/")
	if len(segments) < 4 {
		return Result{}
	}
	locale, ns := segments[1], segments[2]
	lines := strings.Split(string(src), "\n")
	var r Result
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if !strings.HasPrefix(l, `  "`) || !strings.HasSuffix(strings.TrimSpace(l), "{") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimSpace(l), `": {`)
		key = strings.TrimPrefix(key, `"`)
		end := i
		for end < len(lines)-1 && !strings.HasPrefix(lines[end], "  }") {
			end++
		}
		body := strings.Join(lines[i:end+1], "\n")
		title := ""
		if m := reDefaultMessage.FindStringSubmatch(body); m != nil {
			title = text.Truncate(m[1], 120)
		}
		r.Nodes = append(r.Nodes, Node{ID: "msg:" + ns + ":" + key + "@" + locale, Name: key, Kind: "msg", Title: title, Start: i + 1, OwnEnd: end + 1, End: end + 1, Body: body, Meta: locale})
		i = end
	}
	return r
}
