A two-state icon button for post actions (favourite, boost, bookmark) that exposes `aria-pressed`. **@my/ui:** `ToggleButton` — `app/ui/src/ToggleButton.tsx`.

**Consumer provides:** an icon (with `filled` while selected) and optional count as children, `aria-label` with a name that stays the same in both states (「お気に入り」, not 「お気に入り解除」), `isSelected` / `onChange`, and `tone`.

- Rest: `fg-muted` icon and count (13px/700), `surface-sunken` on hover, 10px radius, 32px min (44px coarse).
- Selected: `tone` colours the icon — `accent` (favourite, bookmark), `success` (boost), `warning` (content warning in the composer) — **and the icon changes shape** (`StarIcon filled`, `BoostIcon filled` = bold), so state never rests on colour alone.
- Describe it by the count with `aria-describedby` when the count matters.
