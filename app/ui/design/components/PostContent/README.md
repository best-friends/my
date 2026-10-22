The body of a post: server-sanitized HTML in a block whose direction follows its own text. **@my/ui:** `PostContent` — `app/ui/src/PostContent.tsx`.

**Consumer provides:** `html` (a `SanitizedHtml` from `serverSanitizedHtml()` — never client-built HTML), `lang`, `emojis` (the post's custom emoji), `onLinkClick` to open app links through the router, and an `id` to describe a PostCard.

- Text 15px `fg`, line-height 1.85 (the brand's `post-body`), wrapping anywhere so nothing scrolls sideways at 320px; blocks 8px apart.
- Links `accent`, underlined (3px offset), `accent-hover` on hover; links that leave the app open in a new tab with the hidden new-tab text.
- Blockquote: a `surface-sunken` block (12px radius) in `fg-muted` — the brand's soft quote instead of a start-edge rule. Code: monospace on `surface-sunken` pills; `pre` scrolls inside.
- Lists keep their markers (24px inset). `:shortcode:` text becomes CustomEmoji, except inside code.
