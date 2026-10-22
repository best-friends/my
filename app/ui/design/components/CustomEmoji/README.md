Custom emoji drawn as images in user text, read as their `:shortcode:`. **@my/ui:** `CustomEmoji` (`CustomEmoji`, `EmojiText`, `insertCustomEmojis`) — `app/ui/src/CustomEmoji.tsx`.

**Consumer provides:** `EmojiText` with `text` and the API's `emojis` for display names and warnings; PostContent handles bodies. `allowMotion` defaults to the app's AutoplayContext.

- 1.25em tall, on the line's middle, as a word of the line. `alt` and `title` are the `:shortcode:`.
- An animated emoji moves only when the reader allows motion; otherwise its still image. Unknown shortcodes stay text; code keeps its colons.
