A small count, such as unread notifications. **@my/ui:** `Badge` — `app/ui/src/Badge.tsx`.

**Consumer provides:** `count`, `label` (what the number means, read instead of the digits: 「未読の通知 12 件」), `max` (99 by default — above it the badge shows 「99+」), `tone`.

- 20px pill, 11px/800 tabular numbers. Hiding it at zero is the caller's decision.
- `accent` (default): `accent-fill` coral with a charcoal label. `danger`: `danger` / `danger-fg`. `neutral`: `surface-sunken` / `fg`.
- For word labels (新着, おすすめ, 公開中, 下書き) use **Tag**.
