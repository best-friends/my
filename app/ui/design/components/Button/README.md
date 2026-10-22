A pill-shaped button for actions, in four @my/ui variants and three sizes. **@my/ui:** `Button` — `app/ui/src/Button.tsx`.

**Consumer provides:** a verb label as children (投稿する, 変更を保存, 削除する), or an icon with `aria-label`; `variant`, `size`, `onPress`, and `isPending` / `isDisabled` when needed.

| Prop | Values | Brand styling |
| --- | --- | --- |
| `variant` | `primary` | the one main action of a view: `accent-fill` (brand coral) with a charcoal `accent-fill-fg` label (4.89:1); hover lightens to `accent-fill-hover` and lifts 1px |
| | `secondary` (default) | other actions: `soft` fill with an `accent` label (4.54:1) |
| | `ghost` | low emphasis in toolbars and lists: `fg` label, `surface-sunken` on hover |
| | `danger` | destructive actions: `danger` fill, `danger-fg` label (7.3:1) |
| `size` | `sm` 32px · `md` 40px (default) · `toolbar` 32px with a 20px icon | coarse pointers grow to 44px |

- Every button is a pill (`radius-pill`), 800 weight. Icons are @my/ui Icons sized by the button (16px at `sm`, 20px otherwise).
- `isPending`: keeps focus, ignores presses, shows a `Spinner size="sm"` before the label and `cursor: wait`.
- Disabled: opacity .5. Focus: 3px `focus` ring, 2px offset.
- **Intentional addition — `outline`:** the brand kit's follow button (transparent, 1px `outline-border`, `accent` label). Not in @my/ui; add it as a variant if you keep it.
- One `primary` per view. Never white text on the coral fill.
