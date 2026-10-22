The card of a link in a post: image, site, title, description and byline, all one link that opens in a new tab. **@my/ui:** `LinkCard` — `app/ui/src/LinkCard.tsx`.

**Consumer provides:** `href`, `title`, `siteName`, `description`, `lang`, `image` (`src`, `width`, `height`, `blurhash`, `alt`), `byline` (「文 · はる · 2026年10月20日」), `sensitive` with `revealLabel` and `defaultRevealed`.

- 16px radius, 1px `border`, `surface-raised`. Image 16:9 on `surface-sunken` above a `border` rule, with its Blurhash while loading or concealed.
- Text block 10×14 padding: site 12px/700 `fg-muted`; title 15px/800 `fg` (underlined on hover / focus) whose link stretches over the whole card; description 14px `fg-muted`, two lines; byline 12px `fg-muted`.
- A sensitive image hides behind the same pill toggle as MediaGrid, above the link's hit area. Page texts are user content: `dir="auto"` and the page's `lang`.
