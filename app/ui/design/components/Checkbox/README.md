A checkbox with its label, optional description and error. **@my/ui:** `Checkbox` — `app/ui/src/Checkbox.tsx`.

**Consumer provides:** the label as children, `isSelected` / `onChange` or `defaultSelected`, `isIndeterminate`, `description`, `errorMessage`, `isInvalid`, `isDisabled`.

- Box: 20px, 6px radius, 2px `border-strong` on `surface-sunken`. Checked or mixed: `accent` fill with an `accent-fg` check / dash (5.05:1 light, 6.9:1 dark).
- Invalid: `danger` border; checked + invalid fills `danger` with `danger-fg`.
- Description and error line up with the label text (32px inset). Label 15px `fg`; 24px row (44px coarse).
