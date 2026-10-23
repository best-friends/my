// SPDX-License-Identifier: AGPL-3.0-only

// Package stats measures how coding agents spent tokens in this repository, from the Claude Code transcripts
// under ~/.claude/projects: the API usage, and which tool outputs filled the context. It is how the effect of
// agentkit is checked: compare a period before and after a change with --since and --until.
package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Options select the transcripts.
type Options struct {
	Dirs  []string // transcript directories
	Since string   // ISO date or time; lines before it are skipped
	Until string
	Top   int
}

// ProjectDirs returns the transcript directories of the working tree root and its worktrees.
func ProjectDirs(root string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	enc := regexp.MustCompile(`[^A-Za-z0-9-]`).ReplaceAllString(root, "-")
	return filepath.Glob(filepath.Join(home, ".claude", "projects", enc+"*"))
}

type usage struct {
	Input, CacheWrite, CacheRead, Output int64
}

type line struct {
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			Input      int64 `json:"input_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
			Output     int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

var categories = []struct {
	name string
	re   *regexp.Regexp
}{
	{"bash:agentkit", regexp.MustCompile(`(^|[\s/])agentkit(\.sh)?\s`)},
	{"bash:ci watch", regexp.MustCompile(`gh (pr checks|run (watch|view))`)},
	{"bash:gh issue", regexp.MustCompile(`\bgh issue`)},
	{"bash:gh pr", regexp.MustCompile(`\bgh pr`)},
	{"bash:gh other", regexp.MustCompile(`\bgh `)},
	{"bash:task check", regexp.MustCompile(`task\.sh (check|[a-z0-9]+:(check|test|lint)\b)`)},
	{"bash:task other", regexp.MustCompile(`\btask\.sh `)},
	{"bash:grep/rg/find", regexp.MustCompile(`\b(grep|rg|find)\b`)},
	{"bash:cat/sed/head", regexp.MustCompile(`^\s*(cat|sed|head|tail|awk)\b`)},
	{"bash:git", regexp.MustCompile(`(^|&& )git `)},
	{"bash:go", regexp.MustCompile(`\bgo (test|vet|build|run)`)},
	{"bash:bun/node", regexp.MustCompile(`\b(bun|bunx|vitest|npx|node)\b`)},
}

func category(name string, input json.RawMessage) string {
	if name != "Bash" {
		return "tool:" + name
	}
	var in struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(input, &in)
	for _, c := range categories {
		if c.re.MatchString(in.Command) {
			return c.name
		}
	}
	return "bash:other"
}

func resultSize(raw json.RawMessage) int {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return len(s)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		n := 0
		for _, p := range parts {
			n += len(p.Text)
		}
		return n
	}
	return 0
}

// Report prints the usage and the tool output by category of the transcripts in o.
func Report(w io.Writer, o Options) error {
	var files []string
	for _, d := range o.Dirs {
		_ = filepath.WalkDir(d, func(p string, e os.DirEntry, err error) error {
			if err == nil && !e.IsDir() && strings.HasSuffix(p, ".jsonl") {
				files = append(files, p)
			}
			return nil
		})
	}
	var u usage
	turns, sessions := 0, 0
	seen := map[string]bool{}
	size := map[string]int{}
	count := map[string]int{}
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		uses := map[string]string{}
		active := false
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			var l line
			if json.Unmarshal(sc.Bytes(), &l) != nil {
				continue
			}
			if (o.Since != "" && l.Timestamp < o.Since) || (o.Until != "" && l.Timestamp >= o.Until) {
				continue
			}
			active = true
			if us := l.Message.Usage; us != nil && !seen[l.Message.ID] {
				seen[l.Message.ID] = true
				turns++
				u.Input += us.Input
				u.CacheWrite += us.CacheWrite
				u.CacheRead += us.CacheRead
				u.Output += us.Output
			}
			var blocks []block
			if json.Unmarshal(l.Message.Content, &blocks) != nil {
				continue
			}
			for _, b := range blocks {
				switch b.Type {
				case "tool_use":
					uses[b.ID] = category(b.Name, b.Input)
				case "tool_result":
					c, ok := uses[b.ToolUseID]
					if !ok {
						c = "unknown"
					}
					size[c] += resultSize(b.Content)
					count[c]++
				}
			}
		}
		_ = fh.Close()
		if active {
			sessions++
		}
	}
	total := 0
	for _, v := range size {
		total += v
	}
	fmt.Fprintf(w, "%d transcripts, %d API turns", sessions, turns)
	if o.Since != "" || o.Until != "" {
		fmt.Fprintf(w, " (%s .. %s)", o.Since, o.Until)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "tokens: cache read %s, cache write %s, output %s, input %s\n", human(u.CacheRead), human(u.CacheWrite), human(u.Output), human(u.Input))
	if turns > 0 {
		fmt.Fprintf(w, "average context per turn: %s tokens\n", human((u.CacheRead+u.CacheWrite+u.Input)/int64(turns)))
	}
	fmt.Fprintf(w, "tool output: %s characters\n", human(int64(total)))
	keys := make([]string, 0, len(size))
	for k := range size {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return size[keys[i]] > size[keys[j]] })
	for i, k := range keys {
		if i == o.Top {
			break
		}
		fmt.Fprintf(w, "  %-20s %8s chars %5.1f%%  n=%-6d avg %s\n", k, human(int64(size[k])), 100*float64(size[k])/float64(max(total, 1)), count[k], human(int64(size[k]/max(count[k], 1))))
	}
	return nil
}

func human(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}
