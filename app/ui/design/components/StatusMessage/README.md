The result of an action that stays on the page (「example.com を 10月24日まで停止しました」). **@my/ui:** `StatusMessage` — `app/ui/src/StatusMessage.tsx`.

**Consumer provides:** render it before the action, empty; pass the result as children once the action succeeded. `tone`: `success` (default) or `neutral`.

- A polite `role="status"` region that is in the page from the first render, so the message is announced.
- `success`: `success-bg` box with a `success` check icon; `neutral`: `surface-sunken` box with an AboutIcon in `fg-muted`. Text 15px `fg`, wrapping anywhere. 14px radius.
- A failure is not a status: it stays in the dialog that ran it, or goes to a Toast.
