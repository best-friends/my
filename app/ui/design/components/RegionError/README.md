What a region shows instead of its content when loading it failed: what happened, why, and 「もう一度試す」. **@my/ui:** `RegionError` — `app/ui/src/RegionError.tsx`.

**Consumer provides:** `heading` (「投稿を読み込めませんでした」), `headingLevel` (omit for a paragraph, as for a failed next page), `reason`, `retryLabel` + `onRetry` + `isRetrying`, and `alert` only when it is the result of the reader's own retry.

- Replaces only that region, so the page keeps its h1 and a list keeps what it already loaded.
- Brand styling: a `danger-bg` box, 14px radius, a 22px filled WarningIcon in `danger`, heading 15px/800 `fg`, reason 13px `fg`; the retry is a small Button on `surface-raised` (pending shows a Spinner).
- @my/ui draws a start-edge bar here; the brand uses the tinted box and icon instead.
