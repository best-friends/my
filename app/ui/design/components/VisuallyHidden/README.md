Text that assistive technology reads but the screen does not show. **@my/ui:** `VisuallyHidden` — `app/ui/src/VisuallyHidden.tsx`.

**Consumer provides:** the text as children.

- The `sr-only` pattern: still in the accessibility tree. To hide something from everyone, use `hidden` instead.
- Used for the label behind a drawn number (Badge), the 「(新しいタブで開きます)」 of ExternalLink, the name of an icon-only link, and a busy region's loading label.
