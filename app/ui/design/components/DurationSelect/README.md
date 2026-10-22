A Select of fixed durations for moderation actions (silence, suspend), with an optional “no end”. **@my/ui:** `DurationSelect` — `app/ui/src/DurationSelect.tsx`.

**Consumer provides:** `durations` in seconds, `formatDuration(seconds)` in the viewer's locale (「1日」「7日」), `indefiniteLabel` (「無期限」) when the action may have no end, `value` / `onChange` (seconds, `null` for no end).

- Looks exactly like Select. “No end” comes last, so the list reads from the lightest choice to the heaviest.
