// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"

	"github.com/best-friends/my/tools/agentkit/internal/text"
)

const modulePath = "github.com/best-friends/my/"

// Go makes one node per top-level declaration (its doc comment included) and a file node for the package
// clause and imports. Calls by name become name: edges, imports of the repository's packages pkg: edges.
func Go(p string, src []byte) Result {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return Result{}
	}
	dir := path.Dir(p)
	kind := "go"
	if strings.HasSuffix(p, "_test.go") {
		kind = "go-test"
	}
	var r Result
	fileID := "file:" + p
	headEnd := fset.Position(f.Name.End()).Line
	if len(f.Imports) > 0 {
		headEnd = fset.Position(f.Imports[len(f.Imports)-1].End()).Line + 1
	}
	head := text.Lines(src, 1, headEnd)
	r.Nodes = append(r.Nodes, Node{ID: fileID, Name: path.Base(p), Kind: "file", Title: "package " + f.Name.Name, Start: 1, OwnEnd: headEnd, End: lineCount(src), Body: head})
	r.Edges = append(r.Edges, mentions(fileID, head, false)...)
	for _, imp := range f.Imports {
		ip, _ := strconv.Unquote(imp.Path.Value)
		if strings.HasPrefix(ip, modulePath) {
			r.Edges = append(r.Edges, Edge{fileID, "pkg:" + strings.TrimPrefix(ip, modulePath), "imports"})
		}
	}
	for _, d := range f.Decls {
		start := d.Pos()
		var names []string
		var doc *ast.CommentGroup
		title := ""
		switch d := d.(type) {
		case *ast.FuncDecl:
			doc = d.Doc
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = recvName(d.Recv.List[0].Type) + "." + name
			}
			names = []string{name}
			title = firstLine(src, fset, d.Pos())
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			doc = d.Doc
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name != "_" {
							names = append(names, n.Name)
						}
					}
				}
			}
			title = firstLine(src, fset, d.Pos())
		}
		if len(names) == 0 {
			continue
		}
		if doc != nil {
			start = doc.Pos()
		}
		sl, el := fset.Position(start).Line, fset.Position(d.End()).Line
		body := text.Lines(src, sl, el)
		// A const or var block declares several names; the first names the node, and the others resolve to it.
		id := "sym:" + dir + "." + names[0]
		short := names[0]
		if i := strings.LastIndexByte(short, '.'); i >= 0 {
			short = short[i+1:]
		}
		r.Nodes = append(r.Nodes, Node{ID: id, Name: short, Kind: kind, Title: text.Truncate(title, 160), Start: sl, OwnEnd: el, End: el, Body: body, Meta: names[0]})
		for _, extra := range names[1:] {
			r.Nodes = append(r.Nodes, Node{ID: "sym:" + dir + "." + extra, Name: extra, Kind: kind, Title: text.Truncate(title, 160), Start: sl, OwnEnd: el, End: el, Meta: extra})
		}
		r.Edges = append(r.Edges, mentions(id, body, false)...)
		for _, c := range calls(d) {
			r.Edges = append(r.Edges, Edge{id, "name:" + c, "calls"})
		}
	}
	return r
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

func firstLine(src []byte, fset *token.FileSet, pos token.Pos) string {
	l := fset.Position(pos).Line
	return strings.TrimSpace(strings.TrimSuffix(text.Lines(src, l, l), "{\n"))
}

// calls lists the exported names a declaration calls, by selector or identifier.
func calls(n ast.Node) []string {
	seen := map[string]bool{}
	var out []string
	ast.Inspect(n, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch f := c.Fun.(type) {
		case *ast.SelectorExpr:
			name = f.Sel.Name
		case *ast.Ident:
			name = f.Name
		}
		if len(name) > 2 && ast.IsExported(name) && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
		return true
	})
	return out
}
