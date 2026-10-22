A text link in three variants. **@my/ui:** `Link` — `app/ui/src/Link.tsx`.

**Consumer provides:** `href` (client navigation comes from the app's React Aria `RouterProvider`), children, `variant`.

- `inline` (default) — inside running text: `accent`, always underlined (3px offset), 2px underline on hover, so colour is not the only cue.
- `standalone` — on its own (「もっと見る」): 700 weight, no underline until hover, 24px min height (44px coarse), 16px icon.
- `navigation` — an item of the app navigation: `fg` text, 24px icon, 12px gap. The current item sits in a `soft` pill (see RailItem).
- Hover: `accent-hover`. Disabled: opacity .5. `excludeFromTabOrder` for a duplicate link (an avatar next to the name) — hide it with `aria-hidden` too.
