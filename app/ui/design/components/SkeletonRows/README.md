Skeletons in the shape of the rows a list loads: a post, an account row and a notification. **@my/ui:** `SkeletonRows` (`PostSkeleton`, `AccountRowSkeleton`, `NotificationRowSkeleton`) — `app/ui/src/SkeletonRows.tsx`.

**Consumer provides:** how many rows to repeat; the busy region carries the one loading label however many rows it shows.

- `PostSkeleton` mirrors PostCard: a raised card, avatar block beside two header lines, three body lines, four 48×32 action blocks.
- `AccountRowSkeleton`: avatar beside name and handle, separated by `border` dividers (no card).
- `NotificationRowSkeleton`: avatar beside the sentence and time, then two lines of the quoted post.
