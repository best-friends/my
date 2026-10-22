A switch (`role="switch"`) for a setting that takes effect at once. **@my/ui:** `Switch` — `app/ui/src/Switch.tsx`.

**Consumer provides:** the label as children, `isSelected` / `onChange` or `defaultSelected`, `description`, `errorMessage`, `isDisabled`.

- Track 44×26 pill: `switch-off` (3.16:1 on card) when off, `accent` when on; the 18px `switch-knob` sits at the inline start and moves to the end, so it follows the writing direction.
- Track first, then the label (15px `fg`); the description lines up with the label (56px inset).
- Focus: 3px `focus` ring around the track. Disabled: opacity .5.
- For a settings list, put switches in rows separated by `border` dividers (see Card).
