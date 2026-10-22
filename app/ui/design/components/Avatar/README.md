A rounded-square profile picture that falls back to the first letter of the name. **@my/ui:** `Avatar` — `app/ui/src/Avatar.tsx`.

**Consumer provides:** `name` (the accessible name and the fallback letter), `src` or `sources`, `size`, and `decorative` — `true` when the name is shown beside it (a post header), `false` for a standalone avatar.

| `size` | xs | sm | md (default) | lg | xl |
| --- | --- | --- | --- | --- | --- |
| px | 24 | 32 | 40 | 48 | 80 |

- Shape: `radius-avatar` (21%, the App-Icon's corner) at every size — never a circle.
- Image: cover-fitted. Missing or failed image: the first grapheme of the name, upper-cased, in Georgia bold on one of the five pastel tints (`avatar-*-bg` / `avatar-*-fg`; pick the tint per account, stable by id). Blank name: a silhouette in `fg-muted` on `surface-sunken`.
- Initials reach 4.5:1 on every tint (lilac 4.62, peach 4.51, mint 4.51, lavender 4.56, yellow 4.53), so they stay readable even at `xs`.
