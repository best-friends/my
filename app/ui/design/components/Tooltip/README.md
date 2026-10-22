A short description of its trigger on hover or keyboard focus. **@my/ui:** `Tooltip` (with `TooltipTrigger`) — `app/ui/src/Tooltip.tsx`.

**Consumer provides:** the text as children, `placement` (`top` default), `offset` (8). It never holds focus or interactive content, and never replaces an icon button's own `aria-label`.

- Brand styling: a dark pill like the Toast — `toast-bg`, `toast-fg` 13px/600, 10px radius, `elevation-md`, max 256px, with a small arrow toward the trigger; in Dark a 1px `border` ring.
- @my/ui currently draws it on `surface-raised` with a `border-strong` outline; swap those classes for this look.
- Opens after a hover delay, at once on focus; closes on Esc. Describes the trigger with `aria-describedby`.
