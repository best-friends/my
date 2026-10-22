A multi-line text field with an optional live character count. **@my/ui:** `TextArea` — `app/ui/src/TextArea.tsx`.

**Consumer provides:** everything TextField takes, plus `rows`, and `characterLimit` with `formatRemaining(remaining)` (「残り 12 文字」 / 「3 文字オーバー」).

- Same control box as TextField, 96px min height, resizes vertically.
- Count: 13px `fg-muted` under the control; over the limit it turns `danger`, 700, and the field becomes invalid (text is never cut).
- The visible count is linked with `aria-describedby`; a separate `role="status"` announces it after a 1s pause, or at once at 50, 20 and 0 left.
