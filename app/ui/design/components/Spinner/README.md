An indeterminate progress indicator named by what is in progress. **@my/ui:** `Spinner` — `app/ui/src/Spinner.tsx`.

**Consumer provides:** `label` (「投稿を読み込み中」), `size` (`sm` 16px · `md` 24px default · `lg` 40px), and a text colour through `className` to inherit.

- A `border` track with an `accent` quarter arc (or the current colour, e.g. inside a Button). Spins only without a reduced-motion preference; the name still says something is loading when it stands still.
- Inside a loading region, the region also sets `aria-busy="true"`.
