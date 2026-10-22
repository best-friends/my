// SPDX-License-Identifier: AGPL-3.0-only

package ops

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/best-friends/my/tools/agentkit/internal/gh"
	"github.com/best-friends/my/tools/agentkit/internal/repo"
)

// OpenOptions are the parts of a pull request the agent writes; the rest comes from the issue and the commits.
type OpenOptions struct {
	Issue     int
	Base      string
	Title     string
	Summary   string
	Decisions string
	Footer    string // e.g. the attribution line the agent's harness asks for
	Confirm   bool   // the agent confirms the docs and no-secrets items of the checklist
	Draft     bool
	NoPush    bool
}

var reCheckbox = regexp.MustCompile(`^\s*- \[[ xX]\]\s+(.*)$`)

// acceptanceCriteria returns the checklist under the issue's "Acceptance criteria" heading.
func acceptanceCriteria(body string) []string {
	var out []string
	in := false
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "#") {
			in = strings.Contains(strings.ToLower(l), "acceptance criteria") || strings.Contains(l, "受け入れ基準")
			continue
		}
		if m := reCheckbox.FindStringSubmatch(l); in && m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// Open pushes the branch and opens its pull request from .github/pull_request_template.md's sections: the
// summary, Closes #N (or Stacked on #M first), the commit subjects as the changes, the issue's acceptance
// criteria checked, the decisions, and the checklist (make check checked when `agentkit check` passed on
// this content of the tree). It comments the PR on the issue and returns the PR URL.
func Open(r *repo.Repo, o OpenOptions) (string, error) {
	branch, err := r.Branch()
	if err != nil {
		return "", err
	}
	if branch == "main" || branch == "HEAD" {
		return "", fmt.Errorf("on %s: work on a <type>/<issue>-<slug> branch (workflow.md §3)", branch)
	}
	if o.Issue == 0 {
		o.Issue, _ = strconv.Atoi(gh.BranchIssue(branch))
	}
	if o.Base == "" {
		o.Base = "main"
	}
	var is *gh.Issue
	if o.Issue > 0 {
		if is, err = gh.GetIssue(o.Issue); err != nil {
			return "", err
		}
	}
	if !o.NoPush {
		if out, err := exec.Command("git", "-C", r.Root, "push", "-u", "origin", "HEAD").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git push: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}
	logOut, err := repo.Git(r.Root, "log", "--reverse", "--no-merges", "--format=%s", "origin/"+o.Base+"..HEAD")
	if err != nil {
		return "", err
	}
	subjects := strings.Split(strings.TrimSpace(logOut), "\n")
	if len(subjects) == 0 || subjects[0] == "" {
		return "", fmt.Errorf("no commits on %s since origin/%s", branch, o.Base)
	}
	if o.Title == "" {
		o.Title = subjects[0]
	}

	var b strings.Builder
	if o.Base != "main" {
		var prs []struct {
			Number int `json:"number"`
		}
		if err := gh.JSON(&prs, "pr", "list", "-R", gh.Repo(), "--head", o.Base, "--json", "number"); err == nil && len(prs) > 0 {
			fmt.Fprintf(&b, "Stacked on #%d\n\n", prs[0].Number)
		}
	}
	summary := strings.TrimSpace(o.Summary)
	if summary == "" && is != nil {
		summary = is.Title
	}
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", summary)
	if o.Issue > 0 {
		fmt.Fprintf(&b, "Closes #%d\n\n", o.Issue)
	}
	b.WriteString("## Changes\n\n")
	for _, s := range subjects {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	b.WriteString("\n## Acceptance criteria\n\n")
	var ac []string
	if is != nil {
		ac = acceptanceCriteria(is.Body)
	}
	if len(ac) == 0 {
		b.WriteString("- [ ] (the issue has no checklist)\n")
	}
	for _, a := range ac {
		fmt.Fprintf(&b, "- [x] %s\n", a)
	}
	decisions := strings.TrimSpace(o.Decisions)
	if decisions == "" {
		decisions = "None"
	}
	fmt.Fprintf(&b, "\n## Decisions and assumptions\n\n%s\n\n## Checklist\n\n", decisions)
	fmt.Fprintf(&b, "- [%s] `make check` passes locally\n", mark(CheckPassed(r)))
	fmt.Fprintf(&b, "- [%s] Docs / ADR index / i18n catalogs updated where needed\n", mark(o.Confirm))
	fmt.Fprintf(&b, "- [%s] No secrets, no unrelated changes\n", mark(o.Confirm))
	if o.Footer != "" {
		fmt.Fprintf(&b, "\n%s\n", o.Footer)
	}

	args := []string{"pr", "create", "-R", gh.Repo(), "--base", o.Base, "--head", branch, "--title", o.Title, "--body-file", "-"}
	if o.Draft {
		args = append(args, "--draft")
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = r.Root
	cmd.Stdin = strings.NewReader(b.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w: %s", err, strings.TrimSpace(string(out)))
	}
	url := strings.TrimSpace(string(out))
	if i := strings.LastIndex(url, "\n"); i >= 0 {
		url = url[i+1:]
	}
	if o.Issue > 0 {
		if _, err := gh.Run("issue", "comment", strconv.Itoa(o.Issue), "-R", gh.Repo(), "--body", "PR: "+url); err != nil {
			return url, err
		}
	}
	note := ""
	if !CheckPassed(r) {
		note = " (make check is unchecked: run `agentkit check` and tick it)"
	}
	return url + note, nil
}

func mark(b bool) string {
	if b {
		return "x"
	}
	return " "
}

// check is one entry of a pull request's statusCheckRollup.
type check struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
	DetailsURL string `json:"detailsUrl"`
	TargetURL  string `json:"targetUrl"`
}

func (c check) name() string {
	if c.Name != "" {
		return c.Name
	}
	return c.Context
}

// result is pending, failed, or passed.
func (c check) result() string {
	if c.Typename == "StatusContext" {
		switch c.State {
		case "SUCCESS":
			return "passed"
		case "FAILURE", "ERROR":
			return "failed"
		}
		return "pending"
	}
	if c.Status != "COMPLETED" {
		return "pending"
	}
	switch c.Conclusion {
	case "FAILURE", "TIMED_OUT", "STARTUP_FAILURE", "ACTION_REQUIRED", "CANCELLED":
		return "failed"
	}
	return "passed"
}

type prState struct {
	Number            int     `json:"number"`
	Title             string  `json:"title"`
	State             string  `json:"state"`
	Mergeable         string  `json:"mergeable"`
	URL               string  `json:"url"`
	HeadRefOid        string  `json:"headRefOid"`
	StatusCheckRollup []check `json:"statusCheckRollup"`
	AutoMergeRequest  *struct {
		EnabledAt string `json:"enabledAt"`
	} `json:"autoMergeRequest"`
}

const prFields = "number,title,state,mergeable,url,headRefOid,statusCheckRollup,autoMergeRequest"

func viewPR(n int) (prState, error) {
	var p prState
	err := gh.JSON(&p, "pr", "view", strconv.Itoa(n), "-R", gh.Repo(), "--json", prFields)
	return p, err
}

// summarize returns the CI result of a pull request (none, failed, pending, passed), the names of the failed
// checks, and whether any check is still running.
func summarize(cs []check) (string, []string, bool) {
	if len(cs) == 0 {
		return "none", nil, false
	}
	var failed []string
	pending := false
	for _, c := range cs {
		switch c.result() {
		case "failed":
			failed = append(failed, c.name())
		case "pending":
			pending = true
		}
	}
	sort.Strings(failed)
	switch {
	case len(failed) > 0:
		return "failed", failed, pending
	case pending:
		return "pending", nil, true
	}
	return "passed", nil, false
}

// WaitResult is how Wait ended: the exit status and the lines to print.
type WaitResult struct {
	Code  int
	Lines []string
}

var reJob = regexp.MustCompile(`/actions/runs/(\d+)/job/(\d+)`)

// Wait polls a pull request until its CI ends, it conflicts, it merges, or the timeout passes. With all false
// it returns at the first failed check instead of waiting for the others. A failure comes with the failing
// lines of the failed jobs' logs, which are saved under logDir.
func Wait(n int, timeout, interval time.Duration, all bool, logDir string) WaitResult {
	deadline := time.Now().Add(timeout)
	started := time.Now()
	for {
		p, err := viewPR(n)
		switch {
		case err != nil:
			// gh and the API fail now and then; give up only at the deadline.
		case p.State == "MERGED":
			return WaitResult{0, []string{fmt.Sprintf("PR #%d is merged", n)}}
		case p.State == "CLOSED":
			return WaitResult{1, []string{fmt.Sprintf("PR #%d is closed", n)}}
		case p.Mergeable == "CONFLICTING":
			return WaitResult{3, []string{fmt.Sprintf("PR #%d conflicts with its base: CI does not run. Merge origin/main, regenerate generated files (agentkit regen), agentkit check, push (workflow.md §7)", n)}}
		default:
			res, failed, pending := summarize(p.StatusCheckRollup)
			if res == "failed" && (!all || !pending) {
				return WaitResult{1, failureReport(n, p, failed, logDir)}
			}
			if res == "passed" {
				return WaitResult{0, []string{fmt.Sprintf("PR #%d: CI passed (%d checks, mergeable %s)", n, len(p.StatusCheckRollup), strings.ToLower(p.Mergeable))}}
			}
			if res == "none" && time.Since(started) > 10*time.Minute {
				return WaitResult{2, []string{fmt.Sprintf("PR #%d: no check started in 10 minutes (mergeable %s); check the PR page %s", n, strings.ToLower(p.Mergeable), p.URL)}}
			}
		}
		if time.Now().After(deadline) {
			return WaitResult{124, []string{fmt.Sprintf("PR #%d: CI still running after %s", n, timeout)}}
		}
		time.Sleep(interval)
	}
}

func failureReport(n int, p prState, failed []string, logDir string) []string {
	lines := []string{fmt.Sprintf("PR #%d: CI failed: %s", n, strings.Join(failed, ", "))}
	shown := 0
	for _, c := range p.StatusCheckRollup {
		// `check` only collects the other jobs' results; its log says nothing new.
		if c.result() != "failed" || c.name() == "check" || shown == 3 {
			continue
		}
		m := reJob.FindStringSubmatch(c.DetailsURL + c.TargetURL)
		if m == nil {
			lines = append(lines, fmt.Sprintf("== %s: %s", c.name(), c.DetailsURL+c.TargetURL))
			continue
		}
		shown++
		out, err := gh.Run("run", "view", m[1], "--job", m[2], "--log-failed", "-R", gh.Repo())
		if err != nil {
			lines = append(lines, fmt.Sprintf("== %s: could not read the log: %v", c.name(), err))
			continue
		}
		file := filepath.Join(logDir, fmt.Sprintf("pr-%d-%s.log", n, m[2]))
		_ = os.WriteFile(file, out, 0o600)
		lines = append(lines, fmt.Sprintf("== %s (full log: %s)", c.name(), file))
		lines = append(lines, Failures(string(out), 40)...)
	}
	return lines
}

// Watch prints one line per event of the watched pull requests — opened, merged, closed, conflicting,
// ci-failed, ci-green, dequeued (auto-merge turned off while open) — until every watched pull request is
// merged or closed. With no numbers it watches every open pull request, including new ones, forever.
func Watch(w io.Writer, nums []int, interval time.Duration) error {
	all := len(nums) == 0
	watched := map[int]bool{}
	for _, n := range nums {
		watched[n] = true
	}
	type last struct {
		ci, mergeable string
		auto          bool
	}
	prev := map[int]last{}
	first := true
	emit := func(n int, title, event string) {
		fmt.Fprintf(w, "%s #%d %s — %s\n", time.Now().Format("15:04"), n, event, title)
		if f, ok := w.(*os.File); ok {
			_ = f.Sync()
		}
	}
	for {
		var open []prState
		if err := gh.JSON(&open, "pr", "list", "-R", gh.Repo(), "--state", "open", "--limit", "100", "--json", prFields); err != nil {
			time.Sleep(interval)
			continue
		}
		seen := map[int]bool{}
		for _, p := range open {
			seen[p.Number] = true
			if all && !watched[p.Number] {
				watched[p.Number] = true
				if !first {
					emit(p.Number, p.Title, "opened")
				}
			}
			if !watched[p.Number] {
				continue
			}
			ci, failed, _ := summarize(p.StatusCheckRollup)
			cur := last{ci, p.Mergeable, p.AutoMergeRequest != nil}
			old, had := prev[p.Number]
			switch {
			case cur.mergeable == "CONFLICTING" && (!had || old.mergeable != "CONFLICTING"):
				emit(p.Number, p.Title, "conflicting")
			case cur.ci == "failed" && (!had || old.ci != "failed"):
				emit(p.Number, p.Title, "ci-failed "+strings.Join(failed, ","))
			case cur.ci == "passed" && had && old.ci != "passed":
				emit(p.Number, p.Title, "ci-green")
			}
			if had && old.auto && !cur.auto {
				emit(p.Number, p.Title, "dequeued (auto-merge is off: conflict or failed merge group)")
			}
			prev[p.Number] = cur
		}
		for n := range watched {
			if seen[n] {
				continue
			}
			p, err := viewPR(n)
			if err != nil {
				continue
			}
			switch p.State {
			case "MERGED":
				emit(n, p.Title, "merged")
				delete(watched, n)
			case "CLOSED":
				emit(n, p.Title, "closed")
				delete(watched, n)
			}
		}
		first = false
		if !all && len(watched) == 0 {
			return nil
		}
		time.Sleep(interval)
	}
}
