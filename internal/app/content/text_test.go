// SPDX-License-Identifier: AGPL-3.0-only

package content_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/app/content"
)

const textHost = "social.example"

// rel is the rel and target every rendered link carries.
const rel = `rel="nofollow noopener noreferrer" target="_blank"`

var textMentions = map[content.MentionHandle]content.MentionTarget{
	{Username: "alice"}:                           {Username: "Alice", URL: "https://social.example/@Alice"},
	{Username: "alice", Domain: "social.example"}: {Username: "Alice", URL: "https://social.example/@Alice"},
	// A target whose fields would break out of the markup if they were not escaped.
	{Username: "evil"}: {Username: `<b>"x"</b>`, URL: `https://social.example/@evil"><script>`},
}

func mention(username, url string) string {
	return `<span class="h-card"><a href="` + url + `" class="u-url mention" ` + rel + `>@<span>` + username + `</span></a></span>`
}

func hashtag(name, path string) string {
	return `<a href="https://social.example/tags/` + path + `" class="mention hashtag" ` + rel + `>#<span>` + name + `</span></a>`
}

func link(href, display string) string {
	return `<a href="` + href + `" ` + rel + `>` + display + `</a>`
}

func TestTextHTML(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"blank", " \n\t\n ", ""},
		{"plain", "hello", "<p>hello</p>"},
		{"line break", "a\nb", "<p>a<br>b</p>"},
		{"CRLF", "a\r\nb\rc", "<p>a<br>b<br>c</p>"},
		{"paragraphs", "a\n\nb\n\n\n\nc", "<p>a</p><p>b</p><p>c</p>"},
		{"surrounding space", "\n\n  a  \n\n", "<p>a</p>"},
		{"unicode", "こんにちは、世界", "<p>こんにちは、世界</p>"},

		// Mentions.
		{"mention", "hi @alice!", "<p>hi " + mention("Alice", "https://social.example/@Alice") + "!</p>"},
		{"mention case", "@ALICE", "<p>" + mention("Alice", "https://social.example/@Alice") + "</p>"},
		{"mention local domain", "@alice@social.example.", "<p>" + mention("Alice", "https://social.example/@Alice") + ".</p>"},
		{"unresolved mention", "@bob hi", "<p>@bob hi</p>"},
		{"remote mention", "@alice@remote.example", "<p>@alice@remote.example</p>"},
		{"e-mail address", "mail alice@social.example", "<p>mail alice@social.example</p>"},
		{"mention after slash", "/@alice", "<p>/@alice</p>"},
		{"bare at", "@ @@", "<p>@ @@</p>"},
		{"mention in Japanese", "こんにちは@alice さん", "<p>こんにちは@alice さん</p>"},
		{"mention after Japanese punctuation", "、@alice", "<p>、" + mention("Alice", "https://social.example/@Alice") + "</p>"},

		// Hashtags.
		{"hashtag", "#Go is fun", "<p>" + hashtag("Go", "go") + " is fun</p>"},
		{"hashtag trailing punctuation", "#go.", "<p>" + hashtag("go", "go") + ".</p>"},
		{"Japanese hashtag", "#日本語 です", "<p>" + hashtag("日本語", "%E6%97%A5%E6%9C%AC%E8%AA%9E") + " です</p>"},
		{"NFKC hashtag", "#ＧＯ", "<p>" + hashtag("ＧＯ", "go") + "</p>"},
		{"half-width katakana hashtag", "#ｶﾀｶﾅ", "<p>" + hashtag("ｶﾀｶﾅ", "%E3%82%AB%E3%82%BF%E3%82%AB%E3%83%8A") + "</p>"},
		{"digits only", "#123", "<p>#123</p>"},
		{"digits and underscore", "#1_2", "<p>#1_2</p>"},
		{"hashtag with digits", "#web3", "<p>" + hashtag("web3", "web3") + "</p>"},
		{"mid-word hash", "C#", "<p>C#</p>"},
		{"character reference", "&#39;", "<p>&amp;#39;</p>"},
		{"double hash", "##go", "<p>##go</p>"},

		// URLs.
		{"URL", "see https://example.com/a", "<p>see " + link("https://example.com/a",
			`<span class="invisible">https://</span><span>example.com/a</span>`) + "</p>"},
		{"URL www", "http://www.example.com", "<p>" + link("http://www.example.com",
			`<span class="invisible">http://www.</span><span>example.com</span>`) + "</p>"},
		{"long URL", "https://example.com/0123456789012345678901234567890123456789", "<p>" + link(
			"https://example.com/0123456789012345678901234567890123456789",
			`<span class="invisible">https://</span><span class="ellipsis">example.com/012345678901234567</span>`+
				`<span class="invisible">8901234567890123456789</span>`) + "</p>"},
		{"URL trailing period", "https://example.com.", "<p>" + link("https://example.com",
			`<span class="invisible">https://</span><span>example.com</span>`) + ".</p>"},
		{"URL in parentheses", "(https://example.com/a_(b))", "<p>(" + link("https://example.com/a_(b)",
			`<span class="invisible">https://</span><span>example.com/a_(b)</span>`) + ")</p>"},
		{"URL query", "https://example.com/?a=1&b=2", "<p>" + link("https://example.com/?a=1&amp;b=2",
			`<span class="invisible">https://</span><span>example.com/?a=1&amp;b=2</span>`) + "</p>"},
		{"URL with fragment and mention", "https://example.com/@alice#top", "<p>" + link("https://example.com/@alice#top",
			`<span class="invisible">https://</span><span>example.com/@alice#top</span>`) + "</p>"},
		{"URL then Japanese", "https://example.comです", "<p>" + link("https://example.com",
			`<span class="invisible">https://</span><span>example.com</span>`) + "です</p>"},
		{"uppercase scheme", "HTTPS://EXAMPLE.COM", "<p>" + link("https://EXAMPLE.COM",
			`<span class="invisible">HTTPS://</span><span>EXAMPLE.COM</span>`) + "</p>"},
		{"no host", "https:// and https:///x", "<p>https:// and https:///x</p>"},
		{"URL with credentials", "https://user:pass@example.com", "<p>https://user:pass@example.com</p>"},
		{"mid-word URL", "xhttps://example.com", "<p>xhttps://example.com</p>"},
		{"ftp", "ftp://example.com", "<p>ftp://example.com</p>"},

		// XSS: markup in the text is always escaped, and nothing but http(s) is ever linked.
		{"script tag", "<script>alert(1)</script>", "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>"},
		{"attribute breakout", `"><img src=x onerror=alert(1)>`, "<p>&#34;&gt;&lt;img src=x onerror=alert(1)&gt;</p>"},
		{"javascript scheme", "javascript:alert(1)", "<p>javascript:alert(1)</p>"},
		{"data scheme", "data:text/html,<script>alert(1)</script>", "<p>data:text/html,&lt;script&gt;alert(1)&lt;/script&gt;</p>"},
		{"quote in URL", `https://example.com/"onmouseover="alert(1)`, "<p>" + link("https://example.com/",
			`<span class="invisible">https://</span><span>example.com/</span>`) + "&#34;onmouseover=&#34;alert(1)</p>"},
		{"apostrophe in URL", `https://example.com/a'onclick='x'`, "<p>" + link("https://example.com/a&#39;onclick=&#39;x",
			`<span class="invisible">https://</span><span>example.com/a&#39;onclick=&#39;x</span>`) + "&#39;</p>"},
		{"tag in URL", "https://example.com/<script>", "<p>" + link("https://example.com/",
			`<span class="invisible">https://</span><span>example.com/</span>`) + "&lt;script&gt;</p>"},
		{"javascript in path", "https://example.com/javascript:alert(1)", "<p>" + link("https://example.com/javascript:alert(1)",
			`<span class="invisible">https://</span><span class="ellipsis">example.com/javascript:alert(1</span><span class="invisible">)</span>`) + "</p>"},
		{"markup in hashtag", "#<b>x</b>", "<p>#&lt;b&gt;x&lt;/b&gt;</p>"},
		{"markup after hashtag", "#go<script>", "<p>" + hashtag("go", "go") + "&lt;script&gt;</p>"},
		{"markup in mention", "@<b>alice</b>", "<p>@&lt;b&gt;alice&lt;/b&gt;</p>"},
		{"escaped mention target", "@evil", "<p>" + mention("&lt;b&gt;&#34;x&#34;&lt;/b&gt;",
			"https://social.example/@evil&#34;&gt;&lt;script&gt;") + "</p>"},
		{"entity", "&amp; &lt;", "<p>&amp;amp; &amp;lt;</p>"},
		{"tag-like line breaks", "<\n>", "<p>&lt;<br>&gt;</p>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := content.ParseText(tc.in).HTML(content.RenderOptions{Host: textHost, Mentions: textMentions})
			if got != tc.want {
				t.Errorf("HTML(%q)\n got %s\nwant %s", tc.in, got, tc.want)
			}
			checkAllowlisted(t, got)
		})
	}
}

func TestParseTextMentionsAndHashtags(t *testing.T) {
	p := content.ParseText("@Alice @alice@Social.Example @bob@remote.example @alice #Go #go #ＧＯ #日本語 alice@example.com #123")
	wantMentions := []content.MentionHandle{
		{Username: "alice"}, {Username: "alice", Domain: "social.example"}, {Username: "bob", Domain: "remote.example"},
	}
	if got := p.Mentions(); !reflect.DeepEqual(got, wantMentions) {
		t.Errorf("Mentions = %+v, want %+v", got, wantMentions)
	}
	wantTags := []content.Hashtag{{Name: "Go", NameNormalized: "go"}, {Name: "日本語", NameNormalized: "日本語"}}
	if got := p.Hashtags(); !reflect.DeepEqual(got, wantTags) {
		t.Errorf("Hashtags = %+v, want %+v", got, wantTags)
	}
}

func TestNormalizeTag(t *testing.T) {
	for in, want := range map[string]string{
		"Go":    "go",
		"ＧＯ":    "go",
		"ｶﾀｶﾅ":  "カタカナ",
		"Ⅻ":     "xii",
		"ÉCOLE": "école",
		"é":    "é",
		"日本語":   "日本語",
	} {
		if got := content.NormalizeTag(in); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}

// FuzzTextHTML checks that whatever the text, the HTML uses only the allowlisted markup.
func FuzzTextHTML(f *testing.F) {
	for _, s := range []string{
		"<script>alert(1)</script>", `"><img src=x onerror=alert(1)>`, "https://example.com/?a=<b>&c='d'",
		"@alice #go https://example.com", "#<a href=x>", "@evil", "\n\n\r\n<", "javascript:alert(1)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkAllowlisted(t, content.ParseText(s).HTML(content.RenderOptions{Host: textHost, Mentions: textMentions}))
	})
}

var (
	tagPattern  = regexp.MustCompile(`^<(/?)([a-z]+)((?: [a-z]+="[^"<>]*")*)>`)
	attrPattern = regexp.MustCompile(` ([a-z]+)="([^"<>]*)"`)
	// allowedAttrs are the attributes each element may carry (ADR-0019 §2).
	allowedAttrs = map[string][]string{"p": nil, "br": nil, "span": {"class"}, "a": {"href", "class", "rel", "target"}}
	allowedClass = map[string]bool{"h-card": true, "u-url mention": true, "mention hashtag": true, "invisible": true, "ellipsis": true}
)

// checkAllowlisted fails unless every tag of h is an allowlisted element with allowlisted
// attributes, links point at http(s), and no "<" or ">" appears outside a tag.
func checkAllowlisted(t *testing.T, h string) {
	t.Helper()
	for i := 0; i < len(h); {
		switch h[i] {
		case '>':
			t.Fatalf("stray > at %d in %s", i, h)
		case '<':
			m := tagPattern.FindStringSubmatch(h[i:])
			if m == nil {
				t.Fatalf("stray < at %d in %s", i, h)
			}
			allowed, ok := allowedAttrs[m[2]]
			if !ok {
				t.Fatalf("element <%s> is not allowlisted in %s", m[2], h)
			}
			for _, a := range attrPattern.FindAllStringSubmatch(m[3], -1) {
				name, value := a[1], a[2]
				if !contains(allowed, name) {
					t.Fatalf("attribute %s of <%s> is not allowlisted in %s", name, m[2], h)
				}
				switch name {
				case "class":
					if !allowedClass[value] {
						t.Fatalf("class %q is not allowlisted in %s", value, h)
					}
				case "href":
					if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
						t.Fatalf("href %q is not http(s) in %s", value, h)
					}
				case "rel":
					if value != "nofollow noopener noreferrer" {
						t.Fatalf("rel %q in %s", value, h)
					}
				case "target":
					if value != "_blank" {
						t.Fatalf("target %q in %s", value, h)
					}
				}
			}
			i += len(m[0])
		default:
			i++
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
