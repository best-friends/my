A modal sheet that rises from the bottom edge on narrow screens and is a centred dialog elsewhere. **@my/ui:** `Sheet` (in `Dialog.tsx`) — `app/ui/src/Sheet.tsx`.

**Consumer provides:** the same props as Dialog, without `size`.

- Below a 32rem container: full width, 24px top corners, a 40×4 `border-strong` grip, padding for the bottom safe area, max 90svh; slides up from the edge.
- From 32rem: a centred dialog, max 512px, rounded on all corners.
- Use it for menus of actions and pickers on phones (the composer's options, a post's share sheet).
