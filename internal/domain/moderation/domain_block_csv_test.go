// SPDX-License-Identifier: AGPL-3.0-only

package moderation_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/domain/moderation"
)

// mastodonExport is a domain block CSV as Mastodon 4 exports it.
const mastodonExport = "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate\r\n" +
	"spam.example,suspend,false,false,Spam,true\r\n" +
	"noisy.example,silence,true,false,\"Harassment, mostly\",false\r\n" +
	"media.example,noop,true,true,,false\r\n"

func TestParseDomainBlockCSV(t *testing.T) {
	block := func(line int, b moderation.DomainBlock) moderation.DomainBlockRow {
		return moderation.DomainBlockRow{Line: line, DomainBlock: b}
	}
	for _, tc := range []struct {
		name, text string
		want       []moderation.DomainBlockRow
	}{
		{"mastodon's export", mastodonExport, []moderation.DomainBlockRow{
			block(2, moderation.DomainBlock{Domain: "spam.example", Severity: moderation.DomainBlockSuspend, PublicComment: "Spam", Obfuscate: true}),
			block(3, moderation.DomainBlock{Domain: "noisy.example", Severity: moderation.DomainBlockSilence, RejectMedia: true, PublicComment: "Harassment, mostly"}),
			block(4, moderation.DomainBlock{Domain: "media.example", Severity: moderation.DomainBlockNoop, RejectMedia: true, RejectReports: true}),
		}},
		{"columns in another order, extra and missing ones", "\uFEFF#domain,#note,#obfuscate,#severity\n bad.example ,x,1,SILENCE\n\n,,,\nworse.example,,,\n", []moderation.DomainBlockRow{
			block(2, moderation.DomainBlock{Domain: "bad.example", Severity: moderation.DomainBlockSilence, Obfuscate: true}),
			block(5, moderation.DomainBlock{Domain: "worse.example", Severity: moderation.DomainBlockSuspend}),
		}},
		{"a list of domains", "bad.example\nworse.example,whatever\n", []moderation.DomainBlockRow{
			block(1, moderation.DomainBlock{Domain: "bad.example", Severity: moderation.DomainBlockSuspend}),
			block(2, moderation.DomainBlock{Domain: "worse.example", Severity: moderation.DomainBlockSuspend}),
		}},
		{"booleans as rails casts them", "#domain,#reject_media,#reject_reports,#obfuscate\na.example,F,off,0\nb.example,yes,t,TRUE\n", []moderation.DomainBlockRow{
			block(2, moderation.DomainBlock{Domain: "a.example", Severity: moderation.DomainBlockSuspend}),
			block(3, moderation.DomainBlock{Domain: "b.example", Severity: moderation.DomainBlockSuspend, RejectMedia: true, RejectReports: true, Obfuscate: true}),
		}},
		{"an unknown severity", "#domain,#severity\nodd.example,ban\n", []moderation.DomainBlockRow{
			{Line: 2, DomainBlock: moderation.DomainBlock{Domain: "odd.example", Severity: "ban"}, Invalid: true},
		}},
		{"a comment over lines", "#domain,#public_comment\na.example,\"one\ntwo\"\nb.example,\n", []moderation.DomainBlockRow{
			block(2, moderation.DomainBlock{Domain: "a.example", Severity: moderation.DomainBlockSuspend, PublicComment: "one\ntwo"}),
			block(4, moderation.DomainBlock{Domain: "b.example", Severity: moderation.DomainBlockSuspend}),
		}},
		{"nothing", "", nil},
		{"only the header", "#domain,#severity\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := moderation.ParseDomainBlockCSV(tc.text)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseDomainBlockCSV = %+v, %v\nwant %+v", got, err, tc.want)
			}
		})
	}
}

func TestParseDomainBlockCSVFails(t *testing.T) {
	if _, err := moderation.ParseDomainBlockCSV("a.example,\"b\"c\n"); !errors.Is(err, moderation.ErrMalformedDomainBlockCSV) {
		t.Errorf("a bare quote = %v, want ErrMalformedDomainBlockCSV", err)
	}
	if _, err := moderation.ParseDomainBlockCSV("\"a.example\n"); !errors.Is(err, moderation.ErrMalformedDomainBlockCSV) {
		t.Errorf("an unterminated quote = %v, want ErrMalformedDomainBlockCSV", err)
	}
	full := "#domain\n" + strings.Repeat("a.example\n", moderation.MaxDomainBlockImport)
	if rows, err := moderation.ParseDomainBlockCSV(full); err != nil || len(rows) != moderation.MaxDomainBlockImport {
		t.Errorf("%d rows = %d, %v", moderation.MaxDomainBlockImport, len(rows), err)
	}
	if _, err := moderation.ParseDomainBlockCSV(full + "b.example\n"); !errors.Is(err, moderation.ErrTooManyDomainBlocks) {
		t.Errorf("one row too many = %v, want ErrTooManyDomainBlocks", err)
	}
}

func TestDomainBlockKinds(t *testing.T) {
	for b, want := range map[moderation.DomainBlock][]moderation.DomainActionKind{
		{Severity: moderation.DomainBlockSuspend}:                                       {moderation.DomainSuspend},
		{Severity: moderation.DomainBlockSilence, RejectMedia: true}:                    {moderation.DomainLimit, moderation.DomainRejectMedia},
		{Severity: moderation.DomainBlockNoop, RejectMedia: true, RejectReports: true}:  {moderation.DomainRejectMedia, moderation.DomainRejectReports},
		{Severity: moderation.DomainBlockSuspend, RejectReports: true, Obfuscate: true}: {moderation.DomainSuspend, moderation.DomainRejectReports},
		{Severity: moderation.DomainBlockNoop}:                                          nil,
	} {
		if got := b.Kinds(); !reflect.DeepEqual(got, want) {
			t.Errorf("%+v.Kinds() = %v, want %v", b, got, want)
		}
	}
}

func TestDomainBlocksOfAndBack(t *testing.T) {
	actions := []moderation.DomainAction{
		{Domain: "spam.example", Kind: moderation.DomainLimit, PublicComment: "first a limit"},
		{Domain: "spam.example", Kind: moderation.DomainSuspend, PublicComment: "Spam", Obfuscate: true},
		{Domain: "noisy.example", Kind: moderation.DomainRejectMedia, PublicComment: "Harassment, mostly"},
		{Domain: "noisy.example", Kind: moderation.DomainLimit},
		{Domain: "media.example", Kind: moderation.DomainRejectReports},
		{Domain: "media.example", Kind: moderation.DomainRejectMedia},
		{Domain: "media.example", Kind: moderation.DomainForceSensitive},
		{Domain: "nsfw.example", Kind: moderation.DomainForceSensitive},
	}
	blocks := moderation.DomainBlocksOf(actions)
	var b strings.Builder
	if err := moderation.WriteDomainBlockCSV(&b, blocks); err != nil {
		t.Fatal(err)
	}
	want := "#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate\n" +
		"media.example,noop,true,true,,false\n" +
		"noisy.example,silence,true,false,\"Harassment, mostly\",false\n" +
		"spam.example,suspend,false,false,Spam,true\n"
	if b.String() != want {
		t.Errorf("CSV =\n%s\nwant\n%s", b.String(), want)
	}
	// What Mastodon imports back is the same blocks.
	rows, err := moderation.ParseDomainBlockCSV(b.String())
	if err != nil || len(rows) != len(blocks) {
		t.Fatalf("parse = %+v, %v", rows, err)
	}
	for i, r := range rows {
		if r.Invalid || r.DomainBlock != blocks[i] {
			t.Errorf("row %d = %+v, want %+v", i, r, blocks[i])
		}
	}
	// So does Mastodon's own export, written again.
	parsed, _ := moderation.ParseDomainBlockCSV(mastodonExport)
	again := make([]moderation.DomainBlock, len(parsed))
	for i, r := range parsed {
		again[i] = r.DomainBlock
	}
	b.Reset()
	if err := moderation.WriteDomainBlockCSV(&b, again); err != nil {
		t.Fatal(err)
	}
	if got := strings.ReplaceAll(mastodonExport, "\r\n", "\n"); b.String() != got {
		t.Errorf("Mastodon's export written again =\n%s\nwant\n%s", b.String(), got)
	}
}
