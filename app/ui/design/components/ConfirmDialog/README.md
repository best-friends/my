The confirmation before an action that cannot be undone: an alert dialog whose title asks and whose buttons are verbs. **@my/ui:** `ConfirmDialog` — `app/ui/src/ConfirmDialog.tsx`.

**Consumer provides:** `title` as a question (「この投稿を削除しますか?」), `description` (what happens, whether it can be undone), optional fields as children (ReasonField, DurationSelect — the first gets focus), `confirmLabel` (「削除する」), `cancelLabel` (「キャンセル」), `tone` (`danger` default / `primary`), `onConfirm(close)`, `isPending`, `errorMessage`.

- A Dialog of size `sm` with `role="alertdialog"` and no ×: cancel, Esc and a press outside close it (not outside when it has fields).
- Focus opens on the first field, else on cancel — never on the destructive button.
- Actions end-aligned: cancel as a secondary Button, confirm as `danger` (or `primary`). Pending: the confirm button waits with a Spinner. A failure shows above the buttons in 13px `danger` (`role="alert"`).
- Brand voice: offer the softer path in the description (「まずは下書きとして保存することもできます。」).
