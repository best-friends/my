A single page of text — server information, rules, terms, privacy — that looks the same whether the reader is signed in or not.

**Structure:** a light top bar (wordmark; ログイン when signed out, ホームへ戻る when signed in — the only difference) → a reading column (680px max, 15px / 1.85) with an eyebrow, a `page-title` serif heading and the last-updated date → server facts in a Card with a two-column DescriptionList → `h2` sections (ルール as an ordered list, then the long text as PostContent styles) → footer.

- **Table of contents:** a sticky 200px list beside the column on wide screens, current section in a `soft` pill; on phones it disappears (sections stay linkable by anchor).
- No left rail, no tab bar, no composer: nothing that depends on the session, so the page can be cached and shared as-is.
- Headings form a real outline (one `h1`, then `h2`s); long texts never need horizontal scroll at 320px.
