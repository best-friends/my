The facts of a record (a server's state, an account's history) as terms above their values, flowing into columns as the container widens. **@my/ui:** `DescriptionList` with `DescriptionItem` — `app/ui/src/DescriptionList.tsx`.

**Consumer provides:** `DescriptionItem`s (`term`, value as children) and `columns` (1–4; the most columns reached in a wide container).

- Term 13px/700 `fg-muted`; value 15px `fg`, wrapping anywhere so a long domain never scrolls the page.
- The list is its own size container: 1 column when narrow, 2 from 24rem, 3–4 from 42rem. Gaps 12px × 24px.
- Long values (an error, a payload) belong in a one-column list.
