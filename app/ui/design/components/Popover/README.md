A non-modal dialog anchored to its trigger, for interactive content (an emoji picker, a short form). **@my/ui:** `Popover` (with `PopoverTrigger`) — `app/ui/src/Popover.tsx`.

**Consumer provides:** `title` (or `aria-label`), content as children (or a function receiving `close`), `placement` (`bottom` by default). Put it inside a `PopoverTrigger` after its trigger button.

- `surface-raised`, `radius-card`, 1px `border`, `elevation-lg`, max 24rem; 16px padding, title 15px/800, content gap 8px; scrolls inside.
- Focus moves in on open; Esc or a press outside closes it and returns focus. Fades over 100ms under motion-safe.
- For a non-interactive hint use a Tooltip; for a list of actions use a Menu.
