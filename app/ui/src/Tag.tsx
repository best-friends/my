// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, createElement } from "react";
import { tv } from "tailwind-variants";

const tag = tv({
  variants: {
    variant: {
      outlined: "rounded-full border border-border px-2 text-sm text-fg-muted",
      solid: "rounded-full bg-accent-fill px-3 py-1 text-xs font-bold text-accent-fill-fg",
      pill: "rounded-full bg-surface-sunken px-3 py-1 text-xs font-bold text-fg",
    },
  },
  defaultVariants: { variant: "outlined" },
});

export type TagProps = Omit<ComponentPropsWithRef<"span">, "className" | "style"> & {
  /** `outlined` qualifies the value beside it; `pill` is a filled, rounded label in a list of them; `solid` is an accent-colored mark such as "Unread". */
  variant?: "outlined" | "pill" | "solid";
  /** `li` for a tag in a `ul`. */
  as?: "span" | "li";
};

/**
 * A short word that qualifies what it sits beside ("Estimated") or labels an account ("Bot"). It is text, not a control
 * or a count: a count is a `Badge`, a role a `RoleBadge`.
 */
export function Tag({ variant, as = "span", ...props }: TagProps) {
  return createElement(as, { ...props, className: tag({ variant }) });
}
