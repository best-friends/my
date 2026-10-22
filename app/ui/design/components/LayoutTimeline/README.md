The timeline page, built for a feed that keeps receiving new posts: nothing is fixed over the posts, and nothing sits between the reader and the newest post.

**What is left out, on purpose:**

- **No header bar.** The page starts with the timeline tabs (ホーム / ローカル / 連合), which scroll away with the posts; the page's `h1` 「ホーム」 is visually hidden and names the Feed.
- **No inline composer.** Writing starts from the rail's 「投稿する」 button (PC), the round coral button in the tab bar (phone), or the `n` key — each opens the composer in a Dialog (PC) or Sheet (phone), so the feed's top is always a post.

**Layout:**

- **PC (≥1120px):** three columns — the left rail (260px: wordmark, RailItems, 「投稿する」, the account at the bottom), the Feed (≤640px), the right rail (300px: search, trends, footer links). Both rails are `position: sticky` beside the feed, never on top of it.
- **Tablet (700–1119px):** the right rail goes; search moves into 「見つける」.
- **Phone (<700px):** one column, 12px gutters; a 64px bottom tab bar (ホーム, 見つける, the compose button, 通知, メニュー) is the only fixed element, at the edge far from where new posts arrive.

**New posts while reading:**

- New posts are inserted at the top automatically. If the reader is at the very top, they simply appear; if the reader has scrolled, the inserted height is compensated (scroll anchoring), so the post being read does not move.
- A small NewItemsButton pill (「12件の新しい投稿」) floats over the top of the feed column only while the reader is away from the top; pressing it scrolls back up. It is the only element that may overlay the feed, and it never takes focus.
- Inserted posts appear without animation. Keep a pause control in settings (「タイムラインを自動更新」) — WCAG 2.2.2 asks that content that updates by itself can be paused.
- Note: @my/ui's Feed currently holds new items until the reader asks (ADR-0027). This layout's auto-insert with anchoring changes that behaviour; decide it in the ADR before building.
