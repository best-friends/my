// SPDX-License-Identifier: AGPL-3.0-only
import { Button as RACButton, type ButtonProps as RACButtonProps, composeRenderProps } from "react-aria-components";
import { tv, type VariantProps } from "tailwind-variants";

const button = tv({
  base: [
    "inline-flex cursor-default items-center justify-center gap-2 rounded-md text-center font-medium select-none",
    "data-disabled:opacity-50 data-focus-visible:focus-ring data-pending:cursor-wait motion-safe:transition-colors",
    // The button sizes a child icon, which sets no size of its own (web-client.md G11).
    "pointer-coarse:min-h-11 [&>svg]:shrink-0",
  ],
  variants: {
    variant: {
      /** The one main action of a view. */
      primary: "bg-accent text-accent-fg data-hovered:bg-accent-hover data-pressed:bg-accent-hover",
      /** Other actions. */
      secondary: "border border-border-strong bg-surface-raised text-fg data-hovered:bg-surface-sunken data-pressed:bg-surface-sunken",
      /** Low-emphasis actions in toolbars and lists. */
      ghost: "text-fg data-hovered:bg-surface-sunken data-pressed:bg-surface-sunken",
      /** A low-emphasis action beside content, in the muted text color (the button that lists who reacted). */
      subtle: "text-fg-muted data-hovered:bg-surface-sunken data-pressed:bg-surface-sunken",
      /** Destructive actions. */
      danger: "bg-danger text-danger-fg data-hovered:shadow-md data-pressed:shadow-sm",
    },
    size: {
      sm: "min-h-8 px-3 text-sm [&>svg]:size-4",
      md: "min-h-10 px-4 text-base [&>svg]:size-5",
      /** Among `ToggleButton`s in a row of actions (a post's): as tall as they are, with its icon as large. */
      toolbar: "min-h-8 px-2 text-sm [&>svg]:size-5",
    },
  },
  defaultVariants: { variant: "secondary", size: "md" },
});

export type ButtonProps = RACButtonProps & VariantProps<typeof button>;

/**
 * A button (React Aria Button): pressed with a pointer, Enter or Space. Its accessible name is its
 * children, or `aria-label` when they are only an icon. While `isPending` it ignores presses but keeps
 * its focus.
 */
export function Button({ variant, size, className, ...props }: ButtonProps) {
  return <RACButton {...props} className={composeRenderProps(className, (className) => button({ variant, size, className }))} />;
}
