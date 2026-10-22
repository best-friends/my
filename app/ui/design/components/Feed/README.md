A timeline as a WAI-ARIA feed that scrolls with the page, with only the articles near the viewport mounted. **@my/ui:** `Feed` (`Feed`, `FeedArticle`) — `app/ui/src/Feed.tsx`.

**Consumer provides:** `items`, `getKey`, `renderItem` (one FeedArticle — PostCard is one), `aria-labelledby` (a visible heading), `setSize`, `isLoading`, `hasMore`, `onLoadMore`, `loadMoreLabel` (「さらに読み込む」), `loadingLabel` (「投稿を読み込み中」).

- `role="feed"` with one focusable article per item (`aria-posinset` / `aria-setsize`), `aria-busy` while loading.
- Keys: Page Down / Page Up move between articles; Ctrl+End / Ctrl+Home leave the feed. Document them where the feed is used.
- The next page loads as the end nears; a centred secondary Button 「さらに読み込む」 (with a Spinner while pending) is the fallback.
- New items never push in while the reader is inside; show a NewItemsButton above instead.
- Brand: the articles are PostCards stacked 12px apart; focus shows the 3px `focus` ring around the card.
