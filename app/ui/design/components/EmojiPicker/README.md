A search field over a grid of emoji grouped by category, for posts, profiles and reactions. **@my/ui:** `EmojiPicker` — `app/ui/src/EmojiPicker.tsx`.

**Consumer provides:** `emojis` (custom emoji with `category` and `aliases`), `recent`, `unicode` (for reactions), `onSelect` / `onSelectUnicode`, and the `labels` (検索 / 最近使ったもの / カテゴリなし / 見つかりません).

- A TextField (`type="search"`) above a 256px-tall scrolling grid.
- Section headers 12px/800 `fg-muted`. Cells 40px (44px coarse), 10px radius, emoji 24–28px; hover and focus `surface-sunken`, keyboard focus adds the `focus` ring.
- Arrow keys move in two dimensions, Home / End jump, Enter / Space pick. Each custom emoji is named by its `:shortcode:`.
- Usually shown inside a Popover opened from the composer's EmojiIcon.
