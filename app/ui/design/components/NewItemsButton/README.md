「3件の新しい投稿」: new items wait above a feed until the reader asks for them. **@my/ui:** `NewItemsButton` — `app/ui/src/NewItemsButton.tsx`.

**Consumer provides:** `count`, `label` formatted by count, `onPress` (insert the items and move to the top of the feed).

- A small primary Button (coral `accent-fill`, charcoal label) with the NewItemsIcon (arrow up), a pill with `elevation-md`, centred.
- It sits in a polite `role="status"` region that is always present, so the new count is announced without taking focus. At zero nothing is drawn.
