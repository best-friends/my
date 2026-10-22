An action menu opened from a button (APG menu button). **@my/ui:** `Menu` (`MenuTrigger`, `Menu`, `MenuItem`, `MenuSeparator`, `SubmenuTrigger`) — `app/ui/src/Menu.tsx`.

**Consumer provides:** a trigger Button (usually ghost toolbar with MoreIcon and `aria-label` 「その他」), the menu's `aria-label`, `MenuItem`s with `onAction` or `href`, `intent="danger"` for destructive items, separators, submenus.

- Popover: `surface-raised`, 16px radius, 1px `border`, `elevation-lg`, 6px padding, min 192px.
- Items: 14px, 8×12 padding, 10px radius, optional 18px leading icon; focused `surface-sunken`; danger items `danger`; disabled `fg-muted`; a submenu caret (`SubmenuIcon`, mirrors) at the end. 44px tall on coarse pointers.
- Arrow keys, Home / End and type-ahead move; Esc closes and returns focus. Submenus open toward the inline end.
