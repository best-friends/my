A short, visible note about something that just happened, stacked at the bottom edge. **@my/ui:** `Toast` (`ToastProvider`, `useToast`) — `app/ui/src/Toast.tsx`.

**Consumer provides:** mount `ToastProvider` once inside the AnnouncerProvider (`regionLabel` 「お知らせ」, `dismissLabel` 「閉じる」, `defaultTimeout` 5000ms); call `show({ message, tone, action, timeout })`.

- Brand styling: a dark pill (`toast-bg`, 18px radius, `elevation-lg`) with `toast-fg` text 14px, max 448px. In Dark it gets a 1px `border` to part from the page.
- `tone` shows by icon, not a coloured bar: neutral AboutIcon (`toast-muted`), success check (`toast-success`), danger WarningIcon (`toast-danger`).
- One `action` (「元に戻す」) in `toast-action`, 800; a 32px × dismiss in `toast-muted`.
- Never takes focus; announced through the Announcer (danger assertively). Auto-dismiss pauses while hovered or focused; a toast with an action stays until dismissed.
