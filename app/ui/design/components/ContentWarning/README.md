A content warning as a disclosure: the author's warning, a button, and the content hidden behind it. **@my/ui:** `ContentWarning` — `app/ui/src/ContentWarning.tsx`.

**Consumer provides:** `summary` (the author's text, drawn `dir="auto"`), `summaryLang`, `toggleLabel` (「内容を表示」 — the same in both states), the content as children, `isExpanded` / `onExpandedChange` or `defaultExpanded`.

- Brand styling: the summary sits in a `warning-bg` box (14px radius) with a 20px filled WarningIcon in `warning`, the text 15px/700 `fg`, and a small Button on `surface-raised` with a chevron that turns 180° when open.
- The button carries `aria-expanded` and `aria-controls`; the panel is plain `hidden` while closed (not `until-found`), so find-in-page never reveals it.
