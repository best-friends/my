A placeholder shape for content that is loading. **@my/ui:** `Skeleton` — `app/ui/src/Skeleton.tsx`.

**Consumer provides:** `shape` and, for `circle` and `rect`, the size. The region it fills sets `aria-busy="true"` and carries the one loading label (a Spinner or VisuallyHidden text).

- `text`: one line, 1em tall, full width, 6px radius. `circle`: an avatar — 40px with `radius-avatar`, matching Avatar's shape. `rect`: a media or card block, 96px, 14px radius.
- Fill `border`; pulses (opacity to .5 over 2s) only without a reduced-motion preference. Hidden from assistive technology.
