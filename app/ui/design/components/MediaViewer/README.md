The lightbox that opens a post's picture at its original size and steps through the others. **@my/ui:** `MediaViewer` (opened by MediaGrid) — `app/ui/src/MediaViewer.tsx`.

**Consumer provides:** nothing directly — pass `viewerLabels` to MediaGrid (`title(position, count)` → 「画像 2 / 3」, `close` 「閉じる」, `previous` 「前へ」, `next` 「次へ」, and a new `altText` 「代替テキスト」 for the note button) and an `original` on each picture.

- **The picture fills the frame.** The frame is the picture itself — no header, no padding — scaled to fit (max 85svh − 4rem, up to 80rem wide), with `radius-card` corners, `elevation-lg`, on the `overlay` scrim.
- **Close:** a 40px round × over the picture's top-right corner (`toast-bg` with a `toast-fg` icon). The dialog's title (「画像 2 / 3」) is visually hidden; it still names the dialog.
- **Pagination sits outside the frame,** centred under it: previous / next as round `surface-raised` icon Buttons (they mirror in right-to-left text and wrap around at the ends) and a `toast-bg` pill with the position 「2 / 3」. The arrow keys step too.
- **Alt text:** when the picture has a description, a 40px round note button sits in the bottom-right corner (`aria-label` 「代替テキスト」, `aria-expanded`, `aria-controls`). Pressing it lays the description over the whole picture — white `toast-fg` text, 15px / 1.85, on `alt-overlay` (black 80%), scrolling inside if long; the button turns white with a dark icon while open and closes it again. **No description → no button at all**; the picture is then named by `noDescriptionLabel` for screen readers.
- The note icon is not in @my/ui yet: add an `AltTextIcon` to `Icons.tsx` (Phosphor's PiNote shape is drawn here).
- Esc, the × and a press on the scrim close the viewer, and focus returns to the picture that opened it.
