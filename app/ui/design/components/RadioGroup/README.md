A group of radios; Tab enters and leaves the group, arrow keys move and select. **@my/ui:** `RadioGroup` with `Radio` — `app/ui/src/RadioGroup.tsx`.

**Consumer provides:** the group's `label`, `description`, `errorMessage`, `orientation` (`vertical` default / `horizontal`), `value` / `onChange`; each `Radio` takes `value`, its label, and an optional `description`.

- Circle: 20px, 2px `border-strong` on `surface-sunken`; selected: `accent` border and a 10px `accent` dot. Invalid: `danger`.
- Group label 13px/700; options 4px apart vertically, 24px horizontally.
