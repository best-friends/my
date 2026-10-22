The app's single place for screen-reader announcements: one polite and one assertive live region, rendered once near the root. **@my/ui:** `Announcer` (`AnnouncerProvider`, `useAnnouncer`) — `app/ui/src/Announcer.tsx`.

**Consumer provides:** mount `AnnouncerProvider` once at the app root (`clearAfter`, 7000ms by default); call `announce(message, "polite" | "assertive")` from event handlers.

- Nothing is visible: both regions are `sr-only` and present from the first render. Announcing never moves focus.
- `polite` (`role="status"`) for counts and completions; `assertive` (`role="alert"`) only for errors.
- A new node per message, removed after `clearAfter`, so a repeated message is read again and stale ones never linger.
- The preview shows what the two regions would read; on screen there is nothing.
