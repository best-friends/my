A modal dialog centred over a scrim. **@my/ui:** `Dialog` (with `DialogTrigger`) — `app/ui/src/Dialog.tsx`.

**Consumer provides:** `title` (names the dialog), content as children (or a function receiving `close`), `closeLabel` (「閉じる」 — required; only an alert dialog passes `null`), `size` (`sm` 384 · `md` 448 default · `lg` 672px), `isDismissable` (off for a form a stray tap would lose), `isOpen` / `onOpenChange` or a `DialogTrigger`.

- Panel: `surface-raised`, `radius-card` (19px), `elevation-lg`, max 85svh; the body scrolls with `overscroll-behavior: contain`.
- Header: title 20px/800 (-0.4px), a 36px round × in `fg-muted` (`surface-sunken` on hover). Body padding 20px.
- Scrim `overlay`. Enters with a 95% scale and fade over 200ms under motion-safe.
- Traps focus, closes on Esc and the ×, returns focus to the trigger.
