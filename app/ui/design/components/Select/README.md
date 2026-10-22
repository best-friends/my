A select-only combobox: a trigger that opens a list of options. **@my/ui:** `Select` — `app/ui/src/Select.tsx`.

**Consumer provides:** `label`, `SelectItem`s (or `items` + a render function), `value` / `onChange` or `defaultValue`, `placeholder`; `variant="toolbar"` with an `icon` and `aria-label` for a compact select in a row of tools.

- `field` (default): the TextField control box with a 16px chevron in `fg-muted`; placeholder in `fg-muted`.
- `toolbar`: a ghost toolbar button (32px, 10px radius, 20px icon, no chevron) showing the chosen option — the composer's visibility and language pickers.
- List: `surface-raised` popover, 14px radius, 1px `border`, `elevation-md`, 6px padding. Options 34px (44px coarse), 10px radius, focused `surface-sunken`; the selected one has an `accent` check; an option's `description` sits under its label in 13px `fg-muted` and is hidden in the closed select.
