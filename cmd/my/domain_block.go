// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/domain/moderation"
)

// maxDomainBlockCSVInput bounds the CSV that `my admin domain import` reads: well above
// moderation.MaxDomainBlockImport rows with a comment.
const maxDomainBlockCSVInput = 8 << 20

// runAdminDomainCSV implements `my admin domain export` and `my admin domain import`: the
// domain block list in Mastodon's CSV (FR-706). The import previews by default and issues the
// actions only with -apply, for the rows that -only keeps.
func runAdminDomainCSV(sub string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var (
		apply bool
		only  string
		note  string
	)
	nargs := 1
	if sub == "export" {
		nargs = 0
	}
	c, code := parseSubcommand("my admin domain "+sub, args, nargs, stderr, adminDomainUsage, func(fs *flag.FlagSet) {
		if sub == "import" {
			fs.BoolVar(&apply, "apply", false, "issue the actions; without it, only show what the import would do")
			fs.StringVar(&only, "only", "", "import only these domains, separated by commas")
			fs.StringVar(&note, "private-note", "", "a note for the moderators on every action issued")
		}
	})
	if code >= 0 {
		return code
	}
	var text string
	if sub == "import" {
		var err error
		if text, err = readDomainBlockCSV(c.fs.Arg(0), stdin); err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %v\n", c.name, err)
			return 1
		}
	}
	ctx := context.Background()
	app, code := c.open(ctx, stderr)
	if code >= 0 {
		return code
	}
	defer func() { _ = app.Close() }()
	var err error
	if sub == "export" {
		err = app.Moderation.ExportDomainBlocks(ctx, stdout)
	} else {
		err = importDomainBlocks(ctx, app.Moderation, text, apply, only, note, stdout)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", c.name, err)
		return 1
	}
	return 0
}

// domainBlockImporter is the part of *appmoderation.Service that the import uses.
type domainBlockImporter interface {
	PreviewDomainBlocks(ctx context.Context, text string) ([]appmoderation.DomainBlockPreviewRow, error)
	ImportDomainBlocks(ctx context.Context, in appmoderation.DomainBlockImportInput) (appmoderation.DomainBlockImportResult, error)
}

// importDomainBlocks prints what importing each row of the CSV would do and, with apply,
// imports the importable rows whose domain (as written or canonical) only names, or all of them
// when only is empty.
func importDomainBlocks(ctx context.Context, m domainBlockImporter, text string, apply bool, only, note string, w io.Writer) error {
	rows, err := m.PreviewDomainBlocks(ctx, text)
	if err != nil {
		return err
	}
	var kept []string
	for d := range strings.SplitSeq(only, ",") {
		if d = strings.TrimSpace(d); d != "" {
			kept = append(kept, strings.ToLower(strings.TrimSuffix(d, ".")))
		}
	}
	chosen := func(r appmoderation.DomainBlockPreviewRow) bool {
		return len(kept) == 0 || slices.Contains(kept, strings.ToLower(strings.TrimSuffix(r.Written, "."))) || slices.Contains(kept, r.Block.Domain)
	}
	var blocks []moderation.DomainBlock
	for _, r := range rows {
		status := string(r.Status)
		if r.Status == appmoderation.DomainBlockImportable {
			kinds := make([]string, len(r.Issue))
			for i, k := range r.Issue {
				kinds[i] = string(k)
			}
			status = "issue " + strings.Join(kinds, ", ")
			if chosen(r) {
				blocks = append(blocks, r.Block)
			} else {
				status += " (not chosen)"
			}
		}
		_, _ = fmt.Fprintf(w, "line %d: %s: %s\n", r.Line, oneLine(r.Written), status)
	}
	if !apply {
		_, _ = fmt.Fprintf(w, "%d row(s) to import; run again with -apply to issue their actions\n", len(blocks))
		return nil
	}
	res, err := m.ImportDomainBlocks(ctx, appmoderation.DomainBlockImportInput{Blocks: blocks, PrivateNote: note})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "issued %d action(s), skipped %d row(s)\n", len(res.Issued), len(res.Skipped))
	for _, s := range res.Skipped {
		_, _ = fmt.Fprintf(w, "%s: %s\n", s.Domain, s.Reason)
	}
	return nil
}

// readDomainBlockCSV reads the CSV of an import from the file path, or from stdin for "-".
func readDomainBlockCSV(path string, stdin io.Reader) (string, error) {
	r := stdin
	if path != "-" {
		f, err := os.Open(path) //nolint:gosec // the operator names the file to import
		if err != nil {
			return "", err
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	b, err := io.ReadAll(io.LimitReader(r, maxDomainBlockCSVInput+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxDomainBlockCSVInput {
		return "", fmt.Errorf("the CSV is larger than %d bytes", maxDomainBlockCSVInput)
	}
	return string(b), nil
}
