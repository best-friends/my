Tabs with automatic activation; the selection can live in the URL. **@my/ui:** `Tabs` (`Tabs`, `TabList`, `Tab`, `TabPanel`) — `app/ui/src/Tabs.tsx`.

**Consumer provides:** `Tab`s with `id` and label (or `href` to make them links), a `TabList` `aria-label`, matching `TabPanel`s, and `selectedKey` / `onSelectionChange` for a URL-kept selection.

- List: a 1px `border` rule; tabs 14px/700 `fg-muted`, 10×16 padding, 44px min on coarse pointers; hover `fg`.
- Selected: `accent` label and a 3px `accent` underline.
- Tabs that do not fit scroll sideways without a scrollbar; the edge with hidden tabs fades over its last 2rem.
- Focus ring is inset (the list clips an outer one). Left / Right follow the reading direction; Home / End jump.
