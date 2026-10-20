// SPDX-License-Identifier: AGPL-3.0-only

package moderation

import (
	"cmp"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// The domain block list in Mastodon's CSV (FR-706): the file that Mastodon's moderation
// screens export and import, with the header
// "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate". A row is a
// Mastodon domain block, which this server keeps as up to three domain actions: the severity
// (suspend or limit) and the reject_media and reject_reports actions.

// DomainBlockSeverity is the #severity of a row of Mastodon's domain block CSV.
type DomainBlockSeverity string

// Severities of Mastodon's domain blocks.
const (
	// DomainBlockSuspend is the action suspend.
	DomainBlockSuspend DomainBlockSeverity = "suspend"
	// DomainBlockSilence is Mastodon's name of the action limit.
	DomainBlockSilence DomainBlockSeverity = "silence"
	// DomainBlockNoop is no action but the rejections of the row.
	DomainBlockNoop DomainBlockSeverity = "noop"
)

// Valid reports whether s is one of the severities.
func (s DomainBlockSeverity) Valid() bool {
	return s == DomainBlockSuspend || s == DomainBlockSilence || s == DomainBlockNoop
}

// DomainBlockCSVHeader is the header of the CSV, in Mastodon's order.
var DomainBlockCSVHeader = []string{"#domain", "#severity", "#reject_media", "#reject_reports", "#public_comment", "#obfuscate"}

// MaxDomainBlockImport is the number of rows that one import of a domain block CSV may hold.
const MaxDomainBlockImport = 5000

// ErrMalformedDomainBlockCSV reports a file that is not CSV, such as one with a stray quote.
var ErrMalformedDomainBlockCSV = errors.New("moderation: malformed domain block CSV")

// ErrTooManyDomainBlocks reports a CSV of more than MaxDomainBlockImport rows.
var ErrTooManyDomainBlocks = fmt.Errorf("moderation: more than %d domain blocks to import", MaxDomainBlockImport)

// DomainBlock is one row of Mastodon's domain block CSV.
type DomainBlock struct {
	Domain        string
	Severity      DomainBlockSeverity
	RejectMedia   bool
	RejectReports bool
	PublicComment string
	Obfuscate     bool
}

// Kinds returns the kinds of the domain actions that b stands for, the severity first: suspend
// for suspend, limit for silence, none for noop, then reject_media and reject_reports when set.
func (b DomainBlock) Kinds() []DomainActionKind {
	var out []DomainActionKind
	switch b.Severity {
	case DomainBlockSuspend:
		out = append(out, DomainSuspend)
	case DomainBlockSilence:
		out = append(out, DomainLimit)
	}
	if b.RejectMedia {
		out = append(out, DomainRejectMedia)
	}
	if b.RejectReports {
		out = append(out, DomainRejectReports)
	}
	return out
}

// DomainBlockRow is a row of a parsed CSV: its line, from 1, and its block. Invalid reports a row
// whose #severity is not one of the severities; its Severity is then as written.
type DomainBlockRow struct {
	Line int
	DomainBlock
	Invalid bool
}

// ParseDomainBlockCSV parses Mastodon's domain block CSV as Mastodon imports it. A file whose
// first field is "#domain" has a header, which names the columns in any order (the unknown
// ones are ignored); any other file is a list of domains, one per row in the first column, as
// Mastodon also accepts. A missing or empty #severity is suspend, and the booleans are false
// when empty, "0", "f", "false", or "off" in any case, and true otherwise. Rows with no field
// are skipped. The domain and the comment are trimmed, not checked: the caller canonicalizes
// the domain. It fails with ErrMalformedDomainBlockCSV for a file that is not CSV, and with
// ErrTooManyDomainBlocks for more than MaxDomainBlockImport rows.
func ParseDomainBlockCSV(text string) ([]DomainBlockRow, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\uFEFF")))
	r.FieldsPerRecord = -1
	column := map[string]int{"#domain": 0}
	var out []DomainBlockRow
	first := true
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMalformedDomainBlockCSV, err)
		}
		line, _ := r.FieldPos(0)
		if first {
			first = false
			if strings.TrimSpace(rec[0]) == "#domain" {
				for i, name := range rec {
					name = strings.ToLower(strings.TrimSpace(name))
					if _, ok := column[name]; !ok {
						column[name] = i
					}
				}
				continue
			}
		}
		if blankRecord(rec) {
			continue
		}
		if len(out) == MaxDomainBlockImport {
			return nil, ErrTooManyDomainBlocks
		}
		field := func(name string) string {
			i, ok := column[name]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		row := DomainBlockRow{Line: line, DomainBlock: DomainBlock{
			Domain:        field("#domain"),
			Severity:      DomainBlockSeverity(strings.ToLower(field("#severity"))),
			RejectMedia:   csvBool(field("#reject_media")),
			RejectReports: csvBool(field("#reject_reports")),
			PublicComment: field("#public_comment"),
			Obfuscate:     csvBool(field("#obfuscate")),
		}}
		if row.Severity == "" {
			row.Severity = DomainBlockSuspend
		}
		row.Invalid = !row.Severity.Valid()
		out = append(out, row)
	}
	return out, nil
}

func blankRecord(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

// csvBool reads a boolean of the CSV as Rails casts it.
func csvBool(s string) bool {
	switch strings.ToLower(s) {
	case "", "0", "f", "false", "off":
		return false
	}
	return true
}

// WriteDomainBlockCSV writes blocks as Mastodon's domain block CSV, with its header and the
// booleans as "true" and "false".
func WriteDomainBlockCSV(w io.Writer, blocks []DomainBlock) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(DomainBlockCSVHeader); err != nil {
		return err
	}
	for _, b := range blocks {
		rec := []string{
			b.Domain, string(b.Severity), strconv.FormatBool(b.RejectMedia), strconv.FormatBool(b.RejectReports),
			b.PublicComment, strconv.FormatBool(b.Obfuscate),
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// DomainBlocksOf returns the domain actions as Mastodon's domain blocks, one per domain in
// domain order: the severity is suspend when a suspend is among the actions on the domain,
// silence when a limit is, and noop otherwise; reject_media, reject_reports, and obfuscate are
// set when one of the actions has them. The public comment is that of the action that gives the
// severity, or else the first non-empty one. A domain whose only actions are force_sensitive,
// which Mastodon's domain blocks cannot hold, is left out. The caller passes the actions in
// effect.
func DomainBlocksOf(actions []DomainAction) []DomainBlock {
	byDomain := make(map[string][]DomainAction)
	for _, a := range actions {
		byDomain[a.Domain] = append(byDomain[a.Domain], a)
	}
	out := make([]DomainBlock, 0, len(byDomain))
	for domain, as := range byDomain {
		b := DomainBlock{Domain: domain, Severity: DomainBlockNoop}
		var severityComment, otherComment string
		for _, a := range as {
			switch a.Kind {
			case DomainSuspend:
				b.Severity, severityComment = DomainBlockSuspend, a.PublicComment
			case DomainLimit:
				if b.Severity != DomainBlockSuspend {
					b.Severity, severityComment = DomainBlockSilence, a.PublicComment
				}
			case DomainRejectMedia:
				b.RejectMedia = true
			case DomainRejectReports:
				b.RejectReports = true
			}
			b.Obfuscate = b.Obfuscate || a.Obfuscate
			if otherComment == "" {
				otherComment = a.PublicComment
			}
		}
		if len(b.Kinds()) == 0 {
			continue
		}
		b.PublicComment = cmp.Or(severityComment, otherComment)
		out = append(out, b)
	}
	slices.SortFunc(out, func(a, b DomainBlock) int { return strings.Compare(a.Domain, b.Domain) })
	return out
}

// CheckDomainActionTexts checks a public comment and a private note of a domain action as
// DomainAction.Validate does, returning a *TextError wrapped with ErrInvalidAction.
func CheckDomainActionTexts(publicComment, privateNote string) error {
	return checkTexts(textField{"public_comment", publicComment}, textField{"private_note", privateNote})
}
