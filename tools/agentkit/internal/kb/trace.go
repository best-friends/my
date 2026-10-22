// SPDX-License-Identifier: AGPL-3.0-only

package kb

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/best-friends/my/tools/agentkit/internal/store"
)

// layer orders the server's layers along a request: interface, app, adapter, query, table. Other code is 0
// and is not followed.
func layer(n store.Node) int {
	switch {
	case n.Kind == "op":
		return 0
	case n.Kind == "table":
		return 5
	case n.Kind == "query":
		return 4
	case n.Kind != "go":
		return -1
	case strings.HasPrefix(n.Path, "internal/interface/"):
		return 1
	case strings.HasPrefix(n.Path, "internal/app/"):
		return 2
	case strings.HasPrefix(n.Path, "internal/adapter/"):
		return 3
	}
	return -1
}

// Trace prints the path of a request from an operation (or a Go declaration) down to the tables, following
// calls by name into deeper layers only (and from one app module into another). Names that resolve to many declarations are left out as generic.
func (k *KB) Trace(w io.Writer, ref string, depth int) error {
	ns, err := k.Resolve(ref)
	if err != nil {
		return err
	}
	if len(ns) == 0 {
		return fmt.Errorf("nothing matches %q", ref)
	}
	root := ns[0]
	seen := map[string]bool{}
	fmt.Fprintln(w, k.Line(root))
	if root.Kind == "op" {
		if p := k.S.APIPath(root.Path); p != "" {
			fmt.Fprintf(w, "  %s %s\n", strings.ToUpper(root.Meta), p)
		}
		handlers, err := k.S.ByName(pascal(root.Name))
		if err != nil {
			return err
		}
		for _, h := range handlers {
			if layer(h) == 1 {
				fmt.Fprintln(w, "└ handler "+k.Line(h))
				seen[h.ID] = true
				if err := k.trace(w, h, 1, depth, seen); err != nil {
					return err
				}
			}
		}
		return nil
	}
	seen[root.ID] = true
	return k.trace(w, root, 0, depth, seen)
}

func (k *KB) trace(w io.Writer, n store.Node, level, depth int, seen map[string]bool) error {
	if level >= depth {
		return nil
	}
	es, err := k.S.Out(n.ID)
	if err != nil {
		return err
	}
	from := layer(n)
	indent := strings.Repeat("  ", level+1)
	for _, e := range es {
		var next []store.Node
		switch {
		case strings.HasPrefix(e.Dst, "name:"):
			cands, err := k.S.ByName(strings.TrimPrefix(e.Dst, "name:"))
			if err != nil {
				return err
			}
			for _, c := range cands {
				l := layer(c)
				crossModule := l == 2 && from == 2 && path.Dir(c.Path) != path.Dir(n.Path)
				if (l > from || crossModule) && l <= 4 && !seen[c.ID] {
					next = append(next, c)
				}
			}
			next = CollapseDialects(next)
			if len(next) > 4 || generic[strings.TrimPrefix(e.Dst, "name:")] {
				continue // a generic name, such as Get or Now
			}
		case strings.HasPrefix(e.Dst, "table:") && n.Kind == "query":
			ts, err := k.S.Get(e.Dst)
			if err != nil {
				return err
			}
			if ts = CollapseDialects(ts); len(ts) > 0 && !seen[e.Dst] {
				next = ts[:1]
			}
		}
		for _, c := range next {
			if seen[c.ID] && c.ID != e.Dst {
				continue // printed by a sibling's subtree meanwhile
			}
			seen[c.ID] = true
			seen[otherDialect(c.ID)] = true
			note := ""
			if c.Path != otherDialect(c.Path) {
				note = "  (+postgres)"
			}
			fmt.Fprintf(w, "%s└ %s%s\n", indent, k.Line(c), note)
			if err := k.trace(w, c, level+1, depth, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// generic names are helpers every layer calls; following them adds noise, not the path of a request.
var generic = map[string]bool{
	"Now": true, "NewID": true, "Normalize": true, "Emit": true, "New": true, "String": true, "Error": true,
	"Close": true, "Begin": true, "Commit": true, "Rollback": true, "WithTx": true, "Do": true, "Get": true,
	"Set": true, "Add": true, "Len": true, "Errorf": true, "Context": true, "Value": true, "Load": true,
}

// CollapseDialects keeps the SQLite one of a pair of declarations or queries that differ only by dialect.
func CollapseDialects(ns []store.Node) []store.Node {
	have := map[string]bool{}
	for _, n := range ns {
		have[n.ID+"\x00"+n.Path] = true
	}
	out := ns[:0]
	for _, n := range ns {
		if strings.Contains(n.Path, "postgres") && have[otherDialect(n.ID)+"\x00"+otherDialect(n.Path)] {
			continue
		}
		out = append(out, n)
	}
	return out
}

// otherDialect swaps sqlite for postgres in s; s without a dialect is returned as is.
func otherDialect(s string) string {
	return strings.NewReplacer("sqlite", "postgres", "postgres", "sqlite").Replace(s)
}

// pascal turns an operationId into the Go method oapi-codegen generates for it.
func pascal(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
