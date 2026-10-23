// SPDX-License-Identifier: AGPL-3.0-only

package timeline

import (
	"context"
	"errors"
	"slices"
	"strings"

	appcontent "github.com/mycroblog/my/internal/app/content"
)

// The prefix search of hashtags for the autocomplete of the compose form and the hashtag search
// (ADR-0092 §1, FR-119, FR-306): the tags whose normalized name starts with the query and that
// a post of visibility public, local, or unlisted carries now, in the order exact match, most
// posts, most recently used.

// Limits of the prefix search of hashtags.
const (
	DefaultTagSearchLimit = 8
	MaxTagSearchLimit     = 20
	// MaxTagSearchQueryLength is the longest query, in characters, after its leading #.
	MaxTagSearchQueryLength = 100
	// tagSearchCandidates is how many tags one search reads from the start of the range.
	tagSearchCandidates = 100
)

// ErrInvalidTagSearchQuery reports a query that is empty or longer than MaxTagSearchQueryLength
// characters after its leading #.
var ErrInvalidTagSearchQuery = errors.New("timeline: invalid tag search query")

// TagReader reads the hashtags of the content module by the prefix of their normalized name.
// appcontent.Repository implements it.
type TagReader interface {
	ListTagsByPrefix(ctx context.Context, low, high string, limit int) ([]appcontent.Tag, error)
}

// TagSuggestion is a result of the prefix search of hashtags: the tag's name as first written
// and how many posts that CountsTagUsage carry it.
type TagSuggestion struct {
	ID            string
	Name          string
	StatusesCount int64
}

// TagSearch is the prefix search of hashtags.
type TagSearch struct {
	tags TagReader
	repo Repository
}

// NewTagSearch returns the search of the tags of tags, counted in the tag_usage of repo.
func NewTagSearch(tags TagReader, repo Repository) *TagSearch {
	return &TagSearch{tags: tags, repo: repo}
}

// NormalizeTagSearchQuery returns the form of a query that the search compares with the tags:
// without its leading #, under appcontent.NormalizeTag. It fails with ErrInvalidTagSearchQuery
// when the query without the # is empty or longer than MaxTagSearchQueryLength characters.
func NormalizeTagSearchQuery(q string) (string, error) {
	q = strings.TrimPrefix(q, "#")
	if q == "" || len([]rune(q)) > MaxTagSearchQueryLength {
		return "", ErrInvalidTagSearchQuery
	}
	n := appcontent.NormalizeTag(q)
	if strings.TrimSpace(n) == "" {
		return "", ErrInvalidTagSearchQuery
	}
	return n, nil
}

// Search returns up to limit tags (DefaultTagSearchLimit when zero or less, at most
// MaxTagSearchLimit) whose normalized name starts with q. It reads the first tagSearchCandidates
// tags of the range in name order, keeps those with a tag_usage row, and orders them: the exact
// match first, then the tags with more posts, then the more recently used, then by name.
func (s *TagSearch) Search(ctx context.Context, q string, limit int) ([]TagSuggestion, error) {
	n, err := NormalizeTagSearchQuery(q)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultTagSearchLimit
	}
	limit = min(limit, MaxTagSearchLimit)
	tags, err := s.tags.ListTagsByPrefix(ctx, n, n+"\U0010FFFF", tagSearchCandidates)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return []TagSuggestion{}, nil
	}
	ids := make([]string, len(tags))
	for i, t := range tags {
		ids[i] = t.ID
	}
	usage, err := s.repo.TagUsageOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]TagUsage, len(usage))
	for _, u := range usage {
		byID[u.TagID] = u
	}

	type candidate struct {
		tag   appcontent.Tag
		usage TagUsage
	}
	var cands []candidate
	for _, t := range tags {
		if u, ok := byID[t.ID]; ok {
			cands = append(cands, candidate{tag: t, usage: u})
		}
	}
	// The tags come in name order, so the stable sort keeps it among equals.
	slices.SortStableFunc(cands, func(a, b candidate) int {
		if ae, be := a.tag.NameNormalized == n, b.tag.NameNormalized == n; ae != be {
			if ae {
				return -1
			}
			return 1
		}
		if a.usage.StatusesCount != b.usage.StatusesCount {
			if a.usage.StatusesCount > b.usage.StatusesCount {
				return -1
			}
			return 1
		}
		return b.usage.LastUsedAt.Compare(a.usage.LastUsedAt)
	})
	out := make([]TagSuggestion, 0, min(limit, len(cands)))
	for _, c := range cands[:min(limit, len(cands))] {
		out = append(out, TagSuggestion{ID: c.tag.ID, Name: c.tag.Name, StatusesCount: c.usage.StatusesCount})
	}
	return out, nil
}
