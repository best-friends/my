The reason an operator gives for a moderation action: a three-row TextArea whose server limit is always counted. **@my/ui:** `ReasonField` — `app/ui/src/ReasonField.tsx`.

**Consumer provides:** `label`, `characterLimit` (5,000 for most), `formatRemaining`, and `isRequired` when the action needs a reason. Everything else is TextArea's.

- Used as the first field inside a ConfirmDialog, which focuses it on open.
