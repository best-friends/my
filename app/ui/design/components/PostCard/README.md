The layout of one post: context line, avatar, author and time, body, media, details, reactions and actions in slots. **@my/ui:** `PostCard` with `PostCardAuthor` — `app/ui/src/PostCard.tsx`.

**Consumer provides:** `author` (a link around `PostCardAuthor` with `name` and `handle`), `time` (a `<time>` link), `content` (PostContent, or a ContentWarning around it), `avatar` (`<Avatar decorative size="md">`), and optionally `context` (「haru さんがブースト」 with a BoostIcon), `media` (MediaGrid or LinkCard), `details`, `reactions`, `actions` (ToggleButtons and a Menu), `label`, `highlighted`, `lineAbove` / `lineBelow`.

- Brand styling: each post is a raised card — `surface-raised`, 1px `border`, `radius-card`, `shadow-card`, 16×18 padding, 12px between cards. (@my/ui draws flat rows with a bottom border; the card is the brand's change.)
- Header: name 15px/800 and handle 13px `fg-muted`, each cut with an ellipsis (the handle from the domain's end); time 13px `fg-muted` at the end. Below a 28rem card the handle wraps under the name; from 36rem the avatar gets its own column.
- Context line 13px/700 in `success` with a 16px BoostIcon.
- Actions: reply (ghost toolbar Button), boost (`success` ToggleButton), favourite (`accent`), bookmark, then the overflow Menu pushed to the end. Icons change shape when on.
- Reactions: 30px pills on `surface`; the reader's own is `soft` with an `accent` border.
- `highlighted` (the post a page is about): 2px `accent` border, plus `label` (「選択中の投稿」) read first. Thread: `lineAbove` / `lineBelow` join consecutive cards (square corners where they meet) with a 2px `border-strong` line through the avatars.
- It is an `<article>` labelled by context, author and time and described by the content; inside a Feed it is one of the feed's articles.
