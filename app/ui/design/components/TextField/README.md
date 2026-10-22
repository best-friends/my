A single-line text field with its label, description and error message wired together. **@my/ui:** `TextField` (parts from `Field.tsx`) — `app/ui/src/TextField.tsx`.

**Consumer provides:** `label` (or `aria-label`), `description`, `errorMessage`, `placeholder`, `type`, and `value` / `onChange` or `defaultValue`; `isRequired`, `isInvalid`, `isDisabled`, `isReadOnly`.

- Label 13px/700 `fg`; description 13px `fg-muted`; error 13px/600 `danger` with a filled WarningIcon, so an error never rests on colour alone.
- Control: `surface-sunken` well, 1px `border-strong` (3:1+), 11px radius, 15px text, 42px min height (44px coarse); placeholder `fg-muted`.
- Focus: 3px `focus` ring. Invalid: 2px `danger` border. Disabled: opacity .5. Read-only: dashed border on `surface`.
