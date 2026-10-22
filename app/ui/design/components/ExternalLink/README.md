The one link that leaves the app and opens in a new tab. **@my/ui:** `ExternalLink` — `app/ui/src/ExternalLink.tsx`.

**Consumer provides:** `href`, the visible name as children, `rel` tokens to keep (`nofollow`, `ugc`), and `icon={false}` for a link that is a whole card. The app sets `ExternalLinkContext` once (`newTabLabel` 「(新しいタブで開きます)」, `isAppLink`).

- Renders a Link with `target="_blank"`, `rel="noopener noreferrer"` + your tokens, the visually hidden new-tab text, and the 16px `ExternalLinkIcon` (mirrors in right-to-left text).
- Styled as the Link variant you pass (`inline` by default).
