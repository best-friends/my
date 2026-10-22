A post's pictures, videos and audio in a 16:9 grid of one to four cells. **@my/ui:** `MediaGrid` — `app/ui/src/MediaGrid.tsx`.

**Consumer provides:** `items` (`id`, `src` / `sources`, `alt` — `null` when the author wrote none, `blurhash`, `playback` for video / audio / gifv, `original` for the viewer), `noDescriptionLabel` (「説明なし」), `sensitive`, `revealLabel` (「センシティブなメディアを表示」), `isRevealed` / `onRevealedChange`, `viewerLabels`.

- Brand frame: 16px radius, 4px gaps, cells on `surface-sunken`. Layouts: 1; 2 side by side; 3 with the first filling the start column; 4 as a 2×2 grid.
- Each cell shows its Blurhash until the image has loaded. A picture with an `original` is a button that opens the MediaViewer (zoom-in cursor, inset focus ring).
- No description: a 「説明なし」 pill (11px/800 on `surface-raised`, `elevation-sm`) in the end corner, and that text names the picture.
- Sensitive: the images are not loaded; each cell shows its blur, and one pill toggle (14px/800, `elevation-md`) sits in the middle. Once shown, a 32px HideMediaIcon toggle in the start corner conceals them again (`aria-pressed`).
- Video: the poster with a 56px coral play disc and a duration chip; never autoplays (a gifv loops muted only when autoplay is allowed and motion is not reduced). Audio: the poster with a raised pill player along the bottom.
