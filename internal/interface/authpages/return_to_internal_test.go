// SPDX-License-Identifier: AGPL-3.0-only

package authpages

import (
	"net/url"
	"strings"
	"testing"
)

// TestSafeReturnTo runs safeReturnTo on paths of this site, which it keeps (normalized), and on
// the known shapes of open redirects (OWASP Unvalidated Redirects and Forwards Cheat Sheet,
// WHATWG URL parsing differences), which it refuses.
func TestSafeReturnTo(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"root", "/", "/"},
		{"path", "/home", "/home"},
		{"path and query", "/oauth/authorize?client_id=a&redirect_uri=https%3A%2F%2Fweb.example%2Fcb", "/oauth/authorize?client_id=a&redirect_uri=https%3A%2F%2Fweb.example%2Fcb"},
		{"fragment dropped", "/home#top", "/home"},
		{"dot segments resolved", "/a/./b/../c", "/a/c"},
		{"dot segments above the root", "/../../home", "/home"},
		{"encoded slash inside the path kept encoded", "/tags/a%2Fb", "/tags/a%2Fb"},
		{"non-ASCII path", "/tags/日本語", "/tags/%E6%97%A5%E6%9C%AC%E8%AA%9E"},
		{"query with a URL", "/x?next=//evil.example", "/x?next=//evil.example"},

		{"empty", "", ""},
		{"relative path", "home", ""},
		{"absolute URL", "https://evil.example/", ""},
		{"absolute URL of this placeholder", "https://return-to.invalid/home", ""},
		{"scheme without slashes", "https:evil.example", ""},
		{"javascript", "javascript:alert(1)", ""},
		{"data", "data:text/html,<script>alert(1)</script>", ""},
		{"protocol-relative", "//evil.example", ""},
		{"protocol-relative with path", "//evil.example/home", ""},
		{"three slashes", "///evil.example", ""},
		{"backslash", "/\\evil.example", ""},
		{"backslashes", "\\\\evil.example", ""},
		{"slash and backslash", "/\\/evil.example", ""},
		{"encoded slashes", "/%2F%2Fevil.example", ""},
		{"encoded slash after a slash", "/%2Fevil.example", ""},
		{"lower-case encoded slashes", "/%2f%2fevil.example", ""},
		{"encoded backslash", "/%5Cevil.example", ""},
		{"encoded backslash after a slash", "/%5C/evil.example", ""},
		{"tab", "/\t/evil.example", ""},
		{"newline", "/\n/evil.example", ""},
		{"carriage return", "/\r/evil.example", ""},
		{"header injection", "/x\r\nSet-Cookie: a=b", ""},
		{"NUL", "/x\x00", ""},
		{"encoded tab", "/%09/evil.example", ""},
		{"encoded newline", "/%0A/evil.example", ""},
		{"encoded NUL", "/x%00", ""},
		{"DEL", "/x\x7f", ""},
		{"C1 control", "/x\u0085", ""},
		{"dot segment to protocol-relative", "/..//evil.example", ""},
		{"dot segments to protocol-relative", "/a/../..//evil.example", ""},
		{"encoded dot segment to protocol-relative", "/%2E%2E//evil.example", ""},
		{"encoded dot segment", "/a/%2e%2e/home", ""},
		{"encoded single dot", "/%2E/home", ""},
		{"empty segment", "/a//b", ""},
		{"userinfo", "//user@evil.example", ""},
		{"userinfo after scheme", "https://return-to.invalid@evil.example/", ""},
		{"IDN host", "//ｅｖｉｌ.example", ""},
		{"punycode host", "//xn--80ak6aa92e.example", ""},
		{"invalid escape", "/%zz", ""},
		{"too long", "/" + strings.Repeat("a", maxReturnTo), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := safeReturnTo(tc.in)
			if got != tc.want {
				t.Fatalf("safeReturnTo(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if got == "" {
				return
			}
			u, err := url.Parse(got)
			if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(got, "/") || strings.HasPrefix(got, "//") {
				t.Fatalf("safeReturnTo(%q) = %q, which is not a path on this site", tc.in, got)
			}
		})
	}
}
