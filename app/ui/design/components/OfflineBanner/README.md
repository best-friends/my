The app's one offline bar, stuck to the top edge. **@my/ui:** `OfflineBanner` — `app/ui/src/OfflineBanner.tsx`.

**Consumer provides:** `isOffline` and the message as children (「オフラインです。送った投稿は、つながったときに送信されます。」). Render it once at the top of the app for the whole session.

- A polite `role="status"` region that is always in the page, empty while online, so going offline is announced.
- Brand styling: a `warning-bg` bar with `fg` text, 13px/700, centred, with a 18px WarningIcon in `warning`; pads itself for the top safe-area inset.
- @my/ui currently paints a solid `warning` bar; switch it to `bg-warning-bg text-fg` for this look.
